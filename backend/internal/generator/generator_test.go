package generator

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	// Note: 9x9+ tests are omitted because the brute-force solver is too slow.
	// The constraint propagation solver (GN-03) will handle larger grids.
	tests := []struct {
		name       string
		gridSize   int
		iterations int
		timeout    time.Duration
	}{
		{
			name:       "5x5 puzzle",
			gridSize:   5,
			iterations: 5,
			timeout:    30 * time.Second,
		},
		{
			name:       "7x7 puzzle (slow - brute force solver)",
			gridSize:   7,
			iterations: 1,
			timeout:    120 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.gridSize > 5 && testing.Short() {
				t.Skipf("skipping %dx%d in short mode (brute-force solver is slow for larger grids)", tt.gridSize, tt.gridSize)
			}
			for i := 0; i < tt.iterations; i++ {
				t.Run("iteration", func(t *testing.T) {
					// Arrange
					gridSize := tt.gridSize

					// Act
					solver := NewBacktrackSolver()
					regions := NewBFSRegionGenerator()
					pipeline := NewIterativeRefinementPipeline(solver, regions)
					opts := GenerateOpts{Timeout: tt.timeout}
					puzzle, err := pipeline.Generate(gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}

					if puzzle.GridSize != gridSize {
						t.Errorf("GridSize = %d, want %d", puzzle.GridSize, gridSize)
					}

					if puzzle.Mode != "" {
						t.Errorf("Mode = %q, want empty string", puzzle.Mode)
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

					solCount := CountSolutions(puzzle.RegionMap, gridSize, 1)
					if solCount != 1 {
						t.Errorf("CountSolutions = %d, want 1", solCount)
					}

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
				})
			}
		})
	}
}

func TestGenerateTimeout(t *testing.T) {
	// Arrange
	solver := NewBacktrackSolver()
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
