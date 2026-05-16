package puzzlestore

import (
	"context"
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeStore records the last PutPuzzle call for inspection.
type fakeStore struct {
	putFunc func(ctx context.Context, puzzle *repository.PuzzleRecord) error
}

func (f *fakeStore) PutPuzzle(ctx context.Context, puzzle *repository.PuzzleRecord) error {
	return f.putFunc(ctx, puzzle)
}

func TestStorePuzzle(t *testing.T) {
	t.Run("builds record correctly and calls store", func(t *testing.T) {
		// Arrange
		var captured *repository.PuzzleRecord
		svc := New(&fakeStore{
			putFunc: func(_ context.Context, puzzle *repository.PuzzleRecord) error {
				captured = puzzle
				return nil
			},
		})

		in := &PuzzleInput{
			ID:                   "test-id",
			GridSize:             7,
			Mode:                 "standard",
			RegionMap:            [][]int{{0, 1}, {2, 3}},
			Solution:             [][]bool{{true, false}, {false, true}},
			Status:               "ready",
			Difficulty:           2,
			MaxTier:              3,
			TierCounts:           []int{0, 1, 2, 3, 4},
			TraceLen:             10,
			GenerationDurationMs: 1234,
			CreatedAt:            "2026-05-16T00:00:00Z",
			Seed:                 42,
		}

		// Act
		err := svc.StorePuzzle(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if captured == nil {
			t.Fatal("PutPuzzle was not called")
		}
		if captured.ID != "test-id" {
			t.Errorf("ID = %q, want %q", captured.ID, "test-id")
		}
		if captured.GridSize != 7 {
			t.Errorf("GridSize = %d, want 7", captured.GridSize)
		}
		if captured.Mode != "standard" {
			t.Errorf("Mode = %q, want %q", captured.Mode, "standard")
		}
		if captured.Difficulty != 2 {
			t.Errorf("Difficulty = %d, want 2", captured.Difficulty)
		}
		if captured.MaxTier != 3 {
			t.Errorf("MaxTier = %d, want 3", captured.MaxTier)
		}
		if len(captured.TierCounts) != 5 {
			t.Errorf("TierCounts length = %d, want 5", len(captured.TierCounts))
		}
		if captured.Seed != 42 {
			t.Errorf("Seed = %d, want 42", captured.Seed)
		}
		if captured.CreatedAt != "2026-05-16T00:00:00Z" {
			t.Errorf("CreatedAt = %q, want %q", captured.CreatedAt, "2026-05-16T00:00:00Z")
		}
	})

	t.Run("propagates store error", func(t *testing.T) {
		// Arrange
		storeErr := errors.New("dynamodb unavailable")
		svc := New(&fakeStore{
			putFunc: func(_ context.Context, _ *repository.PuzzleRecord) error {
				return storeErr
			},
		})

		// Act
		err := svc.StorePuzzle(context.Background(), &PuzzleInput{ID: "err-id"})

		// Assert
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, storeErr) {
			t.Errorf("error %v does not wrap %v", err, storeErr)
		}
	})
}
