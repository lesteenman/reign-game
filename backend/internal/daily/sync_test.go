package daily

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeRepo records the call sequence and returns canned responses, so
// tests can assert ordering and that FinalizeDailyTransaction is never
// called when ErrPoolExhausted is the expected outcome.
type fakeRepo struct {
	scheduleByDate map[string]*repository.ScheduleRecord
	scheduleErr    map[string]error

	candidate    *repository.CandidateRecord
	candidateErr error

	finalizeErr  error
	finalizeCall *finalizeArgs

	// scheduleByDate snapshot returned AFTER the finalize call commits.
	// Tests use this to simulate the "transaction stamped assignedAt"
	// effect: keys in this map override scheduleByDate after finalize
	// runs.
	scheduleAfterFinalize map[string]*repository.ScheduleRecord

	// approvedPool is what ListApprovedPool returns. Empty/nil keeps
	// pre-bootstrap-era tests intact (they expect ErrCandidatePoolEmpty
	// to bubble up as ErrPoolExhausted from the cold-start path).
	approvedPool []repository.PuzzleRecord

	calls []string
}

type finalizeArgs struct {
	date            string
	puzzleID        string
	sourcePartition string
	mode            repository.FinalizeMode
}

func (f *fakeRepo) GetSchedule(_ context.Context, date string) (*repository.ScheduleRecord, error) {
	f.calls = append(f.calls, "GetSchedule:"+date)
	if err, ok := f.scheduleErr[date]; ok {
		return nil, err
	}
	if f.finalizeCall != nil {
		if rec, ok := f.scheduleAfterFinalize[date]; ok {
			return rec, nil
		}
	}
	return f.scheduleByDate[date], nil
}

func (f *fakeRepo) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	f.calls = append(f.calls, "GetCandidate")
	if f.candidateErr != nil {
		return nil, f.candidateErr
	}
	return f.candidate, nil
}

func (f *fakeRepo) FinalizeDailyTransaction(_ context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error {
	f.calls = append(f.calls, "FinalizeDailyTransaction")
	f.finalizeCall = &finalizeArgs{
		date:            date,
		puzzleID:        puzzleID,
		sourcePartition: sourcePartition,
		mode:            mode,
	}
	return f.finalizeErr
}

// ListApprovedPool returns the seeded pool. Cold-start bootstrap
// tests populate f.approvedPool; pre-bootstrap-era tests leave it
// nil so EnsureCandidate sees an empty pool and surfaces
// ErrCandidatePoolEmpty (which the cold-start path translates back
// to ErrPoolExhausted).
func (f *fakeRepo) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	f.calls = append(f.calls, "ListApprovedPool")
	return f.approvedPool, nil
}

// PutCandidateIfAbsent simulates the conditional Put: only writes
// when no candidate exists. Sets f.candidate so the bootstrap path's
// re-read sees the freshly-written row, mirroring what DDB would
// surface to a follow-up GetItem.
func (f *fakeRepo) PutCandidateIfAbsent(_ context.Context, puzzleID, sourcePartition string) error {
	f.calls = append(f.calls, "PutCandidateIfAbsent")
	if f.candidate == nil {
		f.candidate = &repository.CandidateRecord{
			PuzzleID:        puzzleID,
			SourcePartition: sourcePartition,
		}
	}
	return nil
}

func TestSyncFinalizeForToday_ConfirmCandidate_YesterdayHasSolves(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 100, Solved: 42},
			},
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
			QueuedAt:        "2026-05-01T18:00:00Z",
		},
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-candidate",
				SourcePartition: "9#double",
				AssignedAt:      "2026-05-02T00:00:01Z",
			},
		},
	}

	// Act
	got, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.finalizeCall == nil {
		t.Fatal("expected FinalizeDailyTransaction to be called")
	}
	if repo.finalizeCall.mode != repository.FinalizeModeConfirm {
		t.Errorf("expected mode=confirm, got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-candidate" {
		t.Errorf("expected puzzleID=puzzle-candidate, got %q", repo.finalizeCall.puzzleID)
	}
	if repo.finalizeCall.sourcePartition != "9#double" {
		t.Errorf("expected sourcePartition=9#double, got %q", repo.finalizeCall.sourcePartition)
	}
	if repo.finalizeCall.date != today {
		t.Errorf("expected date=%s, got %q", today, repo.finalizeCall.date)
	}
	if got == nil || got.PuzzleID != "puzzle-candidate" {
		t.Errorf("expected canonical schedule row with puzzle-candidate, got %+v", got)
	}
	if got.AssignedAt != "2026-05-02T00:00:01Z" {
		t.Errorf("expected canonical AssignedAt from re-read, got %q", got.AssignedAt)
	}
}

func TestSyncFinalizeForToday_RecycleYesterday_NoSolves(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 5, Solved: 0},
			},
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
		},
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				AssignedAt:      "2026-05-02T00:00:01Z",
			},
		},
	}

	// Act
	_, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.finalizeCall.mode != repository.FinalizeModeRecycle {
		t.Errorf("expected mode=recycle, got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-yesterday" {
		t.Errorf("expected puzzleID=puzzle-yesterday (recycle), got %q", repo.finalizeCall.puzzleID)
	}
	if repo.finalizeCall.sourcePartition != "9#standard" {
		t.Errorf("expected sourcePartition=9#standard (yesterday's), got %q", repo.finalizeCall.sourcePartition)
	}
}

func TestSyncFinalizeForToday_RecycleYesterday_NoCandidate(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 100, Solved: 50},
			},
		},
		candidate: nil,
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				AssignedAt:      "2026-05-02T00:00:01Z",
			},
		},
	}

	// Act
	_, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.finalizeCall.mode != repository.FinalizeModeRecycle {
		t.Errorf("expected mode=recycle, got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-yesterday" {
		t.Errorf("expected puzzleID=puzzle-yesterday, got %q", repo.finalizeCall.puzzleID)
	}
}

func TestSyncFinalizeForToday_ConfirmCandidate_NoYesterday(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#standard",
		},
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-candidate",
				SourcePartition: "9#standard",
				AssignedAt:      "2026-05-02T00:00:01Z",
			},
		},
	}

	// Act
	_, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.finalizeCall.mode != repository.FinalizeModeConfirm {
		t.Errorf("expected mode=confirm (yesterday missing -> treat solved=0 -> but candidate present), got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-candidate" {
		t.Errorf("expected puzzleID=puzzle-candidate, got %q", repo.finalizeCall.puzzleID)
	}
}

func TestSyncFinalizeForToday_BothAbsent_BootstrapsFromApprovedPool(t *testing.T) {
	// Arrange — cold-start: candidate AND yesterday both absent, but the
	// approved 9_standard pool has a puzzle ready. Sync-fallback should
	// bootstrap a candidate from the pool and then confirm-finalize it,
	// rather than 500ing as ErrPoolExhausted.
	today := "2026-05-08"
	yesterday := "2026-05-07"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{}, // yesterday absent
		candidate:      nil,                                     // candidate absent → cold-start
		approvedPool: []repository.PuzzleRecord{
			{ID: "puzzle-from-pool"},
		},
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-from-pool",
				SourcePartition: "9#standard",
				AssignedAt:      "2026-05-08T00:00:01Z",
			},
		},
	}

	// Act
	canonical, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error after pool bootstrap, got %v", err)
	}
	if repo.finalizeCall == nil {
		t.Fatal("FinalizeDailyTransaction MUST be called after the cold-start bootstrap")
	}
	if repo.finalizeCall.mode != repository.FinalizeModeConfirm {
		t.Errorf("expected mode=confirm (bootstrapped candidate present, yesterday absent), got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-from-pool" {
		t.Errorf("expected puzzleID from approved pool, got %q", repo.finalizeCall.puzzleID)
	}
	if repo.finalizeCall.sourcePartition != "9#standard" {
		t.Errorf("expected sourcePartition=9#standard (CandidatePool constants), got %q", repo.finalizeCall.sourcePartition)
	}
	if canonical == nil || canonical.PuzzleID != "puzzle-from-pool" {
		t.Errorf("expected canonical schedule with bootstrapped puzzle, got %+v", canonical)
	}
}

func TestSyncFinalizeForToday_PoolExhausted(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{},
		candidate:      nil,
	}

	// Act
	got, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil schedule on pool-exhausted, got %+v", got)
	}
	if repo.finalizeCall != nil {
		t.Error("FinalizeDailyTransaction MUST NOT be called when pool is exhausted")
	}
	for _, call := range repo.calls {
		if call == "FinalizeDailyTransaction" {
			t.Fatal("FinalizeDailyTransaction should not appear in call sequence")
		}
	}
}

func TestSyncFinalizeForToday_RaceLoser(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	winnerRow := &repository.ScheduleRecord{
		Date:            today,
		PuzzleID:        "puzzle-winner",
		SourcePartition: "9#standard",
		AssignedAt:      "2026-05-02T00:00:00Z",
	}
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 10, Solved: 5},
			},
			today: winnerRow,
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
		},
		finalizeErr: repository.ErrScheduleAlreadyFinalized,
	}

	// Act
	got, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error on race-loser, got %v", err)
	}
	if got == nil || got.PuzzleID != "puzzle-winner" {
		t.Errorf("expected winner row puzzle-winner, got %+v", got)
	}
	// Verify a follow-up GetSchedule(today) happened after the failed finalize.
	var sawFinalize bool
	var sawTodayReadAfter bool
	for _, call := range repo.calls {
		if call == "FinalizeDailyTransaction" {
			sawFinalize = true
			continue
		}
		if sawFinalize && call == "GetSchedule:"+today {
			sawTodayReadAfter = true
		}
	}
	if !sawTodayReadAfter {
		t.Errorf("expected GetSchedule(today) AFTER finalize on race-loser; calls=%v", repo.calls)
	}
}

func TestSyncFinalizeForToday_RaceLoser_VanishingSchedule(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 10, Solved: 5},
			},
			// today: deliberately absent — race-loser sentinel says it
			// should be there, but our re-read can't find it. System
			// invariant violation.
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
		},
		finalizeErr: repository.ErrScheduleAlreadyFinalized,
	}

	// Act
	got, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err == nil {
		t.Fatal("expected error when schedule vanishes after race-loser, got nil")
	}
	if errors.Is(err, repository.ErrScheduleAlreadyFinalized) {
		t.Errorf("expected wrapped invariant error, not the bare race sentinel; got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil schedule on invariant violation, got %+v", got)
	}
}

func TestSyncFinalizeForToday_TransactionGenericFailure(t *testing.T) {
	// Arrange
	today := "2026-05-02"
	yesterday := "2026-05-01"
	ddbErr := errors.New("ddb broken")
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 10, Solved: 5},
			},
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
		},
		finalizeErr: ddbErr,
	}

	// Act
	got, err := SyncFinalizeForToday(context.Background(), repo, today, yesterday, time.Now())

	// Assert
	if err == nil {
		t.Fatal("expected error on generic finalize failure, got nil")
	}
	if !errors.Is(err, ddbErr) {
		t.Errorf("expected wrapped ddb error, got %v", err)
	}
	if errors.Is(err, ErrPoolExhausted) {
		t.Errorf("must not surface ErrPoolExhausted on generic ddb failure")
	}
	if got != nil {
		t.Errorf("expected nil schedule on error, got %+v", got)
	}
}
