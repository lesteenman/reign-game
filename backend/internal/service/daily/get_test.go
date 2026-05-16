package daily_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// getDailyFakeStore is a configurable Store for GetDaily tests. Each
// function field controls one Store method; nil fields fall back to
// a default (no-op / nil return).
type getDailyFakeStore struct {
	getScheduleFunc            func(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	getPuzzleFunc              func(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	getPlayFunc                func(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	putPlayStartedIfAbsentFunc func(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error
	getCandidateFunc           func(ctx context.Context) (*repository.CandidateRecord, error)
	finalizeDailyFunc          func(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error
}

func (f *getDailyFakeStore) GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error) {
	if f.getScheduleFunc != nil {
		return f.getScheduleFunc(ctx, date)
	}
	return nil, nil
}
func (f *getDailyFakeStore) GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error) {
	if f.getPuzzleFunc != nil {
		return f.getPuzzleFunc(ctx, size, mode, puzzleID)
	}
	return nil, nil
}
func (f *getDailyFakeStore) GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error) {
	if f.getPlayFunc != nil {
		return f.getPlayFunc(ctx, playerID, date)
	}
	return nil, nil
}
func (f *getDailyFakeStore) PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error {
	if f.putPlayStartedIfAbsentFunc != nil {
		return f.putPlayStartedIfAbsentFunc(ctx, playerID, date, puzzleID, assignedAt)
	}
	return nil
}
func (f *getDailyFakeStore) GetCandidate(ctx context.Context) (*repository.CandidateRecord, error) {
	if f.getCandidateFunc != nil {
		return f.getCandidateFunc(ctx)
	}
	return nil, nil
}
func (f *getDailyFakeStore) FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error {
	if f.finalizeDailyFunc != nil {
		return f.finalizeDailyFunc(ctx, date, puzzleID, sourcePartition, mode)
	}
	return nil
}
func (f *getDailyFakeStore) WriteTransaction(_ context.Context, _ []types.TransactWriteItem) error {
	return nil
}
func (f *getDailyFakeStore) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *getDailyFakeStore) PutCandidateIfAbsent(_ context.Context, _, _ string) error { return nil }
func (f *getDailyFakeStore) SubmitPlayTransactionally(_ context.Context, _, _ string, _ *repository.SubmitInput) error {
	return nil
}
func (f *getDailyFakeStore) LeaderboardRank(_ context.Context, _ string, _ int64, _ string) (int, error) {
	return 0, nil
}

// fixedClock returns a clock function that always returns t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// testDate is today in the test scenarios (YYYY-MM-DD UTC).
const testDate = "2026-05-16"

// testNow is the fixed "now" used across tests: noon UTC on testDate.
var testNow = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

// testSchedule returns a minimal ScheduleRecord for testDate.
func testSchedule() *repository.ScheduleRecord {
	return &repository.ScheduleRecord{
		Date:            testDate,
		PuzzleID:        "puzzle-uuid-1",
		SourcePartition: "9#standard",
		AssignedAt:      "2026-05-16T00:00:00Z",
	}
}

// testPuzzle returns a minimal PuzzleRecord for the schedule above.
func testPuzzle() *repository.PuzzleRecord {
	return &repository.PuzzleRecord{
		GridSize:  9,
		RegionMap: [][]int{{1}},
	}
}

// testStartedPlay returns a PlayRecord in the "started" state.
func testStartedPlay(playerID string) *repository.PlayRecord {
	return &repository.PlayRecord{
		PlayerID:   playerID,
		Date:       testDate,
		PuzzleID:   "puzzle-uuid-1",
		Outcome:    repository.PlayOutcomeStarted,
		AssignedAt: "2026-05-16T10:00:00Z",
	}
}

func TestGetDaily_HappyPath_ExistingStartedPlay(t *testing.T) {
	// Arrange
	play := testStartedPlay("player-1")
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return play, nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("GetDaily unexpected error: %v", err)
	}
	if view == nil {
		t.Fatal("GetDaily returned nil view, want non-nil")
	}
	if view.PuzzleID != "puzzle-uuid-1" {
		t.Errorf("view.PuzzleID = %q, want puzzle-uuid-1", view.PuzzleID)
	}
	if view.Grid != 9 {
		t.Errorf("view.Grid = %d, want 9", view.Grid)
	}
	if view.Outcome != repository.PlayOutcomeStarted {
		t.Errorf("view.Outcome = %q, want %q", view.Outcome, repository.PlayOutcomeStarted)
	}
	if view.ServerElapsedMs != nil {
		t.Error("view.ServerElapsedMs should be nil for started outcome")
	}
	if view.SubmittedAt != nil {
		t.Error("view.SubmittedAt should be nil for started outcome")
	}
}

func TestGetDaily_HappyPath_SolvedPlay(t *testing.T) {
	// Arrange
	elapsed := int64(12345)
	submittedAt := "2026-05-16T10:03:00Z"
	play := &repository.PlayRecord{
		PlayerID:        "player-1",
		Date:            testDate,
		PuzzleID:        "puzzle-uuid-1",
		Outcome:         repository.PlayOutcomeSolved,
		AssignedAt:      "2026-05-16T10:00:00Z",
		ServerElapsedMs: elapsed,
		SubmittedAt:     submittedAt,
	}
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return play, nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("GetDaily(solved) unexpected error: %v", err)
	}
	if view.Outcome != repository.PlayOutcomeSolved {
		t.Errorf("view.Outcome = %q, want solved", view.Outcome)
	}
	if view.ServerElapsedMs == nil || *view.ServerElapsedMs != elapsed {
		t.Errorf("view.ServerElapsedMs = %v, want %d", view.ServerElapsedMs, elapsed)
	}
	if view.SubmittedAt == nil || *view.SubmittedAt != submittedAt {
		t.Errorf("view.SubmittedAt = %v, want %q", view.SubmittedAt, submittedAt)
	}
}

func TestGetDaily_HappyPath_NoExistingPlay_CreatesFresh(t *testing.T) {
	// Arrange — GetPlay returns nil; PutPlayStartedIfAbsent succeeds.
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return nil, nil
		},
		putPlayStartedIfAbsentFunc: func(_ context.Context, _, _, _ string, _ time.Time) error {
			return nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: true, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("GetDaily(no play) unexpected error: %v", err)
	}
	if view == nil {
		t.Fatal("GetDaily returned nil view")
	}
	if view.Outcome != repository.PlayOutcomeStarted {
		t.Errorf("view.Outcome = %q, want started", view.Outcome)
	}
}

func TestGetDaily_RaceLoserPlayCreate_ReturnsWinnerPlay(t *testing.T) {
	// Arrange — GetPlay returns nil initially; PutPlayStartedIfAbsent races
	// → ErrPlayAlreadyExists; second GetPlay returns the winner's row.
	getPlayCalls := 0
	winnerPlay := testStartedPlay("player-1")
	winnerPlay.AssignedAt = "2026-05-16T09:59:00Z" // winner's timestamp

	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			getPlayCalls++
			if getPlayCalls == 1 {
				// First call (from fetchScheduleAndPlay) returns nil.
				return nil, nil
			}
			// Second call (race-loser re-read) returns winner.
			return winnerPlay, nil
		},
		putPlayStartedIfAbsentFunc: func(_ context.Context, _, _, _ string, _ time.Time) error {
			return repository.ErrPlayAlreadyExists
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: true, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("GetDaily(race-loser) unexpected error: %v", err)
	}
	if view == nil {
		t.Fatal("GetDaily returned nil view")
	}
	if view.AssignedAt != winnerPlay.AssignedAt {
		t.Errorf("view.AssignedAt = %q, want winner's %q", view.AssignedAt, winnerPlay.AssignedAt)
	}
}

func TestGetDaily_SyncFallback_NoSchedule_CallsSyncAndReturnsView(t *testing.T) {
	// Arrange — today has no schedule; SyncFinalizeForToday (via
	// FinalizeDailyTransaction) produces one; the subsequent GetSchedule
	// returns it. Uses a getScheduleFunc that returns nil first, then the
	// schedule (simulating what SyncFinalizeForToday does internally).
	scheduleCalls := 0
	sched := testSchedule()

	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, date string) (*repository.ScheduleRecord, error) {
			scheduleCalls++
			// First two calls return nil (today + yesterday from SyncFinalizeForToday),
			// then the re-read after finalize returns the schedule.
			if scheduleCalls <= 2 {
				return nil, nil
			}
			return sched, nil
		},
		getCandidateFunc: func(_ context.Context) (*repository.CandidateRecord, error) {
			// Return a candidate to drive the confirm-mode path.
			return &repository.CandidateRecord{
				PuzzleID:        "puzzle-uuid-1",
				SourcePartition: "9#standard",
			}, nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return testStartedPlay("player-1"), nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("GetDaily(sync-fallback) unexpected error: %v", err)
	}
	if view == nil {
		t.Fatal("GetDaily returned nil view")
	}
	if view.PuzzleID != sched.PuzzleID {
		t.Errorf("view.PuzzleID = %q, want %q", view.PuzzleID, sched.PuzzleID)
	}
}

func TestGetDaily_SyncFallback_PoolExhausted_ReturnsErrPoolExhausted(t *testing.T) {
	// Arrange — no schedule, no candidate, no yesterday schedule.
	// SyncFinalizeForToday will return ErrPoolExhausted.
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return nil, nil
		},
		getCandidateFunc: func(_ context.Context) (*repository.CandidateRecord, error) {
			return nil, nil
		},
		// ListApprovedPool returns empty (drives ErrCandidatePoolEmpty path).
		// Default no-op returns (nil, nil) which is []PuzzleRecord{}, so
		// EnsureCandidate hits pool-empty → ErrPoolExhausted.
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("GetDaily(pool-exhausted) expected error, got nil")
	}
	if !errors.Is(err, daily.ErrPoolExhausted) {
		t.Errorf("error = %v, want daily.ErrPoolExhausted", err)
	}
}

func TestGetDaily_YesterdayScheduleAbsent_ReturnsErrScheduleNotFinalized(t *testing.T) {
	// Arrange — requesting yesterday's date; no schedule exists.
	yesterday := "2026-05-15"
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return nil, nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return nil, nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: yesterday}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected ErrScheduleNotFinalized, got nil")
	}
	if !errors.Is(err, daily.ErrScheduleNotFinalized) {
		t.Errorf("error = %v, want daily.ErrScheduleNotFinalized", err)
	}
}

func TestGetDaily_InvalidDateFormat_ReturnsErrInvalidDate(t *testing.T) {
	// Arrange
	store := &getDailyFakeStore{}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: "not-a-date"}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected ErrInvalidDate, got nil")
	}
	if !errors.Is(err, daily.ErrInvalidDate) {
		t.Errorf("error = %v, want daily.ErrInvalidDate", err)
	}
}

func TestGetDaily_DateOutsideWindow_ReturnsErrOutOfWindow(t *testing.T) {
	// Arrange — requesting a date two days ago.
	store := &getDailyFakeStore{}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: "2026-05-14"}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected ErrOutOfWindow, got nil")
	}
	if !errors.Is(err, daily.ErrOutOfWindow) {
		t.Errorf("error = %v, want daily.ErrOutOfWindow", err)
	}
}

func TestGetDaily_PuzzleMissing_ReturnsNonSentinelError(t *testing.T) {
	// Arrange — schedule exists, but GetPuzzle returns nil (schedule points
	// at a non-existent puzzle — a broken invariant). Handler maps to 500.
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return nil, nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return testStartedPlay("player-1"), nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	view, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error for missing puzzle, got nil")
	}
	if view != nil {
		t.Errorf("expected nil view on error, got %+v", view)
	}
	// Must NOT be any of the sentinel errors — handler maps non-sentinel to 500.
	if errors.Is(err, daily.ErrInvalidDate) || errors.Is(err, daily.ErrOutOfWindow) ||
		errors.Is(err, daily.ErrScheduleNotFinalized) || errors.Is(err, daily.ErrPoolExhausted) {
		t.Errorf("puzzle_missing should NOT be a sentinel error; got %v", err)
	}
}

func TestGetDaily_GetScheduleFails_ReturnsWrappedError(t *testing.T) {
	// Arrange — DDB read for schedule returns an error.
	ddbErr := errors.New("dynamodb throttled")
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return nil, ddbErr
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return nil, nil
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}

func TestGetDaily_PutPlayStartedIfAbsentFails_ReturnsWrappedError(t *testing.T) {
	// Arrange — GetPlay returns nil; PutPlayStartedIfAbsent fails with a
	// non-ErrPlayAlreadyExists error (e.g. a DDB I/O error).
	ddbErr := errors.New("write failed")
	store := &getDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return testSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return testPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return nil, nil
		},
		putPlayStartedIfAbsentFunc: func(_ context.Context, _, _, _ string, _ time.Time) error {
			return ddbErr
		},
	}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.GetInput{PlayerID: "player-1", IsAnonymous: false, Date: testDate}

	// Act
	_, err := svc.GetDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}
