package generator

import (
	"context"
	"testing"
	"time"
)

func TestGenerateConcurrent_ProducesValidPuzzle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent generation test in short mode")
	}

	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{
		Timeout:     30 * time.Second,
		Concurrency: 4,
	}

	// Act
	puzzle, err := GenerateConcurrent(pipeline, 5, 1, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateConcurrent returned error: %v", err)
	}
	assertPuzzleValid(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 5)
}

func TestGenerateConcurrent_ContextCancellation(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{
		Timeout:     30 * time.Second,
		Ctx:         ctx,
		Concurrency: 4,
	}

	// Act
	_, err := GenerateConcurrent(pipeline, 5, 1, opts)

	// Assert
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestGenerateConcurrent_SequentialFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sequential fallback test in short mode")
	}

	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{
		Timeout:     30 * time.Second,
		Concurrency: 1,
	}

	// Act
	puzzle, err := GenerateConcurrent(pipeline, 5, 1, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateConcurrent(concurrency=1) returned error: %v", err)
	}
	assertPuzzleValid(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 5)
}

func TestGenerateConcurrent_ZeroConcurrencyIsSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping zero concurrency test in short mode")
	}

	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{
		Timeout:     30 * time.Second,
		Concurrency: 0,
	}

	// Act
	puzzle, err := GenerateConcurrent(pipeline, 5, 1, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateConcurrent(concurrency=0) returned error: %v", err)
	}
	assertPuzzleValid(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 5)
}
