package generator

import (
	"testing"
	"time"
)

// assertPuzzleValid is a test helper that validates a generated puzzle.
func assertPuzzleValid(t *testing.T, puzzle *PuzzleResult, gridSize int, mode string) {
	t.Helper()

	if puzzle.GridSize != gridSize {
		t.Errorf("GridSize = %d, want %d", puzzle.GridSize, gridSize)
	}

	if puzzle.Mode != mode {
		t.Errorf("Mode = %q, want %q", puzzle.Mode, mode)
	}

	if puzzle.ID != "" {
		t.Errorf("ID = %q, want empty string", puzzle.ID)
	}

	if err := ValidateRegionMap(puzzle.RegionMap, gridSize); err != nil {
		t.Fatalf("ValidateRegionMap failed: %v", err)
	}

	markerCount := 0
	for _, row := range puzzle.Solution {
		for _, cell := range row {
			if cell {
				markerCount++
			}
		}
	}
	if markerCount != gridSize {
		t.Errorf("solution has %d markers, want %d", markerCount, gridSize)
	}

	// Verify uniqueness using propagation solver.
	solver := NewPropagationSolver()
	solCount := solver.CountSolutions(puzzle.RegionMap, gridSize, 1, 2)
	if solCount != 1 {
		t.Errorf("CountSolutions = %d, want 1", solCount)
	}

	// Verify one marker per region.
	regionMarkers := make(map[int]int)
	for r, row := range puzzle.Solution {
		for c, cell := range row {
			if cell {
				rid := puzzle.RegionMap[r][c]
				regionMarkers[rid]++
			}
		}
	}
	for rid := 0; rid < gridSize; rid++ {
		if regionMarkers[rid] != 1 {
			t.Errorf("region %d has %d markers, want 1", rid, regionMarkers[rid])
		}
	}
}

// PuzzleResult mirrors model.Puzzle for in-package testing.
type PuzzleResult struct {
	ID        string
	GridSize  int
	Mode      string
	RegionMap [][]int
	Solution  [][]bool
}

func TestSolutionFirstPipeline_5x5(t *testing.T) {
	tests := []struct {
		name     string
		gridSize int
		timeout  time.Duration
	}{
		{
			name:     "5x5 with propagation solver and BFS regions",
			gridSize: 5,
			timeout:  30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			solver := NewPropagationSolver()
			regions := NewBFSRegionGenerator()
			pipeline := NewSolutionFirstPipeline(solver, regions)
			opts := GenerateOpts{Timeout: tt.timeout, Mode: "standard"}

			// Act
			puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

			// Assert
			if err != nil {
				t.Fatalf("Generate returned error: %v", err)
			}
			assertPuzzleValid(t, &PuzzleResult{
				ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
				RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
			}, tt.gridSize, "standard")
		})
	}
}

func TestSolutionFirstPipeline_Timeout(t *testing.T) {
	// Arrange
	solver := NewPropagationSolver()
	regions := NewBFSRegionGenerator()
	pipeline := NewSolutionFirstPipeline(solver, regions)
	opts := GenerateOpts{Timeout: 1 * time.Nanosecond}

	// Act
	_, err := pipeline.Generate(5, 1, opts)

	// Assert
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestRegionFirstPipeline_5x5(t *testing.T) {
	tests := []struct {
		name       string
		gridSize   int
		iterations int
		timeout    time.Duration
	}{
		{
			name:       "5x5 generates valid unique puzzle",
			gridSize:   5,
			iterations: 3,
			timeout:    30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.iterations; i++ {
				t.Run("iteration", func(t *testing.T) {
					// Arrange
					solver := NewPropagationSolver()
					pipeline := NewRegionFirstPipeline(solver)
					opts := GenerateOpts{Timeout: tt.timeout, Mode: "standard"}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize, "standard")
				})
			}
		})
	}
}

func TestRegionFirstPipeline_Timeout(t *testing.T) {
	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{Timeout: 1 * time.Nanosecond}

	// Act
	_, err := pipeline.Generate(5, 1, opts)

	// Assert
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestIterativeRefinementPipeline_5x5(t *testing.T) {
	tests := []struct {
		name       string
		gridSize   int
		iterations int
		timeout    time.Duration
	}{
		{
			name:       "5x5 generates valid unique puzzle",
			gridSize:   5,
			iterations: 3,
			timeout:    30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.iterations; i++ {
				t.Run("iteration", func(t *testing.T) {
					// Arrange
					solver := NewPropagationSolver()
					regions := NewBFSRegionGenerator()
					pipeline := NewIterativeRefinementPipeline(solver, regions)
					opts := GenerateOpts{Timeout: tt.timeout, Mode: "standard"}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize, "standard")
				})
			}
		})
	}
}

func TestIterativeRefinementPipeline_Timeout(t *testing.T) {
	// Arrange
	solver := NewPropagationSolver()
	regions := NewBFSRegionGenerator()
	pipeline := NewIterativeRefinementPipeline(solver, regions)
	opts := GenerateOpts{Timeout: 1 * time.Nanosecond}

	// Act
	_, err := pipeline.Generate(5, 1, opts)

	// Assert
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestConstraintAwarePipeline_5x5(t *testing.T) {
	tests := []struct {
		name       string
		gridSize   int
		iterations int
		timeout    time.Duration
	}{
		{
			name:       "5x5 generates valid unique puzzle",
			gridSize:   5,
			iterations: 3,
			timeout:    30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.iterations; i++ {
				t.Run("iteration", func(t *testing.T) {
					// Arrange
					solver := NewPropagationSolver()
					pipeline := NewConstraintAwarePipeline(solver)
					opts := GenerateOpts{Timeout: tt.timeout, Mode: "standard"}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize, "standard")
				})
			}
		})
	}
}

func TestConstraintAwarePipeline_Timeout(t *testing.T) {
	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewConstraintAwarePipeline(solver)
	opts := GenerateOpts{Timeout: 1 * time.Nanosecond}

	// Act
	_, err := pipeline.Generate(5, 1, opts)

	// Assert
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestDefaultPipeline_5x5(t *testing.T) {
	// Arrange
	pipeline := NewDefaultPipeline()
	opts := GenerateOpts{Timeout: 30 * time.Second, Mode: "standard"}

	// Act
	puzzle, err := pipeline.Generate(5, 1, opts)

	// Assert
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertPuzzleValid(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 5, "standard")
}

func TestGenerateRandomRegions_5x5(t *testing.T) {
	tests := []struct {
		name       string
		gridSize   int
		iterations int
	}{
		{
			name:       "5x5 produces valid region maps",
			gridSize:   5,
			iterations: 10,
		},
		{
			name:       "7x7 produces valid region maps",
			gridSize:   7,
			iterations: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.gridSize > 5 && testing.Short() {
				t.Skip("skipping larger grid in short mode")
			}
			for i := 0; i < tt.iterations; i++ {
				// Arrange
				opts := RegionOpts{}

				// Act
				regionMap, err := GenerateRandomRegions(tt.gridSize, opts)

				// Assert
				if err != nil {
					t.Fatalf("iteration %d: GenerateRandomRegions returned error: %v", i, err)
				}
				if err := ValidateRegionMap(regionMap, tt.gridSize); err != nil {
					t.Fatalf("iteration %d: ValidateRegionMap failed: %v", i, err)
				}
			}
		})
	}
}

func TestPipelineStrategy_Interface(t *testing.T) {
	// Arrange + Assert: verify all pipelines implement PipelineStrategy.
	var _ PipelineStrategy = NewSolutionFirstPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	var _ PipelineStrategy = NewRegionFirstPipeline(NewPropagationSolver())
	var _ PipelineStrategy = NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	var _ PipelineStrategy = NewConstraintAwarePipeline(NewPropagationSolver())
	var _ PipelineStrategy = NewDefaultPipeline()
}
