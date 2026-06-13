package daily_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// TestNew_NilDepsReturnsNonNil verifies that New does not panic and
// returns a non-nil *Service when all optional dependencies are nil.
// (nil store is structurally valid here; real tests in 43b/c substitute
// a fake Store that is actually called.)
func TestNew_NilDepsReturnsNonNil(t *testing.T) {
	// Arrange — nothing to set up

	// Act
	svc := daily.New(nil, "test-table", nil, nil)

	// Assert
	if svc == nil {
		t.Fatal("New(nil, ...) returned nil, want non-nil *Service")
	}
}

// TestNew_WithDepsReturnsNonNil verifies that New returns a non-nil
// *Service when real (stub) dependencies are provided.
func TestNew_WithDepsReturnsNonNil(t *testing.T) {
	// Arrange
	store := &stubStore{}

	// Act
	svc := daily.New(store, "test-table", time.Now, nil)

	// Assert
	if svc == nil {
		t.Fatal("New(store, ...) returned nil, want non-nil *Service")
	}
}

// TestSubmitDaily_ReturnsNotImplemented verifies that the skeleton stub
// returns a non-nil error (the not-implemented sentinel).
func TestSubmitDaily_ReturnsNotImplemented(t *testing.T) {
	// Arrange
	svc := daily.New(&stubStore{}, "test-table", time.Now, nil)
	in := daily.SubmitInput{
		PlayerID:   "player_1",
		Date:       "2026-05-16",
		PlayTimeMs: 30000,
		Solution:   [][]int{{1, 2}, {3, 4}},
	}

	// Act
	result, err := svc.SubmitDaily(context.Background(), in)

	// Assert
	if err == nil {
		t.Error("SubmitDaily returned nil error on stub, want non-nil (not implemented)")
	}
	if result != nil {
		t.Errorf("SubmitDaily returned non-nil result %+v, want nil on stub", result)
	}
}

// stubStore implements Store with zero/nil return values for all methods.
// Used only to satisfy the interface in smoke tests; 43b/c replace it
// with a configurable fake.
type stubStore struct{}

func (s *stubStore) GetSchedule(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
	return nil, nil
}

func (s *stubStore) GetPuzzle(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
	return nil, nil
}

func (s *stubStore) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	return nil, nil
}

func (s *stubStore) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	return nil, nil
}

func (s *stubStore) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	return nil
}

func (s *stubStore) WriteTransaction(_ context.Context, _ []types.TransactWriteItem) error {
	return nil
}

func (s *stubStore) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}

func (s *stubStore) ListReadyPoolNoDownvotes(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}

func (s *stubStore) PutCandidateIfAbsent(_ context.Context, _, _ string) error {
	return nil
}

func (s *stubStore) SubmitPlayTransactionally(_ context.Context, _, _ string, _ *repository.SubmitInput) error {
	return nil
}

func (s *stubStore) LeaderboardRank(_ context.Context, _ string, _ int64, _ string) (int, error) {
	return 0, nil
}
