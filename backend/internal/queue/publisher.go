// Package queue provides SQS message publishing for puzzle generation requests.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSAPI defines the SQS operations used by Publisher.
// Keeping this minimal makes testing straightforward via mock implementations.
type SQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// GenerationRequest represents an SQS message for puzzle generation.
// Fields match the SQS message schema from the design document.
type GenerationRequest struct {
	Size           int     `json:"size"`
	Mode           string  `json:"mode"`
	Pipeline       string  `json:"pipeline"`
	Solver         string  `json:"solver"`
	Regions        string  `json:"regions"`
	RegionVariance float64 `json:"regionVariance"`
	Deducible      bool    `json:"deducible"`
	Concurrency    int     `json:"concurrency"`
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
func (p *Publisher) PublishGenerationRequest(ctx context.Context, req *GenerationRequest) error {
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
