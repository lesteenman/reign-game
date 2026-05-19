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

// submitDailyFakeStore is a configurable Store for SubmitDaily tests.
// Every Store method has a function field; nil fields fall back to safe
// no-op defaults (nil return / nil error).
type submitDailyFakeStore struct {
	getScheduleFunc             func(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	getPuzzleFunc               func(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	getPlayFunc                 func(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	writeTransactionFunc        func(ctx context.Context, items []types.TransactWriteItem) error
	leaderboardRankFunc         func(ctx context.Context, date string, elapsedMs int64, userID string) (int, error)
	submitPlayTransactionallyFn func(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error
}

func (f *submitDailyFakeStore) GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error) {
	if f.getScheduleFunc != nil {
		return f.getScheduleFunc(ctx, date)
	}
	return nil, nil
}
func (f *submitDailyFakeStore) GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error) {
	if f.getPuzzleFunc != nil {
		return f.getPuzzleFunc(ctx, size, mode, puzzleID)
	}
	return nil, nil
}
func (f *submitDailyFakeStore) GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error) {
	if f.getPlayFunc != nil {
		return f.getPlayFunc(ctx, playerID, date)
	}
	return nil, nil
}
func (f *submitDailyFakeStore) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	return nil, nil
}
func (f *submitDailyFakeStore) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	return nil
}
func (f *submitDailyFakeStore) WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error {
	if f.writeTransactionFunc != nil {
		return f.writeTransactionFunc(ctx, items)
	}
	return nil
}
func (f *submitDailyFakeStore) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *submitDailyFakeStore) PutCandidateIfAbsent(_ context.Context, _, _ string) error { return nil }
func (f *submitDailyFakeStore) SubmitPlayTransactionally(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error {
	if f.submitPlayTransactionallyFn != nil {
		return f.submitPlayTransactionallyFn(ctx, playerID, date, submission)
	}
	return nil
}
func (f *submitDailyFakeStore) LeaderboardRank(ctx context.Context, date string, elapsedMs int64, userID string) (int, error) {
	if f.leaderboardRankFunc != nil {
		return f.leaderboardRankFunc(ctx, date, elapsedMs, userID)
	}
	return 0, nil
}

// submitTestSchedule returns a ScheduleRecord ready for submit tests.
func submitTestSchedule() *repository.ScheduleRecord {
	return &repository.ScheduleRecord{
		Date:            testDate,
		PuzzleID:        "puzzle-uuid-1",
		SourcePartition: "9#standard",
		AssignedAt:      "2026-05-16T00:00:00Z",
	}
}

// submitTestPuzzle returns a 2×2 PuzzleRecord whose Solution matches
// validSolution below.
func submitTestPuzzle() *repository.PuzzleRecord {
	return &repository.PuzzleRecord{
		GridSize:  2,
		RegionMap: [][]int{{1, 2}, {1, 2}},
		Solution:  [][]bool{{true, false}, {false, true}},
	}
}

// validSolution is a 2×2 grid matching submitTestPuzzle.Solution.
var validSolution = [][]int{{1, 0}, {0, 1}}

// submitTestPlay returns a PlayRecord in the "started" state with an
// assignedAt 2 hours before testNow so serverElapsedMs is positive.
func submitTestPlay(playerID string) *repository.PlayRecord {
	return &repository.PlayRecord{
		PlayerID:   playerID,
		Date:       testDate,
		PuzzleID:   "puzzle-uuid-1",
		Outcome:    repository.PlayOutcomeStarted,
		AssignedAt: "2026-05-16T10:00:00Z",
	}
}

// buildSubmitStore assembles a store with the "happy-path" defaults for
// SubmitDaily so individual tests only need to override what they care about.
func buildSubmitStore(overrides *submitDailyFakeStore) *submitDailyFakeStore {
	store := &submitDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return submitTestSchedule(), nil
		},
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return submitTestPuzzle(), nil
		},
		getPlayFunc: func(_ context.Context, playerID, _ string) (*repository.PlayRecord, error) {
			return submitTestPlay(playerID), nil
		},
		leaderboardRankFunc: func(_ context.Context, _ string, _ int64, _ string) (int, error) {
			return 3, nil
		},
	}
	if overrides.getScheduleFunc != nil {
		store.getScheduleFunc = overrides.getScheduleFunc
	}
	if overrides.getPuzzleFunc != nil {
		store.getPuzzleFunc = overrides.getPuzzleFunc
	}
	if overrides.getPlayFunc != nil {
		store.getPlayFunc = overrides.getPlayFunc
	}
	if overrides.writeTransactionFunc != nil {
		store.writeTransactionFunc = overrides.writeTransactionFunc
	}
	if overrides.leaderboardRankFunc != nil {
		store.leaderboardRankFunc = overrides.leaderboardRankFunc
	}
	if overrides.submitPlayTransactionallyFn != nil {
		store.submitPlayTransactionallyFn = overrides.submitPlayTransactionallyFn
	}
	return store
}

func TestSubmitDaily_HappyPath_SignedIn(t *testing.T) {
	// Arrange
	store := buildSubmitStore(&submitDailyFakeStore{})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  7200000,
		Solution:    validSolution,
	}

	// Act
	result, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(signed-in) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("SubmitDaily returned nil result")
	}
	if result.ServerElapsedMs <= 0 {
		t.Errorf("ServerElapsedMs = %d, want > 0", result.ServerElapsedMs)
	}
	if result.LeaderboardRank == nil {
		t.Fatal("LeaderboardRank should be non-nil for signed-in player")
	}
	if *result.LeaderboardRank != 3 {
		t.Errorf("LeaderboardRank = %d, want 3", *result.LeaderboardRank)
	}
}

func TestSubmitDaily_HappyPath_Anonymous(t *testing.T) {
	// Arrange
	store := buildSubmitStore(&submitDailyFakeStore{})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "device-xyz",
		IsAnonymous: true,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	result, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(anonymous) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("SubmitDaily returned nil result")
	}
	if result.ServerElapsedMs <= 0 {
		t.Errorf("ServerElapsedMs = %d, want > 0", result.ServerElapsedMs)
	}
	if result.LeaderboardRank != nil {
		t.Errorf("LeaderboardRank should be nil for anonymous player; got %d", *result.LeaderboardRank)
	}
}

func TestSubmitDaily_LeaderboardFetchError_SubmitSucceedsRankNil(t *testing.T) {
	// Arrange — leaderboard fetch fails; submit should still succeed (best-effort).
	rankErr := errors.New("leaderboard DDB error")
	store := buildSubmitStore(&submitDailyFakeStore{
		leaderboardRankFunc: func(_ context.Context, _ string, _ int64, _ string) (int, error) {
			return 0, rankErr
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	result, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(rank-error) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("SubmitDaily returned nil result")
	}
	if result.LeaderboardRank != nil {
		t.Errorf("LeaderboardRank should be nil on rank-fetch error; got %d", *result.LeaderboardRank)
	}
}

func TestSubmitDaily_RaceLosser_ReturnsErrAlreadySolved(t *testing.T) {
	// Arrange — WriteTransaction leg 0 fails its condition: race-loser.
	store := buildSubmitStore(&submitDailyFakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			code := "ConditionalCheckFailed"
			return &types.TransactionCanceledException{
				CancellationReasons: []types.CancellationReason{
					{Code: &code},
					{Code: func() *string { s := "None"; return &s }()},
				},
			}
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected ErrAlreadySolved, got nil")
	}
	if !errors.Is(err, daily.ErrAlreadySolved) {
		t.Errorf("error = %v, want daily.ErrAlreadySolved", err)
	}
}

func TestSubmitDaily_SubmitPlayGenericError_ReturnsWrappedError(t *testing.T) {
	// Arrange — WriteTransaction returns a non-sentinel error.
	ddbErr := errors.New("dynamodb throttled")
	store := buildSubmitStore(&submitDailyFakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			return ddbErr
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, daily.ErrAlreadySolved) {
		t.Errorf("generic error must NOT map to ErrAlreadySolved; got %v", err)
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}

func TestSubmitDaily_PlayNil_ReturnsErrPlayNotStarted(t *testing.T) {
	// Arrange — no play row for this player/date.
	store := buildSubmitStore(&submitDailyFakeStore{
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return nil, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrPlayNotStarted) {
		t.Errorf("error = %v, want daily.ErrPlayNotStarted", err)
	}
}

func TestSubmitDaily_PlayAlreadySolved_ReturnsErrAlreadySolved(t *testing.T) {
	// Arrange — play row already in solved state.
	store := buildSubmitStore(&submitDailyFakeStore{
		getPlayFunc: func(_ context.Context, playerID, _ string) (*repository.PlayRecord, error) {
			play := submitTestPlay(playerID)
			play.Outcome = repository.PlayOutcomeSolved
			return play, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrAlreadySolved) {
		t.Errorf("error = %v, want daily.ErrAlreadySolved", err)
	}
}

func TestSubmitDaily_GetScheduleError_ReturnsWrappedError(t *testing.T) {
	// Arrange — schedule read fails.
	ddbErr := errors.New("schedule read throttled")
	store := buildSubmitStore(&submitDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return nil, ddbErr
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("error should wrap schedule error; got %v", err)
	}
}

func TestSubmitDaily_ScheduleNilWithPlayPresent_ReturnsWrappedError(t *testing.T) {
	// Arrange — schedule is nil but play exists: invariant violation.
	store := buildSubmitStore(&submitDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			return nil, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error for missing schedule, got nil")
	}
	// Must NOT be a sentinel — handler maps to 500.
	for _, sentinel := range []error{daily.ErrInvalidDate, daily.ErrOutOfWindow, daily.ErrPlayNotStarted, daily.ErrAlreadySolved, daily.ErrInvalidSolution, daily.ErrNegativeClockSkew} {
		if errors.Is(err, sentinel) {
			t.Errorf("schedule_missing should NOT be sentinel %v; got %v", sentinel, err)
		}
	}
}

func TestSubmitDaily_SourcePartitionMalformed_ReturnsWrappedError(t *testing.T) {
	// Arrange — schedule has malformed sourcePartition.
	store := buildSubmitStore(&submitDailyFakeStore{
		getScheduleFunc: func(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
			sched := submitTestSchedule()
			sched.SourcePartition = "bad-partition"
			return sched, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error for malformed sourcePartition, got nil")
	}
	// Must be a wrapped, non-sentinel error.
	for _, sentinel := range []error{daily.ErrInvalidDate, daily.ErrOutOfWindow, daily.ErrPlayNotStarted, daily.ErrAlreadySolved, daily.ErrInvalidSolution, daily.ErrNegativeClockSkew} {
		if errors.Is(err, sentinel) {
			t.Errorf("source_partition_malformed should NOT be sentinel %v; got %v", sentinel, err)
		}
	}
}

func TestSubmitDaily_PuzzleNil_ReturnsWrappedError(t *testing.T) {
	// Arrange — GetPuzzle returns nil (schedule points at a nonexistent puzzle).
	store := buildSubmitStore(&submitDailyFakeStore{
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return nil, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error for nil puzzle, got nil")
	}
	for _, sentinel := range []error{daily.ErrInvalidDate, daily.ErrOutOfWindow, daily.ErrPlayNotStarted, daily.ErrAlreadySolved, daily.ErrInvalidSolution, daily.ErrNegativeClockSkew} {
		if errors.Is(err, sentinel) {
			t.Errorf("puzzle_missing should NOT be sentinel %v; got %v", sentinel, err)
		}
	}
}

func TestSubmitDaily_GetPuzzleError_ReturnsWrappedError(t *testing.T) {
	// Arrange — GetPuzzle returns a DDB error.
	ddbErr := errors.New("puzzle read throttled")
	store := buildSubmitStore(&submitDailyFakeStore{
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return nil, ddbErr
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("error should wrap puzzle error; got %v", err)
	}
}

func TestSubmitDaily_SolutionShape_InvalidCases(t *testing.T) {
	// Table-driven: various invalid solution shapes all → ErrInvalidSolution.
	cases := []struct {
		name     string
		solution [][]int
	}{
		{"empty grid", [][]int{}},
		{"empty row", [][]int{{}}},
		{"jagged rows", [][]int{{1, 0}, {0}}},
		{"out-of-range cell (2)", [][]int{{1, 2}, {0, 1}}},
		{"negative cell", [][]int{{-1, 0}, {0, 1}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			store := buildSubmitStore(&submitDailyFakeStore{})
			svc := daily.New(store, "test-table", fixedClock(testNow), nil)
			in := daily.SubmitInput{
				PlayerID:    "player-1",
				IsAnonymous: false,
				Date:        testDate,
				PlayTimeMs:  5000,
				Solution:    tc.solution,
			}

			// Act
			_, err := svc.SubmitDaily(context.Background(), in)

			// Assert
			if !errors.Is(err, daily.ErrInvalidSolution) {
				t.Errorf("solution shape %q: error = %v, want daily.ErrInvalidSolution", tc.name, err)
			}
		})
	}
}

func TestSubmitDaily_SolutionMismatch_ReturnsErrInvalidSolution(t *testing.T) {
	// Arrange — correct shape, wrong cells (all zeros instead of diagonal).
	wrongSolution := [][]int{{0, 0}, {0, 0}}
	store := buildSubmitStore(&submitDailyFakeStore{})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    wrongSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrInvalidSolution) {
		t.Errorf("error = %v, want daily.ErrInvalidSolution", err)
	}
}

func TestSubmitDaily_BadAssignedAtFormat_ReturnsWrappedError(t *testing.T) {
	// Arrange — play row has a non-RFC3339 assignedAt.
	store := buildSubmitStore(&submitDailyFakeStore{
		getPlayFunc: func(_ context.Context, playerID, _ string) (*repository.PlayRecord, error) {
			play := submitTestPlay(playerID)
			play.AssignedAt = "not-a-timestamp"
			return play, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("expected error for bad assignedAt, got nil")
	}
	for _, sentinel := range []error{daily.ErrInvalidDate, daily.ErrOutOfWindow, daily.ErrPlayNotStarted, daily.ErrAlreadySolved, daily.ErrInvalidSolution, daily.ErrNegativeClockSkew} {
		if errors.Is(err, sentinel) {
			t.Errorf("bad assignedAt should NOT be sentinel %v; got %v", sentinel, err)
		}
	}
}

func TestSubmitDaily_NegativeClockSkew_ReturnsErrNegativeClockSkew(t *testing.T) {
	// Arrange — assignedAt is in the future relative to the fixed clock.
	futureAssignedAt := testNow.Add(1 * time.Hour).Format(time.RFC3339)
	store := buildSubmitStore(&submitDailyFakeStore{
		getPlayFunc: func(_ context.Context, playerID, _ string) (*repository.PlayRecord, error) {
			play := submitTestPlay(playerID)
			play.AssignedAt = futureAssignedAt
			return play, nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: testDate, PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrNegativeClockSkew) {
		t.Errorf("error = %v, want daily.ErrNegativeClockSkew", err)
	}
}

func TestSubmitDaily_NegativePlayTimeMs_ReturnsErrInvalidPlayTime(t *testing.T) {
	// Arrange
	store := buildSubmitStore(&submitDailyFakeStore{})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  -1,
		Solution:    validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrInvalidPlayTime) {
		t.Errorf("error = %v, want daily.ErrInvalidPlayTime", err)
	}
}

func TestSubmitDaily_InvalidDateFormat_ReturnsErrInvalidDate(t *testing.T) {
	// Arrange
	store := &submitDailyFakeStore{}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: "not-a-date", PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrInvalidDate) {
		t.Errorf("error = %v, want daily.ErrInvalidDate", err)
	}
}

func TestSubmitDaily_DateOutsideWindow_ReturnsErrOutOfWindow(t *testing.T) {
	// Arrange — date is two days ago.
	store := &submitDailyFakeStore{}
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	in := daily.SubmitInput{
		PlayerID: "player-1", Date: "2026-05-14", PlayTimeMs: 5000, Solution: validSolution,
	}

	// Act
	_, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if !errors.Is(err, daily.ErrOutOfWindow) {
		t.Errorf("error = %v, want daily.ErrOutOfWindow", err)
	}
}
