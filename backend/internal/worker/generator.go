// Package worker provides SQS consumer logic for puzzle generation.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/model"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// generationTimeout is the maximum time allowed for puzzle generation in
// the SQS consumer. Set to 14 minutes to leave 1 minute for SQS overhead
// and DynamoDB write within the 15-minute Lambda timeout.
const generationTimeout = 14 * time.Minute

// PuzzleStore defines the puzzle persistence operations used by the worker.
type PuzzleStore interface {
	PutPuzzle(ctx context.Context, puzzle *repository.PuzzleRecord) error
}

// SQSConsumerAPI defines the SQS operations used by the local poller.
type SQSConsumerAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// UUIDGenerator produces UUID v4 strings. Abstracted for testing.
type UUIDGenerator func() (string, error)

// GeneratorWorker handles SQS puzzle generation events.
type GeneratorWorker struct {
	store   PuzzleStore
	newUUID UUIDGenerator
}

// NewGeneratorWorker creates a GeneratorWorker with the given puzzle store
// and UUID generator function.
func NewGeneratorWorker(store PuzzleStore, newUUID UUIDGenerator) *GeneratorWorker {
	return &GeneratorWorker{
		store:   store,
		newUUID: newUUID,
	}
}

// HandleSQSEvent processes an SQS event containing puzzle generation requests.
// Each message is deserialized, a pipeline is constructed, and the generated
// puzzle is stored in DynamoDB. Returns an error if any message fails (SQS
// will retry).
func (w *GeneratorWorker) HandleSQSEvent(ctx context.Context, event events.SQSEvent) error {
	for i := range event.Records {
		if err := w.processMessage(ctx, &event.Records[i]); err != nil {
			return fmt.Errorf("processing SQS message %s: %w", event.Records[i].MessageId, err)
		}
	}
	return nil
}

// processMessage handles a single SQS message.
func (w *GeneratorWorker) processMessage(ctx context.Context, record *events.SQSMessage) error {
	var req queue.GenerationRequest
	if err := json.Unmarshal([]byte(record.Body), &req); err != nil {
		return fmt.Errorf("deserializing generation request: %w", err)
	}

	// Build pipeline from request parameters.
	params := handler.GenerateParams{
		Size:           req.Size,
		Mode:           req.Mode,
		Pipeline:       req.Pipeline,
		Solver:         req.Solver,
		Regions:        req.Regions,
		RegionVariance: req.RegionVariance,
		Deducible:      req.Deducible,
		Concurrency:    req.Concurrency,
	}

	// Determine markersPerUnit and minSize based on mode.
	params.MarkersPerUnit = 1
	params.MinSize = 3
	if params.Mode == handler.ModeDouble {
		params.MarkersPerUnit = 2
		params.MinSize = 4
	}

	pipeline := handler.BuildPipeline(&params)

	// Create a timeout context for generation.
	genCtx, cancel := context.WithTimeout(ctx, generationTimeout)
	defer cancel()

	opts := generator.GenerateOpts{
		Timeout:   generationTimeout,
		Ctx:       genCtx,
		Deducible: true,
		RegionOpts: generator.RegionOpts{
			Variance: req.RegionVariance,
			MinSize:  params.MinSize,
		},
	}

	startTime := time.Now()

	var puzzle *model.Puzzle
	var err error
	if req.Concurrency > 1 {
		opts.Concurrency = req.Concurrency
		puzzle, err = generator.GenerateConcurrent(pipeline, req.Size, params.MarkersPerUnit, opts)
	} else {
		puzzle, err = pipeline.Generate(req.Size, params.MarkersPerUnit, opts)
	}
	if err != nil {
		return fmt.Errorf("generating puzzle (size=%d, mode=%s): %w", req.Size, req.Mode, err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	// Generate a UUID for the puzzle.
	puzzleID, err := w.newUUID()
	if err != nil {
		return fmt.Errorf("generating puzzle ID: %w", err)
	}

	// Store the generated puzzle.
	puzzleRecord := &repository.PuzzleRecord{
		GridSize:             req.Size,
		Mode:                 req.Mode,
		ID:                   puzzleID,
		Status:               "ready",
		Verdict:              "none",
		RegionMap:            puzzle.RegionMap,
		Solution:             puzzle.Solution,
		Pipeline:             req.Pipeline,
		Solver:               req.Solver,
		Regions:              req.Regions,
		RegionVariance:       req.RegionVariance,
		Deducible:            req.Deducible,
		Concurrency:          req.Concurrency,
		GenerationDurationMs: durationMs,
		CreatedAt:            time.Now().UTC().Format(time.RFC3339),
	}

	if err := w.store.PutPuzzle(ctx, puzzleRecord); err != nil {
		return fmt.Errorf("storing generated puzzle: %w", err)
	}

	log.Printf("generated puzzle %s (size=%d, mode=%s, duration=%dms)",
		puzzleID, req.Size, req.Mode, durationMs)

	return nil
}

// RunLocalPoller long-polls an SQS queue and processes messages using the
// provided handler function. Designed for local development against
// LocalStack. Exits when the context is cancelled.
func RunLocalPoller(ctx context.Context, sqsClient SQSConsumerAPI, queueURL string, handleFn func(context.Context, events.SQSEvent) error) {
	log.Printf("starting local SQS poller for queue: %s", queueURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("local poller shutting down")
			return
		default:
		}

		output, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("error receiving SQS message: %v", err)
			continue
		}

		for _, msg := range output.Messages {
			sqsEvent := events.SQSEvent{
				Records: []events.SQSMessage{
					{
						MessageId: aws.ToString(msg.MessageId),
						Body:      aws.ToString(msg.Body),
					},
				},
			}

			if err := handleFn(ctx, sqsEvent); err != nil {
				log.Printf("error processing message %s: %v", aws.ToString(msg.MessageId), err)
				continue
			}

			// Delete message after successful processing.
			_, delErr := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if delErr != nil {
				log.Printf("error deleting message %s: %v", aws.ToString(msg.MessageId), delErr)
			}
		}
	}
}
