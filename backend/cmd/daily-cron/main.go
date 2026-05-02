// Package main is the daily-puzzle cron Lambda entry point.
//
// Two EventBridge rules dispatch into this single Lambda binary by
// `detail-type`:
//
//   - t-6h-ensure  -> daily.EnsureCandidate
//     Runs at 18:00 UTC; ensures DAILY-CANDIDATE
//     holds a fresh approved puzzle ahead of the
//     midnight swap. Idempotent on duplicate firings.
//
//   - t-0-finalize -> daily.SyncFinalizeForToday
//     Runs at 00:00 UTC; writes today's DAILY#date
//     schedule row by either confirming the candidate
//     or recycling yesterday. Idempotent on duplicate
//     firings via attribute_not_exists guards.
//
// Empty pool / pool exhaustion paths are LOGGED but the Lambda
// returns nil so EventBridge does not retry — by design; the
// fallback (sync fallback in the GET handler) handles cold-start.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/eriksteenman/reign-game/backend/internal/daily"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// EventBridgeScheduledEvent is the slice of an EventBridge scheduled
// event we care about. Real events have many more fields; we read
// only `detail-type`.
type EventBridgeScheduledEvent struct {
	DetailType string `json:"detail-type"`
}

// Detail-type discriminators. Match the EventBridge rule definitions
// (Terraform chunk 6d).
const (
	DetailTypeEnsure   = "t-6h-ensure"
	DetailTypeFinalize = "t-0-finalize"
)

// dailyService is the narrow surface the dispatcher needs from the
// daily package. Defined as an interface so handle() is unit-testable
// without standing up a real DDB client.
type dailyService interface {
	EnsureCandidate(ctx context.Context, now time.Time) error
	SyncFinalizeForToday(ctx context.Context, today, yesterday string, now time.Time) (*repository.ScheduleRecord, error)
}

// realService is the production wrapper around the daily package's
// free functions, parameterized over a Repo. Tests skip this and
// inject a mock.
type realService struct {
	repo daily.Repo
}

func (r *realService) EnsureCandidate(ctx context.Context, now time.Time) error {
	return daily.EnsureCandidate(ctx, r.repo, now)
}

func (r *realService) SyncFinalizeForToday(ctx context.Context, today, yesterday string, now time.Time) (*repository.ScheduleRecord, error) {
	return daily.SyncFinalizeForToday(ctx, r.repo, today, yesterday, now)
}

// handle dispatches an EventBridge event to the right cron handler.
// `clock` is the time provider — production passes time.Now; tests
// pin it.
func handle(ctx context.Context, event EventBridgeScheduledEvent, svc dailyService, clock func() time.Time) error {
	now := clock().UTC()

	switch event.DetailType {
	case DetailTypeEnsure:
		if err := svc.EnsureCandidate(ctx, now); err != nil {
			if errors.Is(err, daily.ErrCandidatePoolEmpty) {
				log.Printf("daily cron: ensure: pool empty, T=0 will recycle (now=%s)", now.Format(time.RFC3339))
				return nil
			}
			log.Printf("daily cron: ensure failed: %v (now=%s)", err, now.Format(time.RFC3339))
			return err
		}
		log.Printf("daily cron: ensure: ok (now=%s)", now.Format(time.RFC3339))
		return nil

	case DetailTypeFinalize:
		today := now.Format("2006-01-02")
		yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
		sched, err := svc.SyncFinalizeForToday(ctx, today, yesterday, now)
		if err != nil {
			if errors.Is(err, daily.ErrPoolExhausted) {
				log.Printf("daily cron: finalize: pool exhausted, sync fallback in GET will surface 500 until recovery (today=%s)", today)
				return nil
			}
			log.Printf("daily cron: finalize failed: %v (today=%s)", err, today)
			return err
		}
		log.Printf("daily cron: finalize: ok puzzleId=%s sourcePartition=%s (today=%s)", sched.PuzzleID, sched.SourcePartition, today)
		return nil

	default:
		// Unknown detail-type — log and return nil. EventBridge will
		// not retry. Misconfiguration in the rule definitions is the
		// only way to hit this branch.
		log.Printf("daily cron: WARN: unknown detail-type %q, ignoring", event.DetailType)
		return nil
	}
}

// loadAWSConfig mirrors backend/cmd/api/main.go's loadAWSConfig.
// See that file for the full rationale on http.DefaultTransport.Clone();
// even though this Lambda only uses the AWS SDK today, mirroring the
// pattern keeps the two entry points aligned and future-proof.
func loadAWSConfig(ctx context.Context) (aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	httpClient := &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}
	return cfg, nil
}

// newDynamoDBClient creates a DynamoDB client, optionally with a
// LocalStack endpoint via DYNAMODB_ENDPOINT (matches cmd/api).
func newDynamoDBClient(cfg *aws.Config) *dynamodb.Client {
	if endpoint := os.Getenv("DYNAMODB_ENDPOINT"); endpoint != "" {
		return dynamodb.NewFromConfig(*cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return dynamodb.NewFromConfig(*cfg)
}

func main() {
	ctx := context.Background()

	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("daily cron: aws config: %v", err)
	}
	ddb := newDynamoDBClient(&cfg)

	tableName := os.Getenv("PUZZLE_POOL_TABLE")
	if tableName == "" {
		log.Fatal("daily cron: PUZZLE_POOL_TABLE not set")
	}
	repo := repository.NewPuzzleRepository(ddb, tableName)
	svc := &realService{repo: repo}

	lambda.Start(func(ctx context.Context, event EventBridgeScheduledEvent) error {
		return handle(ctx, event, svc, time.Now)
	})
}
