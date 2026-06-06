package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/eriksteenman/reign-game/backend/internal/message"
)

// mockSQSClient implements SQSAPI for testing.
type mockSQSClient struct {
	sendMessageFunc      func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	sendMessageBatchFunc func(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error)
}

func (m *mockSQSClient) SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	return m.sendMessageFunc(ctx, params, optFns...)
}

func (m *mockSQSClient) SendMessageBatch(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
	return m.sendMessageBatchFunc(ctx, params, optFns...)
}

func TestPublishGenerationRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     message.GenerationRequest
		sendErr error
		wantErr bool
	}{
		{
			name: "sends correct JSON message",
			req: message.GenerationRequest{
				Size: 7,
				Mode: "standard",
			},
			wantErr: false,
		},
		{
			name: "sends double mode request",
			req: message.GenerationRequest{
				Size:        9,
				Mode:        "double",
				MaxAttempts: 30,
			},
			wantErr: false,
		},
		{
			name: "propagates SQS error",
			req: message.GenerationRequest{
				Size: 5,
				Mode: "standard",
			},
			sendErr: errors.New("sqs connection error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *sqs.SendMessageInput
			mock := &mockSQSClient{
				sendMessageFunc: func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
					capturedInput = params
					return &sqs.SendMessageOutput{}, tt.sendErr
				},
			}
			pub := NewPublisher(mock, "https://sqs.us-east-1.amazonaws.com/123456789/puzzle-generation")

			// Act
			err := pub.PublishGenerationRequest(context.Background(), &tt.req)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedInput == nil {
				t.Fatal("SendMessage was not called")
			}
			if *capturedInput.QueueUrl != "https://sqs.us-east-1.amazonaws.com/123456789/puzzle-generation" {
				t.Errorf("QueueUrl = %q, want puzzle-generation URL", *capturedInput.QueueUrl)
			}

			// Verify JSON body matches expected shape.
			var parsed message.GenerationRequest
			if err := json.Unmarshal([]byte(*capturedInput.MessageBody), &parsed); err != nil {
				t.Fatalf("failed to parse message body: %v", err)
			}
			if parsed.Size != tt.req.Size {
				t.Errorf("size = %d, want %d", parsed.Size, tt.req.Size)
			}
			if parsed.Mode != tt.req.Mode {
				t.Errorf("mode = %q, want %q", parsed.Mode, tt.req.Mode)
			}
			if parsed.MaxAttempts != tt.req.MaxAttempts {
				t.Errorf("maxAttempts = %d, want %d", parsed.MaxAttempts, tt.req.MaxAttempts)
			}
		})
	}
}

func TestPublishBatch_ChunksIntoTens(t *testing.T) {
	// Arrange — 23 requests should fan out into 3 batches: 10, 10, 3.
	reqs := make([]*message.GenerationRequest, 23)
	for i := range reqs {
		reqs[i] = &message.GenerationRequest{Size: 5 + i, Mode: "standard", MaxAttempts: i}
	}
	var captured []*sqs.SendMessageBatchInput
	mock := &mockSQSClient{
		sendMessageBatchFunc: func(_ context.Context, params *sqs.SendMessageBatchInput, _ ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			captured = append(captured, params)
			return &sqs.SendMessageBatchOutput{}, nil
		},
	}
	pub := NewPublisher(mock, "https://sqs.example/queue")

	// Act
	err := pub.PublishBatch(context.Background(), reqs)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured) != 3 {
		t.Fatalf("batch calls = %d, want 3", len(captured))
	}
	wantSizes := []int{10, 10, 3}
	for i, want := range wantSizes {
		if got := len(captured[i].Entries); got != want {
			t.Errorf("batch %d size = %d, want %d", i, got, want)
		}
	}
	// Every entry body round-trips to the matching request, and Ids are unique within each batch.
	idx := 0
	for b, in := range captured {
		seen := map[string]bool{}
		for _, e := range in.Entries {
			if e.Id == nil {
				t.Fatalf("batch %d entry has nil Id", b)
			}
			if seen[*e.Id] {
				t.Errorf("batch %d has duplicate entry Id %q", b, *e.Id)
			}
			seen[*e.Id] = true

			var parsed message.GenerationRequest
			if err := json.Unmarshal([]byte(*e.MessageBody), &parsed); err != nil {
				t.Fatalf("batch %d entry %q body unmarshal: %v", b, *e.Id, err)
			}
			if parsed != *reqs[idx] {
				t.Errorf("batch %d entry %q = %+v, want %+v", b, *e.Id, parsed, *reqs[idx])
			}
			idx++
		}
		if *in.QueueUrl != "https://sqs.example/queue" {
			t.Errorf("batch %d QueueUrl = %q", b, *in.QueueUrl)
		}
	}
	if idx != len(reqs) {
		t.Errorf("entries published = %d, want %d", idx, len(reqs))
	}
}

func TestPublishBatch_PartialFailureReturnsError(t *testing.T) {
	// Arrange — SendMessageBatch returns HTTP 200 with a non-empty Failed slice.
	reqs := []*message.GenerationRequest{
		{Size: 5, Mode: "standard"},
		{Size: 7, Mode: "standard"},
	}
	mock := &mockSQSClient{
		sendMessageBatchFunc: func(_ context.Context, _ *sqs.SendMessageBatchInput, _ ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			return &sqs.SendMessageBatchOutput{
				Failed: []sqstypes.BatchResultErrorEntry{
					{Id: aws.String("1"), Code: aws.String("InternalError")},
				},
			}, nil
		},
	}
	pub := NewPublisher(mock, "https://sqs.example/queue")

	// Act
	err := pub.PublishBatch(context.Background(), reqs)

	// Assert
	if err == nil {
		t.Fatal("expected error when batch response has Failed entries")
	}
}

func TestPublishBatch_EmptyInputNoCall(t *testing.T) {
	// Arrange
	called := false
	mock := &mockSQSClient{
		sendMessageBatchFunc: func(_ context.Context, _ *sqs.SendMessageBatchInput, _ ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			called = true
			return &sqs.SendMessageBatchOutput{}, nil
		},
	}
	pub := NewPublisher(mock, "https://sqs.example/queue")

	// Act
	err := pub.PublishBatch(context.Background(), nil)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("SendMessageBatch must not be called for empty input")
	}
}

func TestPublishBatch_SendError(t *testing.T) {
	// Arrange
	mock := &mockSQSClient{
		sendMessageBatchFunc: func(_ context.Context, _ *sqs.SendMessageBatchInput, _ ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			return nil, errors.New("sqs down")
		},
	}
	pub := NewPublisher(mock, "https://sqs.example/queue")

	// Act
	err := pub.PublishBatch(context.Background(), []*message.GenerationRequest{{Size: 5, Mode: "standard"}})

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}
