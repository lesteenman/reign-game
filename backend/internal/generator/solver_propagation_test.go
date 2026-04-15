package generator

import (
	"testing"
)

// TestPropagationSolverCountSolutions runs the same correctness tests as the
// BacktrackSolver to verify both solvers agree on solution counts.
func TestPropagationSolverCountSolutions(t *testing.T) {
	tests := []struct {
		name           string
		regionMap      [][]int
		gridSize       int
		markersPerUnit int
		maxSolutions   int
		want           int
	}{
		{
			name:           "valid 5x5 with exactly 1 solution (standard)",
			regionMap:      validRegionMap5x5,
			gridSize:       5,
			markersPerUnit: 1,
			maxSolutions:   2,
			want:           1,
		},
		{
			name: "impossible - column regions force adjacent markers",
			regionMap: [][]int{
				{0, 1, 2},
				{0, 1, 2},
				{0, 1, 2},
			},
			gridSize:       3,
			markersPerUnit: 1,
			maxSolutions:   2,
			want:           0,
		},
		{
			name: "multiple solutions - 4x4 grid with 2x2 blocks",
			regionMap: [][]int{
				{0, 0, 1, 1},
				{0, 0, 1, 1},
				{2, 2, 3, 3},
				{2, 2, 3, 3},
			},
			gridSize:       4,
			markersPerUnit: 1,
			maxSolutions:   2,
			want:           2,
		},
	}

	solver := NewPropagationSolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test case data defined in table)

			// Act
			got := solver.CountSolutions(tt.regionMap, tt.gridSize, tt.markersPerUnit, tt.maxSolutions)

			// Assert
			if got != tt.want {
				t.Errorf("CountSolutions() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestPropagationSolverGenerateSolution verifies that GenerateSolution produces
// valid placements satisfying all row, column, and adjacency constraints.
func TestPropagationSolverGenerateSolution(t *testing.T) {
	tests := []struct {
		name           string
		gridSize       int
		markersPerUnit int
	}{
		{name: "5x5 standard", gridSize: 5, markersPerUnit: 1},
		{name: "7x7 standard", gridSize: 7, markersPerUnit: 1},
		{name: "9x9 standard", gridSize: 9, markersPerUnit: 1},
	}

	solver := NewPropagationSolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test case data defined in table)

			// Act
			solution := solver.GenerateSolution(tt.gridSize, tt.markersPerUnit)

			// Assert
			if solution == nil {
				t.Fatal("expected non-nil solution")
			}

			// Verify grid dimensions.
			if len(solution) != tt.gridSize {
				t.Fatalf("expected %d rows, got %d", tt.gridSize, len(solution))
			}
			for r, row := range solution {
				if len(row) != tt.gridSize {
					t.Fatalf("row %d: expected %d cols, got %d", r, tt.gridSize, len(row))
				}
			}

			// Collect marker positions.
			var markers [][2]int
			for r := 0; r < tt.gridSize; r++ {
				for c := 0; c < tt.gridSize; c++ {
					if solution[r][c] {
						markers = append(markers, [2]int{r, c})
					}
				}
			}

			expectedTotal := tt.gridSize * tt.markersPerUnit
			if len(markers) != expectedTotal {
				t.Errorf("expected %d markers, got %d", expectedTotal, len(markers))
			}

			// Verify row constraint.
			for r := 0; r < tt.gridSize; r++ {
				count := 0
				for c := 0; c < tt.gridSize; c++ {
					if solution[r][c] {
						count++
					}
				}
				if count != tt.markersPerUnit {
					t.Errorf("row %d has %d markers, want %d", r, count, tt.markersPerUnit)
				}
			}

			// Verify column constraint.
			for c := 0; c < tt.gridSize; c++ {
				count := 0
				for r := 0; r < tt.gridSize; r++ {
					if solution[r][c] {
						count++
					}
				}
				if count != tt.markersPerUnit {
					t.Errorf("column %d has %d markers, want %d", c, count, tt.markersPerUnit)
				}
			}

			// Verify no adjacency.
			for i := 0; i < len(markers); i++ {
				for j := i + 1; j < len(markers); j++ {
					rd := markers[i][0] - markers[j][0]
					cd := markers[i][1] - markers[j][1]
					if rd < 0 {
						rd = -rd
					}
					if cd < 0 {
						cd = -cd
					}
					if rd <= 1 && cd <= 1 {
						t.Errorf("markers at (%d,%d) and (%d,%d) are adjacent",
							markers[i][0], markers[i][1], markers[j][0], markers[j][1])
					}
				}
			}
		})
	}
}

// TestPropagationSolverAgreesWithBacktrack verifies both solvers agree on
// solution counts for several region maps.
func TestPropagationSolverAgreesWithBacktrack(t *testing.T) {
	tests := []struct {
		name           string
		regionMap      [][]int
		gridSize       int
		markersPerUnit int
		maxSolutions   int
	}{
		{
			name:           "5x5 reference map",
			regionMap:      validRegionMap5x5,
			gridSize:       5,
			markersPerUnit: 1,
			maxSolutions:   5,
		},
		{
			name: "4x4 with 2x2 blocks",
			regionMap: [][]int{
				{0, 0, 1, 1},
				{0, 0, 1, 1},
				{2, 2, 3, 3},
				{2, 2, 3, 3},
			},
			gridSize:       4,
			markersPerUnit: 1,
			maxSolutions:   5,
		},
		{
			name: "3x3 impossible",
			regionMap: [][]int{
				{0, 1, 2},
				{0, 1, 2},
				{0, 1, 2},
			},
			gridSize:       3,
			markersPerUnit: 1,
			maxSolutions:   2,
		},
	}

	backtrack := NewBacktrackSolver()
	propagation := NewPropagationSolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test case data defined in table)

			// Act
			btCount := backtrack.CountSolutions(tt.regionMap, tt.gridSize, tt.markersPerUnit, tt.maxSolutions)
			propCount := propagation.CountSolutions(tt.regionMap, tt.gridSize, tt.markersPerUnit, tt.maxSolutions)

			// Assert
			if btCount != propCount {
				t.Errorf("solvers disagree: backtrack=%d, propagation=%d", btCount, propCount)
			}
		})
	}
}

// TestPropagationSolverContradictionDetection verifies that the solver quickly
// detects contradictions and backtracks without exhaustive search.
func TestPropagationSolverContradictionDetection(t *testing.T) {
	t.Run("impossible 3x3 grid returns zero solutions", func(t *testing.T) {
		// Arrange
		// 3x3 column-stripe regions: no valid placement exists because adjacency
		// constraint prevents placing markers in adjacent rows with only 3 columns.
		regionMap := [][]int{
			{0, 1, 2},
			{0, 1, 2},
			{0, 1, 2},
		}
		solver := NewPropagationSolver()

		// Act
		count := solver.CountSolutions(regionMap, 3, 1, 2)

		// Assert
		if count != 0 {
			t.Errorf("expected 0 solutions for impossible grid, got %d", count)
		}
	})
}

// TestPropagationSolverDoubleQueens verifies the solver works with markersPerUnit > 1.
func TestPropagationSolverDoubleQueens(t *testing.T) {
	// Row-stripe 8x8: region[r][c] = r.
	doubleQueensRowStripe8x8 := [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 1, 1, 1, 1, 1, 1},
		{2, 2, 2, 2, 2, 2, 2, 2},
		{3, 3, 3, 3, 3, 3, 3, 3},
		{4, 4, 4, 4, 4, 4, 4, 4},
		{5, 5, 5, 5, 5, 5, 5, 5},
		{6, 6, 6, 6, 6, 6, 6, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
	}

	t.Run("CountSolutions returns 2 for multi-solution Double Queens grid", func(t *testing.T) {
		// Arrange
		solver := NewPropagationSolver()

		// Act
		count := solver.CountSolutions(doubleQueensRowStripe8x8, 8, 2, 2)

		// Assert
		if count != 2 {
			t.Errorf("CountSolutions() = %d, want 2 (capped)", count)
		}
	})

	t.Run("GenerateSolution produces valid Double Queens placement", func(t *testing.T) {
		// Arrange
		solver := NewPropagationSolver()
		gridSize := 8
		markersPerUnit := 2

		// Act
		solution := solver.GenerateSolution(gridSize, markersPerUnit)

		// Assert
		if solution == nil {
			t.Fatal("expected non-nil solution for 8x8 Double Queens")
		}

		// Verify row counts.
		for r := 0; r < gridSize; r++ {
			count := 0
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					count++
				}
			}
			if count != markersPerUnit {
				t.Errorf("row %d has %d markers, want %d", r, count, markersPerUnit)
			}
		}

		// Verify column counts.
		for c := 0; c < gridSize; c++ {
			count := 0
			for r := 0; r < gridSize; r++ {
				if solution[r][c] {
					count++
				}
			}
			if count != markersPerUnit {
				t.Errorf("column %d has %d markers, want %d", c, count, markersPerUnit)
			}
		}

		// Verify no adjacency.
		var markers [][2]int
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					markers = append(markers, [2]int{r, c})
				}
			}
		}
		for i := 0; i < len(markers); i++ {
			for j := i + 1; j < len(markers); j++ {
				rd := markers[i][0] - markers[j][0]
				cd := markers[i][1] - markers[j][1]
				if rd < 0 {
					rd = -rd
				}
				if cd < 0 {
					cd = -cd
				}
				if rd <= 1 && cd <= 1 {
					t.Errorf("markers at (%d,%d) and (%d,%d) are adjacent",
						markers[i][0], markers[i][1], markers[j][0], markers[j][1])
				}
			}
		}
	})
}

// Solver-level benchmarks have been consolidated into benchmarks_test.go.
