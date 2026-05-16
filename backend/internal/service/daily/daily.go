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
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// Sentinel errors. The HTTP handler translates these to status codes:
//
//	ErrInvalidDate          -> 400
//	ErrOutOfWindow          -> 404
//	ErrScheduleNotFinalized -> 404 (yesterday's schedule absent)
//	ErrPoolExhausted        -> 500 ("pool exhausted")
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
	ErrPoolExhausted        = errors.New("daily pool exhausted")
	ErrPlayNotStarted       = errors.New("play not started")
	ErrAlreadySolved        = errors.New("already solved")
	ErrInvalidSolution      = errors.New("invalid solution")
	ErrNegativeClockSkew    = errors.New("negative clock skew")
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
	clock         func() time.Time
	replenishHook func(size int, mode string)
}

// New constructs a Service. clock is used in tests to pin "now"; pass
// nil to default to time.Now. replenishHook fires on sync-fallback
// cold-start when the candidate pool drains; nil is acceptable in
// tests and in local-dev wiring without SQS.
func New(store Store, clock func() time.Time, replenishHook func(size int, mode string)) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: store, clock: clock, replenishHook: replenishHook}
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
