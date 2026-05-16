// Package daily is the application service for the daily-puzzle HTTP
// endpoints (GET /api/daily/{date} and POST /api/daily/{date}/result).
// It owns the multi-step orchestration that GET and POST used to do
// inline in the handler: schedule reads, sync-fallback finalize,
// puzzle reads, play-row materialization, submission validation,
// transaction execution, and leaderboard rank lookup.
//
// Method signatures use plain-field DTOs (GetInput, DailyView, SubmitInput,
// SubmitResult) so the HTTP handler can talk to this package without
// importing `internal/repository`.
package daily

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// Sentinel errors. The HTTP handler translates these to status codes:
//
//	ErrInvalidDate          -> 400
//	ErrOutOfWindow          -> 404
//	ErrScheduleNotFinalized -> 404 (yesterday's schedule absent)
//	ErrPoolExhausted        -> 500 ("pool exhausted") — declared in sync.go
//	ErrPlayNotStarted       -> 400 ("play not started")
//	ErrAlreadySolved        -> 409
//	ErrInvalidSolution      -> 400
//	ErrNegativeClockSkew    -> 500
//
// Wrapping is fine (handler uses errors.Is). Other errors (DDB I/O,
// etc.) map to 500.
var (
	ErrInvalidDate          = errors.New("invalid date")
	ErrOutOfWindow          = errors.New("out of window")
	ErrScheduleNotFinalized = errors.New("schedule not finalized")
	// ErrPoolExhausted is declared in sync.go (more descriptive message).
	ErrPlayNotStarted    = errors.New("play not started")
	ErrAlreadySolved     = errors.New("already solved")
	ErrInvalidSolution   = errors.New("invalid solution")
	ErrNegativeClockSkew = errors.New("negative clock skew")
)

// Store is the persistence surface used by Service. Subset of
// *repository.PuzzleRepository's methods — production wiring passes
// the repo straight in; tests substitute a fake.
type Store interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error
	WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error
	ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
	PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error
	SubmitPlayTransactionally(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error
	LeaderboardRank(ctx context.Context, date string, elapsedMs int64, userID string) (int, error)
}

// GetInput is the plain-field input to Service.GetDaily.
type GetInput struct {
	PlayerID    string
	IsAnonymous bool
	Date        string // YYYY-MM-DD UTC
}

// DailyView is the result of GetDaily — enough data for the handler
// to render its 200 JSON body.
//
// ServerElapsedMs and SubmittedAt are pointer-typed: they are set ONLY
// when Outcome == "solved" (the canonical PlayRecord outcome string).
// Handlers use `omitempty` to drop them otherwise.
type DailyView struct {
	PuzzleID        string
	Grid            int
	Regions         [][]int
	AssignedAt      string
	Outcome         string
	ServerElapsedMs *int64
	SubmittedAt     *string
}

// SubmitInput is the plain-field input to Service.SubmitDaily.
type SubmitInput struct {
	PlayerID    string
	IsAnonymous bool
	Date        string // YYYY-MM-DD UTC
	PlayTimeMs  int64
	Solution    [][]int
}

// SubmitResult is the result of SubmitDaily.
// LeaderboardRank is nil for anonymous submits and for rank-fetch
// failures (best-effort post-commit).
type SubmitResult struct {
	ServerElapsedMs int64
	LeaderboardRank *int
}

// Service holds the application logic for the daily-puzzle endpoints.
type Service struct {
	store         Store
	tableName     string
	clock         func() time.Time
	replenishHook func(size int, mode string)
}

// New constructs a Service. tableName is the DynamoDB table name used
// when assembling transaction legs in FinalizeDaily. clock is used in
// tests to pin "now"; pass nil to default to time.Now. replenishHook
// fires on sync-fallback cold-start when the candidate pool drains; nil
// is acceptable in tests and in local-dev wiring without SQS.
func New(store Store, tableName string, clock func() time.Time, replenishHook func(size int, mode string)) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: store, tableName: tableName, clock: clock, replenishHook: replenishHook}
}

// FinalizeDaily writes the T=0 finalize transaction: puts the schedule
// row, updates the puzzle's lastDailyDate, and (confirm-mode only)
// deletes the candidate slot. Mirrors repository.FinalizeDailyTransaction
// in logic but delegates the actual TransactWriteItems call to
// s.store.WriteTransaction, keeping orchestration in service/ per the
// architecture rule.
//
// Returns repository.ErrScheduleAlreadyFinalized when leg 0 fails its
// condition (race-loser / duplicate cron firing). Callers should read the
// winner's schedule row and continue.
func (s *Service) FinalizeDaily(
	ctx context.Context,
	date, puzzleID, sourcePartition string,
	mode repository.FinalizeMode,
) error {
	if mode != repository.FinalizeModeConfirm && mode != repository.FinalizeModeRecycle {
		return fmt.Errorf("invalid finalize mode %q", mode)
	}

	now := s.clock().UTC().Format(time.RFC3339)

	items := []types.TransactWriteItem{
		{
			Put: &types.Put{
				TableName: aws.String(s.tableName),
				Item: map[string]types.AttributeValue{
					"PK":              &types.AttributeValueMemberS{Value: repository.BuildDailySchedulePK(date)},
					"SK":              &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
					"puzzleId":        &types.AttributeValueMemberS{Value: puzzleID},
					"assignedAt":      &types.AttributeValueMemberS{Value: now},
					"sourcePartition": &types.AttributeValueMemberS{Value: sourcePartition},
					"counters": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"started": &types.AttributeValueMemberN{Value: "0"},
							"solved":  &types.AttributeValueMemberN{Value: "0"},
						},
					},
				},
				ConditionExpression: aws.String("attribute_not_exists(PK)"),
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: sourcePartition},
					"SK": &types.AttributeValueMemberS{Value: puzzleID},
				},
				UpdateExpression: aws.String("SET lastDailyDate = :date"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":date": &types.AttributeValueMemberS{Value: date},
				},
			},
		},
	}

	if mode == repository.FinalizeModeConfirm {
		items = append(items, types.TransactWriteItem{
			Delete: &types.Delete{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.DailyCandidatePK},
					"SK": &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
				},
				ConditionExpression: aws.String("puzzleId = :pid"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":pid": &types.AttributeValueMemberS{Value: puzzleID},
				},
			},
		})
	}

	if err := s.store.WriteTransaction(ctx, items); err != nil {
		if repository.IsConditionalCheckFailureOnLeg(err, 0) {
			return repository.ErrScheduleAlreadyFinalized
		}
		return fmt.Errorf("finalizing daily transaction for %s (mode=%s): %w", date, mode, err)
	}
	return nil
}

// SubmitPlay commits a daily-puzzle solve via a single transaction with
// up to three legs (DP-12, D14):
//
//  1. UpdateItem PLAY → outcome=solved, submittedAt, serverElapsedMs,
//     clientClaimedMs. Conditional on outcome=started for idempotency —
//     a duplicate submission produces repository.ErrPlayNotInStartedState
//     (caller maps to HTTP 409).
//  2. UpdateItem schedule row → ADD counters.solved 1. Date keys off
//     submission.AssignedAt (DP-13: cross-midnight submissions credit the
//     prior date's counter).
//  3. PutItem leaderboard row at DAILY-LEADERBOARD#{playOriginDate} —
//     signed-in only. Anonymous (deviceId-keyed) submissions skip this
//     leg (D13).
//
// Mirrors repository.SubmitPlayTransactionally in logic but delegates
// the actual TransactWriteItems call to s.store.WriteTransaction,
// keeping orchestration in service/ per the architecture rule.
func (s *Service) SubmitPlay(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error {
	playOriginDate := submission.AssignedAt.UTC().Format("2006-01-02")
	submittedAt := submission.SubmittedAt.UTC().Format(time.RFC3339)
	elapsedMs := submission.SubmittedAt.Sub(submission.AssignedAt).Milliseconds()
	if elapsedMs < 0 {
		return fmt.Errorf("invalid submission: submittedAt before assignedAt (delta=%dms)", elapsedMs)
	}

	items := []types.TransactWriteItem{
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.BuildPlayPK(playerID)},
					"SK": &types.AttributeValueMemberS{Value: repository.BuildPlaySK(date)},
				},
				UpdateExpression: aws.String(
					"SET #outcome = :solved, submittedAt = :submittedAt, " +
						"serverElapsedMs = :serverMs, clientClaimedMs = :clientMs",
				),
				ConditionExpression: aws.String("#outcome = :started"),
				ExpressionAttributeNames: map[string]string{
					"#outcome": "outcome",
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":solved":      &types.AttributeValueMemberS{Value: repository.PlayOutcomeSolved},
					":started":     &types.AttributeValueMemberS{Value: repository.PlayOutcomeStarted},
					":submittedAt": &types.AttributeValueMemberS{Value: submittedAt},
					":serverMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
					":clientMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(submission.ClientMs, 10)},
				},
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.BuildDailySchedulePK(playOriginDate)},
					"SK": &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
				},
				UpdateExpression: aws.String("ADD #counters.#solved :one"),
				ExpressionAttributeNames: map[string]string{
					"#counters": "counters",
					"#solved":   repository.ScheduleCounterSolved,
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":one": &types.AttributeValueMemberN{Value: "1"},
				},
			},
		},
	}

	if !submission.IsAnonymous {
		items = append(items, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(s.tableName),
				Item: map[string]types.AttributeValue{
					"PK":              &types.AttributeValueMemberS{Value: repository.BuildDailyLeaderboardPK(playOriginDate)},
					"SK":              &types.AttributeValueMemberS{Value: repository.BuildLeaderboardSK(elapsedMs, submission.UserID)},
					"userId":          &types.AttributeValueMemberS{Value: submission.UserID},
					"serverElapsedMs": &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
					"submittedAt":     &types.AttributeValueMemberS{Value: submittedAt},
					"puzzleId":        &types.AttributeValueMemberS{Value: submission.PuzzleID},
				},
			},
		})
	}

	if err := s.store.WriteTransaction(ctx, items); err != nil {
		if repository.IsConditionalCheckFailureOnLeg(err, 0) {
			return repository.ErrPlayNotInStartedState
		}
		return fmt.Errorf("submitting daily play %s/%s: %w", playerID, date, err)
	}
	return nil
}

// errNotImplemented is the placeholder body for the two skeleton
// methods. Tasks 43b (SubmitDaily) and 43c (GetDaily) replace these.
var errNotImplemented = errors.New("not implemented")

// GetDaily returns the daily view for (playerID, date). Implementation
// in task 43c.
func (s *Service) GetDaily(ctx context.Context, in GetInput) (*DailyView, error) {
	return nil, errNotImplemented
}

// SubmitDaily records a solved daily play and returns the elapsed time
// + leaderboard rank. Implementation in task 43b.
func (s *Service) SubmitDaily(ctx context.Context, in SubmitInput) (*SubmitResult, error) {
	return nil, errNotImplemented
}
