package generator

import (
	"context"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// PipelineStrategy orchestrates puzzle generation end-to-end.
// Different pipelines use fundamentally different approaches to producing
// puzzles with unique solutions.
type PipelineStrategy interface {
	Generate(gridSize, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error)
}

// SolverStrategy generates random solutions and counts valid solutions.
type SolverStrategy interface {
	// GenerateSolution produces a random valid marker placement for the given grid.
	// It returns a gridSize*gridSize boolean grid where true indicates a marker.
	// Returns nil if no valid placement is found.
	GenerateSolution(gridSize int, markersPerUnit int) [][]bool
	// CountSolutions returns the number of valid solutions (stops at maxSolutions).
	CountSolutions(regionMap [][]int, gridSize int, markersPerUnit int, maxSolutions int) int
}

// RegionStrategy grows regions around solution markers.
type RegionStrategy interface {
	// GenerateRegions creates a region map from a solution placement.
	GenerateRegions(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error)
}

// RegionOpts configures region generation.
type RegionOpts struct {
	Variance float64 // 0.0 = uniform sizes, 1.0 = max variation
	MinSize  int     // minimum cells per region (3 for standard, 4 for double)
}

// GenerateOpts configures puzzle generation.
type GenerateOpts struct {
	Timeout     time.Duration   // maximum time to spend generating
	Ctx         context.Context // request context; cancelled on client disconnect
	RegionOpts  RegionOpts      // region generation options
	Deducible   bool            // require puzzles solvable without guessing (naked singles only)
	Concurrency int             // number of parallel generation goroutines (0 or 1 = sequential)
}
