package daily

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeCronRepo is the Repo implementation used by cron tests. Mirrors
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

	// unused by cron tests — exist only to satisfy the Repo interface.
	scheduleByDate map[string]*repository.ScheduleRecord
	finalizeErr    error
}

func (f *fakeCronRepo) GetSchedule(_ context.Context, date string) (*repository.ScheduleRecord, error) {
	f.trackedCalls = append(f.trackedCalls, "GetSchedule")
	return f.scheduleByDate[date], nil
}

func (f *fakeCronRepo) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	f.trackedCalls = append(f.trackedCalls, "GetCandidate")
	return f.candidate, f.candidateErr
}

func (f *fakeCronRepo) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	f.trackedCalls = append(f.trackedCalls, "FinalizeDailyTransaction")
	return f.finalizeErr
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

func TestEnsureCandidate_FreshExisting_NoOp(t *testing.T) {
	// Arrange
	now := time.Date(2026, 5, 2, 18, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{candidate: candidateFixture("puzzle-fresh", now.Add(-1*time.Hour))}
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
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
	// Act
	err := EnsureCandidate(context.Background(), repo, now)
	// Assert — wrapped, errors.Is finds the underlying.
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, plain) {
		t.Fatalf("expected wrapped %v, got %v (%T)", plain, err, err)
	}
}

// Compile-time assertion: fakeCronRepo satisfies the Repo interface.
var _ Repo = (*fakeCronRepo)(nil)

// Reference the format helper to make `fmt` import non-empty when
// every other test happens to drop it during edits.
var _ = fmt.Sprintf
