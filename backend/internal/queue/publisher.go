// Package queue provides SQS message publishing for puzzle generation requests.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/eriksteenman/reign-game/backend/internal/message"
)

// maxBatchSize is the SQS SendMessageBatch hard limit on entries per call.
const maxBatchSize = 10

// SQSAPI defines the SQS operations used by Publisher.
// Keeping this minimal makes testing straightforward via mock implementations.
type SQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	SendMessageBatch(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error)
}

// Publisher sends puzzle generation requests to an SQS queue.
type Publisher struct {
	client   SQSAPI
	queueURL string
}

// NewPublisher creates a Publisher with the given SQS client and queue URL.
func NewPublisher(client SQSAPI, queueURL string) *Publisher {
	return &Publisher{
		client:   client,
		queueURL: queueURL,
	}
}

// PublishGenerationRequest serializes a generation request to JSON and sends
// it to the configured SQS queue.
func (p *Publisher) PublishGenerationRequest(ctx context.Context, req *message.GenerationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling generation request: %w", err)
	}

	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("sending SQS message: %w", err)
	}

	return nil
}

// PublishBatch serializes each generation request to JSON and sends them to
// the configured SQS queue using SendMessageBatch, chunking into groups of at
// most 10 entries (the SQS hard limit). Empty input is a no-op.
//
// SendMessageBatch returns HTTP 200 even when individual entries fail; the
// per-entry failures arrive in the response's Failed slice. Any failed entry
// (or transport error) yields a wrapped error so callers never silently drop
// requests.
func (p *Publisher) PublishBatch(ctx context.Context, reqs []*message.GenerationRequest) error {
	for start := 0; start < len(reqs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(reqs) {
			end = len(reqs)
		}
		chunk := reqs[start:end]

		entries := make([]sqstypes.SendMessageBatchRequestEntry, len(chunk))
		for i, req := range chunk {
			body, err := json.Marshal(req)
			if err != nil {
				return fmt.Errorf("marshaling generation request: %w", err)
			}
			entries[i] = sqstypes.SendMessageBatchRequestEntry{
				Id:          aws.String(strconv.Itoa(i)),
				MessageBody: aws.String(string(body)),
			}
		}

		out, err := p.client.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{
			QueueUrl: aws.String(p.queueURL),
			Entries:  entries,
		})
		if err != nil {
			return fmt.Errorf("sending SQS message batch: %w", err)
		}
		if len(out.Failed) > 0 {
			return fmt.Errorf("sending SQS message batch: %d of %d entries failed", len(out.Failed), len(entries))
		}
	}

	return nil
}
