package generator

import (
	"testing"
	"time"
)

// assertPuzzleValid is a test helper that validates a generated puzzle.
// Pipelines return Mode="" because mode stamping is the handler's responsibility.
func assertPuzzleValid(t *testing.T, puzzle *PuzzleResult, gridSize int) {
	t.Helper()

	if puzzle.GridSize != gridSize {
		t.Errorf("GridSize = %d, want %d", puzzle.GridSize, gridSize)
	}

	if puzzle.Mode != "" {
		t.Errorf("Mode = %q, want empty string (handler sets mode)", puzzle.Mode)
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
					opts := GenerateOpts{Timeout: tt.timeout}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize)
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
					opts := GenerateOpts{Timeout: tt.timeout}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize)
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
					opts := GenerateOpts{Timeout: tt.timeout}

					// Act
					puzzle, err := pipeline.Generate(tt.gridSize, 1, opts)

					// Assert
					if err != nil {
						t.Fatalf("Generate returned error: %v", err)
					}
					assertPuzzleValid(t, &PuzzleResult{
						ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
						RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
					}, tt.gridSize)
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
	var _ PipelineStrategy = NewRegionFirstPipeline(NewPropagationSolver())
	var _ PipelineStrategy = NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	var _ PipelineStrategy = NewConstraintAwarePipeline(NewPropagationSolver())
}

// assertPuzzleValidDouble is a test helper that validates a generated Double
// Queens puzzle: 2 markers per row, per column, and per region, plus no
// adjacency violations and unique solution.
func assertPuzzleValidDouble(t *testing.T, puzzle *PuzzleResult, gridSize int) {
	t.Helper()

	if puzzle.GridSize != gridSize {
		t.Errorf("GridSize = %d, want %d", puzzle.GridSize, gridSize)
	}

	if puzzle.Mode != "" {
		t.Errorf("Mode = %q, want empty string (handler sets mode)", puzzle.Mode)
	}

	if puzzle.ID != "" {
		t.Errorf("ID = %q, want empty string", puzzle.ID)
	}

	// Validate region map with double-mode min size.
	if err := ValidateRegionMapWithMinSize(puzzle.RegionMap, gridSize, minRegionSizeDouble); err != nil {
		t.Fatalf("ValidateRegionMapWithMinSize failed: %v", err)
	}

	// Count total markers.
	totalMarkers := 0
	for _, row := range puzzle.Solution {
		for _, cell := range row {
			if cell {
				totalMarkers++
			}
		}
	}
	expectedMarkers := 2 * gridSize
	if totalMarkers != expectedMarkers {
		t.Errorf("solution has %d markers, want %d", totalMarkers, expectedMarkers)
	}

	// Verify 2 markers per row.
	for r := 0; r < gridSize; r++ {
		count := 0
		for c := 0; c < gridSize; c++ {
			if puzzle.Solution[r][c] {
				count++
			}
		}
		if count != 2 {
			t.Errorf("row %d has %d markers, want 2", r, count)
		}
	}

	// Verify 2 markers per column.
	for c := 0; c < gridSize; c++ {
		count := 0
		for r := 0; r < gridSize; r++ {
			if puzzle.Solution[r][c] {
				count++
			}
		}
		if count != 2 {
			t.Errorf("column %d has %d markers, want 2", c, count)
		}
	}

	// Verify 2 markers per region.
	regionMarkers := make(map[int]int)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if puzzle.Solution[r][c] {
				rid := puzzle.RegionMap[r][c]
				regionMarkers[rid]++
			}
		}
	}
	for rid := 0; rid < gridSize; rid++ {
		if regionMarkers[rid] != 2 {
			t.Errorf("region %d has %d markers, want 2", rid, regionMarkers[rid])
		}
	}

	// Verify no two markers are adjacent (8-directional).
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if !puzzle.Solution[r][c] {
				continue
			}
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					if dr == 0 && dc == 0 {
						continue
					}
					nr, nc := r+dr, c+dc
					if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
						if puzzle.Solution[nr][nc] {
							t.Errorf("adjacent markers at (%d,%d) and (%d,%d)", r, c, nr, nc)
						}
					}
				}
			}
		}
	}

	// Verify unique solution.
	solver := NewPropagationSolver()
	solCount := solver.CountSolutions(puzzle.RegionMap, gridSize, 2, 2)
	if solCount != 1 {
		t.Errorf("CountSolutions(markersPerUnit=2) = %d, want 1", solCount)
	}
}

func TestRegionFirstPipeline_DoubleQueens_9x9(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Double Queens generation in short mode")
	}

	// Arrange
	solver := NewPropagationSolver()
	pipeline := NewRegionFirstPipeline(solver)
	opts := GenerateOpts{
		Timeout: 60 * time.Second,
		RegionOpts: RegionOpts{
			MinSize: minRegionSizeDouble,
		},
	}

	// Act
	puzzle, err := pipeline.Generate(9, 2, opts)

	// Assert
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertPuzzleValidDouble(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 9)
}

func TestIterativeRefinementPipeline_DoubleQueens_9x9(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Double Queens generation in short mode")
	}

	// Arrange
	solver := NewPropagationSolver()
	regions := NewBFSRegionGeneratorDouble()
	pipeline := NewIterativeRefinementPipeline(solver, regions)
	opts := GenerateOpts{
		Timeout: 60 * time.Second,
		RegionOpts: RegionOpts{
			MinSize: minRegionSizeDouble,
		},
	}

	// Act
	puzzle, err := pipeline.Generate(9, 2, opts)

	// Assert
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertPuzzleValidDouble(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 9)
}

func TestConstraintAwarePipeline_DoubleQueens_8x8(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Double Queens generation in short mode")
	}

	// Arrange: use 8x8 (minimum viable size for double mode).
	// The constraint-aware pipeline is slower than others for double mode
	// because its greedy region growth must handle paired marker seeds.
	solver := NewPropagationSolver()
	pipeline := NewConstraintAwarePipeline(solver)
	opts := GenerateOpts{
		Timeout: 60 * time.Second,
		RegionOpts: RegionOpts{
			MinSize: minRegionSizeDouble,
		},
	}

	// Act
	puzzle, err := pipeline.Generate(8, 2, opts)

	// Assert
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertPuzzleValidDouble(t, &PuzzleResult{
		ID: puzzle.ID, GridSize: puzzle.GridSize, Mode: puzzle.Mode,
		RegionMap: puzzle.RegionMap, Solution: puzzle.Solution,
	}, 8)
}

func TestPairMarkersForDoubleQueens(t *testing.T) {
	// Arrange: create a 5x5 solution with 2 markers per row.
	solution := make([][]bool, 5)
	for r := range solution {
		solution[r] = make([]bool, 5)
	}
	// Place 2 markers per row (10 markers total, 5 regions).
	solution[0][0] = true
	solution[0][3] = true
	solution[1][1] = true
	solution[1][4] = true
	solution[2][0] = true
	solution[2][3] = true
	solution[3][1] = true
	solution[3][4] = true
	solution[4][2] = true
	solution[4][4] = true

	// Act
	pairs, err := pairMarkersForDoubleQueens(solution, 5)

	// Assert
	if err != nil {
		t.Fatalf("pairMarkersForDoubleQueens returned error: %v", err)
	}
	if len(pairs) != 5 {
		t.Fatalf("got %d pairs, want 5", len(pairs))
	}
	// Each pair should have 2 distinct marker positions.
	seen := make(map[[2]int]bool)
	for _, pair := range pairs {
		for _, m := range pair {
			if seen[m] {
				t.Errorf("duplicate marker at (%d,%d)", m[0], m[1])
			}
			seen[m] = true
			if !solution[m[0]][m[1]] {
				t.Errorf("pair member (%d,%d) is not a marker", m[0], m[1])
			}
		}
	}
	if len(seen) != 10 {
		t.Errorf("total unique markers = %d, want 10", len(seen))
	}
}

func TestPairMarkersForDoubleQueens_WrongCount(t *testing.T) {
	// Arrange: solution with only 5 markers (standard mode, not double).
	solution := make([][]bool, 5)
	for r := range solution {
		solution[r] = make([]bool, 5)
	}
	solution[0][0] = true
	solution[1][2] = true
	solution[2][4] = true
	solution[3][1] = true
	solution[4][3] = true

	// Act
	_, err := pairMarkersForDoubleQueens(solution, 5)

	// Assert
	if err == nil {
		t.Fatal("expected error for wrong marker count, got nil")
	}
}
