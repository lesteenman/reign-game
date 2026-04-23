package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// mockSQSClient implements SQSAPI for testing.
type mockSQSClient struct {
	sendMessageFunc func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

func (m *mockSQSClient) SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	return m.sendMessageFunc(ctx, params, optFns...)
}

func TestPublishGenerationRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     GenerationRequest
		sendErr error
		wantErr bool
	}{
		{
			name: "sends correct JSON message",
			req: GenerationRequest{
				Size: 7,
				Mode: "standard",
			},
			wantErr: false,
		},
		{
			name: "sends double mode request",
			req: GenerationRequest{
				Size:        9,
				Mode:        "double",
				MaxAttempts: 30,
			},
			wantErr: false,
		},
		{
			name: "propagates SQS error",
			req: GenerationRequest{
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
			var parsed GenerationRequest
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
