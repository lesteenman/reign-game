// Package worker provides SQS consumer logic for puzzle generation.
package worker

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// newSeed picks a fresh int64 seed for one generation attempt. Uses
// crypto/rand for an unbiased 63-bit draw. The sign-bit mask is for
// readability — all seeds end up non-negative, which is nicer to copy
// out of logs and paste into `task reproduce`. JS safe-integer
// precision is handled separately by encoding the seed as a JSON
// string in the /api/puzzles/next response, not by the mask. Unbiased
// is not a security requirement here — only collision avoidance at
// pool-stocking concurrency.
func newSeed() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, fmt.Errorf("crypto/rand read: %w", err)
	}
	u := binary.BigEndian.Uint64(buf[:])
	// Mask the sign bit so the result is a non-negative int64.
	return int64(u &^ (1 << 63)), nil
}

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
// Each message is deserialized, a generator is constructed, and the generated
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

	// Build generator options from request. MaxAttempts is a pass-through
	// override; zero means "use generator package default". An explicit
	// seed lets cmd/reproduce regenerate the same puzzle deterministically
	// (R-06C).
	seed, err := newSeed()
	if err != nil {
		return fmt.Errorf("picking seed: %w", err)
	}
	opts := []generator.Option{generator.WithSeed(seed)}
	if req.MaxAttempts > 0 {
		opts = append(opts, generator.WithMaxAttempts(req.MaxAttempts))
	}

	g, err := generator.New(req.Size, handler.MarksPerUnitFromMode(req.Mode), opts...)
	if err != nil {
		return fmt.Errorf("constructing generator (size=%d, mode=%s): %w", req.Size, req.Mode, err)
	}

	// Create a timeout context for generation. Honors both the upstream
	// SQS/Lambda context and our per-puzzle budget.
	genCtx, cancel := context.WithTimeout(ctx, generationTimeout)
	defer cancel()

	startTime := time.Now()
	pz, err := g.Generate(genCtx)
	if err != nil {
		return fmt.Errorf("generating puzzle (size=%d, mode=%s): %w", req.Size, req.Mode, err)
	}
	durationMs := time.Since(startTime).Milliseconds()

	// Generate a UUID for the puzzle.
	puzzleID, err := w.newUUID()
	if err != nil {
		return fmt.Errorf("generating puzzle ID: %w", err)
	}

	// Translate generator.Puzzle → repository.PuzzleRecord.
	solution := make([][]bool, pz.N)
	for i := range solution {
		solution[i] = make([]bool, pz.N)
	}
	for _, m := range pz.Solution {
		solution[m.Row][m.Col] = true
	}

	rec := &repository.PuzzleRecord{
		GridSize:             req.Size,
		Mode:                 req.Mode,
		ID:                   puzzleID,
		Status:               "ready",
		RegionMap:            pz.Regions,
		Solution:             solution,
		Difficulty:           int(pz.Difficulty),
		MaxTier:              pz.Metrics.MaxTier,
		TierCounts:           pz.Metrics.TierCounts,
		TraceLen:             pz.Metrics.TraceLen,
		GenerationDurationMs: durationMs,
		CreatedAt:            time.Now().UTC().Format(time.RFC3339),
		Seed:                 seed,
	}

	if err := w.store.PutPuzzle(ctx, rec); err != nil {
		return fmt.Errorf("storing generated puzzle: %w", err)
	}

	log.Printf("generator: produced puzzle %s (size=%d, mode=%s, difficulty=%d, seed=%d, trips=%d, duration=%dms)",
		puzzleID, req.Size, req.Mode, pz.Difficulty, seed, pz.Metrics.SafetyNetTrips, durationMs)

	if pz.Metrics.SafetyNetTrips > 0 {
		// A guard fire is a real rule leak in the grower or mutator —
		// the safety net rescued the attempt, but the underlying code
		// needs investigating. task reproduce --seed=X --n=N --k=K
		// replays the exact same sequence so the leak can be diagnosed.
		log.Printf("WARN: generator: safety-net fired %d time(s) on puzzle %s (size=%d, mode=%s, seed=%d) — reproduce with `task reproduce -- --seed=%d --n=%d --k=%d`",
			pz.Metrics.SafetyNetTrips, puzzleID, req.Size, req.Mode, seed,
			seed, req.Size, handler.MarksPerUnitFromMode(req.Mode))
	}

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
