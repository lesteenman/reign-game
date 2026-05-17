package verdict_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/verdict"
)

// stubStore is a hand-rolled in-memory Store for unit tests.
type stubStore struct {
	puzzle        *repository.PuzzleRecord
	getPuzzleErr  error
	putVerdictErr error
	summary       repository.VerdictSummary
	recomputeErr  error

	// observation: was PutVerdict called?
	putVerdictCalled bool
	// observation: the record handed to PutVerdict (asserts the
	// service correctly builds the row from SubmitInput fields).
	putVerdictRecord *repository.VerdictRecord
}

func (s *stubStore) GetPuzzle(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
	return s.puzzle, s.getPuzzleErr
}

func (s *stubStore) PutVerdict(_ context.Context, v *repository.VerdictRecord) error {
	s.putVerdictCalled = true
	s.putVerdictRecord = v
	return s.putVerdictErr
}

func (s *stubStore) RecomputeVerdictSummary(_ context.Context, _ int, _, _ string) (repository.VerdictSummary, error) {
	return s.summary, s.recomputeErr
}

// fakePuzzle returns a minimal PuzzleRecord for seeding the stub.
func fakePuzzle() *repository.PuzzleRecord {
	return &repository.PuzzleRecord{GridSize: 7, Mode: "standard", ID: "abc", Status: "ready"}
}

// fakeInput returns a minimal SubmitInput suitable for Service.Submit calls.
func fakeInput() *verdict.SubmitInput {
	return &verdict.SubmitInput{
		GridSize:      7,
		Mode:          "standard",
		PuzzleID:      "abc",
		RaterRole:     "admin",
		RaterID:       "user_abc123",
		Value:         "up",
		PlayTimeMs:    42000,
		Outcome:       "solved",
		ClientVersion: "0.1.0",
	}
}

// TestSubmit_HappyPath verifies that Submit returns the recomputed
// summary when GetPuzzle, PutVerdict, and RecomputeVerdictSummary all
// succeed.
func TestSubmit_HappyPath(t *testing.T) {
	// Arrange
	store := &stubStore{
		puzzle: fakePuzzle(),
		summary: repository.VerdictSummary{
			Up:            3,
			Down:          1,
			LastUpdatedAt: "2026-05-16T10:00:00Z",
		},
	}
	svc := verdict.New(store)

	// Act
	summary, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if err != nil {
		t.Fatalf("Submit returned unexpected error: %v", err)
	}
	if summary.Up != 3 {
		t.Errorf("summary.Up = %d, want 3", summary.Up)
	}
	if summary.Down != 1 {
		t.Errorf("summary.Down = %d, want 1", summary.Down)
	}
	if summary.LastUpdatedAt != "2026-05-16T10:00:00Z" {
		t.Errorf("summary.LastUpdatedAt = %q, want %q", summary.LastUpdatedAt, "2026-05-16T10:00:00Z")
	}
	if !store.putVerdictCalled {
		t.Error("PutVerdict was not called on happy path")
	}
}

// TestSubmit_ConstructsRecordFromInput verifies that the service correctly
// builds the repository.VerdictRecord from SubmitInput fields, so the
// handler can drop the repository import entirely.
func TestSubmit_ConstructsRecordFromInput(t *testing.T) {
	// Arrange
	store := &stubStore{puzzle: fakePuzzle()}
	svc := verdict.New(store)
	in := fakeInput()

	// Act
	_, err := svc.Submit(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("Submit returned unexpected error: %v", err)
	}
	if store.putVerdictRecord == nil {
		t.Fatal("PutVerdict was not invoked with a record")
	}
	got := store.putVerdictRecord
	if got.GridSize != in.GridSize || got.Mode != in.Mode || got.PuzzleID != in.PuzzleID {
		t.Errorf("record key = (%d,%s,%s), want (%d,%s,%s)", got.GridSize, got.Mode, got.PuzzleID, in.GridSize, in.Mode, in.PuzzleID)
	}
	if got.RaterRole != in.RaterRole || got.RaterID != in.RaterID {
		t.Errorf("record rater = (%s,%s), want (%s,%s)", got.RaterRole, got.RaterID, in.RaterRole, in.RaterID)
	}
	if got.Value != in.Value || got.Outcome != in.Outcome || got.PlayTimeMs != in.PlayTimeMs || got.ClientVersion != in.ClientVersion {
		t.Errorf("record payload = (%s,%s,%d,%s), want (%s,%s,%d,%s)",
			got.Value, got.Outcome, got.PlayTimeMs, got.ClientVersion,
			in.Value, in.Outcome, in.PlayTimeMs, in.ClientVersion)
	}
}

// TestSubmit_PuzzleNotFound verifies that Submit returns ErrPuzzleNotFound
// when GetPuzzle returns (nil, nil), and that PutVerdict is NOT called.
func TestSubmit_PuzzleNotFound(t *testing.T) {
	// Arrange
	store := &stubStore{puzzle: nil}
	svc := verdict.New(store)

	// Act
	_, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if !errors.Is(err, verdict.ErrPuzzleNotFound) {
		t.Errorf("err = %v, want ErrPuzzleNotFound", err)
	}
	if store.putVerdictCalled {
		t.Error("PutVerdict must not be called when puzzle is missing")
	}
}

// TestSubmit_GetPuzzleError verifies that a GetPuzzle error is wrapped
// and returned, and that PutVerdict is NOT called.
func TestSubmit_GetPuzzleError(t *testing.T) {
	// Arrange
	sentinel := errors.New("dynamodb timeout")
	store := &stubStore{getPuzzleErr: sentinel}
	svc := verdict.New(store)

	// Act
	_, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrapping sentinel %v", err, sentinel)
	}
	if store.putVerdictCalled {
		t.Error("PutVerdict must not be called when GetPuzzle fails")
	}
}

// TestSubmit_PutVerdictError verifies that a PutVerdict error is wrapped
// and returned.
func TestSubmit_PutVerdictError(t *testing.T) {
	// Arrange
	sentinel := errors.New("dynamodb write failed")
	store := &stubStore{
		puzzle:        fakePuzzle(),
		putVerdictErr: sentinel,
	}
	svc := verdict.New(store)

	// Act
	_, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrapping sentinel %v", err, sentinel)
	}
}

// TestSubmit_RecomputeErrPuzzleNotFoundRace verifies that a benign
// ErrPuzzleNotFound from RecomputeVerdictSummary (puzzle deleted between
// PutVerdict and recompute) causes Submit to return an empty summary and
// nil error — it is NOT an error path.
func TestSubmit_RecomputeErrPuzzleNotFoundRace(t *testing.T) {
	// Arrange
	store := &stubStore{
		puzzle:       fakePuzzle(),
		recomputeErr: repository.ErrPuzzleNotFound,
	}
	svc := verdict.New(store)

	// Act
	summary, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if err != nil {
		t.Errorf("err = %v, want nil (benign race must not fail Submit)", err)
	}
	if summary.Up != 0 || summary.Down != 0 || summary.LastUpdatedAt != "" {
		t.Errorf("summary = %+v, want empty Summary{} on benign race", summary)
	}
	if !store.putVerdictCalled {
		t.Error("PutVerdict must have been called before the recompute race")
	}
}

// TestSubmit_RecomputeOtherError verifies that a non-ErrPuzzleNotFound
// error from RecomputeVerdictSummary also causes Submit to return an
// empty summary and nil error (best-effort recompute).
func TestSubmit_RecomputeOtherError(t *testing.T) {
	// Arrange
	store := &stubStore{
		puzzle:       fakePuzzle(),
		recomputeErr: errors.New("transient DDB error"),
	}
	svc := verdict.New(store)

	// Act
	summary, err := svc.Submit(context.Background(), fakeInput())

	// Assert
	if err != nil {
		t.Errorf("err = %v, want nil (recompute failure must not fail Submit)", err)
	}
	if summary.Up != 0 || summary.Down != 0 || summary.LastUpdatedAt != "" {
		t.Errorf("summary = %+v, want empty Summary{} on recompute failure", summary)
	}
	if !store.putVerdictCalled {
		t.Error("PutVerdict must have been called even when recompute fails")
	}
}
