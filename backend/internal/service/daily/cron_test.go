package daily

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeCronRepo is the Store implementation used by cron tests. Mirrors
// the fakeRepo pattern in sync_test.go but lives in its own type so
// the two test files can evolve independently. The trackedCalls slice
// records call ORDER + method NAMES so determinism / "didn't call X"
// assertions are easy.
type fakeCronRepo struct {
	candidate      *repository.CandidateRecord
	candidateErr   error
	pool           []repository.PuzzleRecord
	poolErr        error
	putErr         error
	putCalledWith  *struct{ puzzleID, sourcePartition string }
	listCalledWith *struct {
		size    int
		mode    string
		exclude bool
		now     time.Time
	}
	trackedCalls []string
}

func (f *fakeCronRepo) GetSchedule(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
	f.trackedCalls = append(f.trackedCalls, "GetSchedule")
	return nil, nil
}

func (f *fakeCronRepo) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	f.trackedCalls = append(f.trackedCalls, "GetCandidate")
	return f.candidate, f.candidateErr
}

func (f *fakeCronRepo) ListApprovedPool(_ context.Context, size int, mode string, exclude bool, now time.Time) ([]repository.PuzzleRecord, error) {
	f.trackedCalls = append(f.trackedCalls, "ListApprovedPool")
	f.listCalledWith = &struct {
		size    int
		mode    string
		exclude bool
		now     time.Time
	}{size, mode, exclude, now}
	return f.pool, f.poolErr
}

func (f *fakeCronRepo) PutCandidateIfAbsent(_ context.Context, puzzleID, sourcePartition string) error {
	f.trackedCalls = append(f.trackedCalls, "PutCandidateIfAbsent")
	f.putCalledWith = &struct{ puzzleID, sourcePartition string }{puzzleID, sourcePartition}
	return f.putErr
}

// Stub methods to satisfy the Store interface — unused by cron tests.
func (f *fakeCronRepo) GetPuzzle(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *fakeCronRepo) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	return nil, nil
}
func (f *fakeCronRepo) PutPlayStartedIfAbsent(_ context.Context, _, _, _ string, _ time.Time) error {
	return nil
}
func (f *fakeCronRepo) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	return nil
}
func (f *fakeCronRepo) WriteTransaction(_ context.Context, _ []types.TransactWriteItem) error {
	return nil
}
func (f *fakeCronRepo) SubmitPlayTransactionally(_ context.Context, _, _ string, _ *repository.SubmitInput) error {
	return nil
}
func (f *fakeCronRepo) LeaderboardRank(_ context.Context, _ string, _ int64, _ string) (int, error) {
	return 0, nil
}

// Compile-time assertion: fakeCronRepo satisfies the Store interface.
var _ Store = (*fakeCronRepo)(nil)

func candidateFixture(puzzleID string, queuedAt time.Time) *repository.CandidateRecord {
	return &repository.CandidateRecord{
		PuzzleID:        puzzleID,
		QueuedAt:        queuedAt.UTC().Format(time.RFC3339),
		SourcePartition: "9#standard",
	}
}

func poolPuzzles(ids ...string) []repository.PuzzleRecord {
	out := make([]repository.PuzzleRecord, 0, len(ids))
	for _, id := range ids {
		out = append(out, repository.PuzzleRecord{ID: id, GridSize: 9, Mode: "standard"})
	}
	return out
}

// newCronTestService constructs a Service backed by the given fakeCronRepo,
// with an optional replenishHook.
func newCronTestService(repo *fakeCronRepo, hook func(int, string)) *Service {
	return New(repo, "test-table", time.Now, hook)
}

func TestEnsureCandidate_FreshExisting_NoOp(t *testing.T) {
	// Arrange
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{candidate: candidateFixture("puzzle-fresh", now.Add(-1*time.Hour))}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range repo.trackedCalls {
		if c == "ListApprovedPool" || c == "PutCandidateIfAbsent" {
			t.Fatalf("fresh existing candidate should skip %s; got calls %v", c, repo.trackedCalls)
		}
	}
}

func TestEnsureCandidate_StaleExisting_Replaces(t *testing.T) {
	// Arrange — 30h-old candidate, eligible pool with two puzzles.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		candidate: candidateFixture("puzzle-stale", now.Add(-30*time.Hour)),
		pool:      poolPuzzles("puzzle-b", "puzzle-a"),
	}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert — picked smallest, called Put with sourcePartition 9#standard.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.putCalledWith == nil {
		t.Fatalf("PutCandidateIfAbsent not called")
	}
	if repo.putCalledWith.puzzleID != "puzzle-a" {
		t.Fatalf("expected puzzle-a, got %s", repo.putCalledWith.puzzleID)
	}
	if repo.putCalledWith.sourcePartition != "9#standard" {
		t.Fatalf("sourcePartition want 9#standard, got %s", repo.putCalledWith.sourcePartition)
	}
}

func TestEnsureCandidate_NoExisting_Picks(t *testing.T) {
	// Arrange
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{pool: poolPuzzles("puzzle-c", "puzzle-a", "puzzle-b")}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.putCalledWith == nil || repo.putCalledWith.puzzleID != "puzzle-a" {
		t.Fatalf("expected pick puzzle-a (lex smallest), got %+v", repo.putCalledWith)
	}
}

func TestEnsureCandidate_DeterministicPick(t *testing.T) {
	// Arrange — same as NoExisting_Picks but explicit about the contract.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{pool: poolPuzzles("puzzle-zzz", "puzzle-001", "puzzle-mid")}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert — lex order: 001 < mid < zzz.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.putCalledWith.puzzleID != "puzzle-001" {
		t.Fatalf("determinism: expected puzzle-001, got %s", repo.putCalledWith.puzzleID)
	}
}

func TestEnsureCandidate_EmptyPool_Sentinel(t *testing.T) {
	// Arrange
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{pool: []repository.PuzzleRecord{}}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert
	if !errors.Is(err, ErrCandidatePoolEmpty) {
		t.Fatalf("expected ErrCandidatePoolEmpty, got %v", err)
	}
	if repo.putCalledWith != nil {
		t.Fatalf("PutCandidateIfAbsent must not be called on empty pool; was called with %+v", repo.putCalledWith)
	}
}

func TestEnsureCandidate_RaceLoser_Nil(t *testing.T) {
	// Arrange — Put fails with the race-loser sentinel.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		pool:   poolPuzzles("puzzle-a"),
		putErr: repository.ErrCandidateAlreadyExists,
	}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert — race-loser cleanly exits.
	if err != nil {
		t.Fatalf("race-loser should return nil, got %v", err)
	}
}

func TestEnsureCandidate_PutGenericFailure_Wrapped(t *testing.T) {
	// Arrange
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	plain := errors.New("ddb broken")
	repo := &fakeCronRepo{pool: poolPuzzles("puzzle-a"), putErr: plain}
	svc := newCronTestService(repo, nil)
	// Act
	err := svc.EnsureCandidate(context.Background(), now)
	// Assert — wrapped, errors.Is finds the underlying.
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, plain) {
		t.Fatalf("expected wrapped %v, got %v (%T)", plain, err, err)
	}
}

// hookCall records one invocation of the replenishHook so tests can
// assert the (size, mode) it received and how often it fired.
type hookCall struct {
	size int
	mode string
}

func captureHook(calls *[]hookCall) func(int, string) {
	return func(size int, mode string) {
		*calls = append(*calls, hookCall{size: size, mode: mode})
	}
}

func TestEnsureCandidate_FiresReplenishHookAfterApprovedPoolRead(t *testing.T) {
	// Arrange — non-empty pool path triggers the hook.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{pool: poolPuzzles("puzzle-a")}
	var calls []hookCall
	svc := newCronTestService(repo, captureHook(&calls))

	// Act
	err := svc.EnsureCandidate(context.Background(), now)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("hook calls = %d, want 1", len(calls))
	}
	if calls[0].size != CandidatePoolSize || calls[0].mode != CandidatePoolMode {
		t.Errorf("hook call = %+v, want {%d %s}", calls[0], CandidatePoolSize, CandidatePoolMode)
	}
}

func TestEnsureCandidate_DoesNotFireHookOnFreshExisting(t *testing.T) {
	// Arrange — fresh existing candidate skips the read; no hook fire.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{candidate: candidateFixture("puzzle-fresh", now.Add(-1*time.Hour))}
	var calls []hookCall
	svc := newCronTestService(repo, captureHook(&calls))

	// Act
	err := svc.EnsureCandidate(context.Background(), now)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("hook must not fire when no read happened; got %d calls", len(calls))
	}
}

func TestEnsureCandidate_DoesNotFireHookOnEmptyPool(t *testing.T) {
	// Arrange — pool empty; ErrCandidatePoolEmpty; no hook fire.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{pool: []repository.PuzzleRecord{}}
	var calls []hookCall
	svc := newCronTestService(repo, captureHook(&calls))

	// Act
	err := svc.EnsureCandidate(context.Background(), now)

	// Assert
	if !errors.Is(err, ErrCandidatePoolEmpty) {
		t.Fatalf("expected ErrCandidatePoolEmpty, got %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("hook must not fire on empty-pool path; got %d calls", len(calls))
	}
}

func TestEnsureCandidate_FiresHookEvenOnRaceLoser(t *testing.T) {
	// Arrange — Put fails with race-loser sentinel; hook should still
	// have fired because the read drained the partition.
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		pool:   poolPuzzles("puzzle-a"),
		putErr: repository.ErrCandidateAlreadyExists,
	}
	var calls []hookCall
	svc := newCronTestService(repo, captureHook(&calls))

	// Act
	err := svc.EnsureCandidate(context.Background(), now)

	// Assert
	if err != nil {
		t.Fatalf("race-loser should return nil, got %v", err)
	}
	if len(calls) != 1 {
		t.Errorf("hook should fire on the read drain regardless of Put outcome; got %d calls", len(calls))
	}
}

// Reference the format helper to make `fmt` import non-empty when
// every other test happens to drop it during edits.
var _ = fmt.Sprintf
