package generator

import (
	"testing"
)

func TestCountSolutions(t *testing.T) {
	tests := []struct {
		name           string
		regionMap      [][]int
		gridSize       int
		markersPerUnit int
		want           int
	}{
		{
			name:           "valid 5x5 with exactly 1 solution (standard)",
			regionMap:      validRegionMap5x5,
			gridSize:       5,
			markersPerUnit: 1,
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
			want:           2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test case data defined in table)

			// Act
			got := CountSolutions(tt.regionMap, tt.gridSize, tt.markersPerUnit)

			// Assert
			if got != tt.want {
				t.Errorf("CountSolutions() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSolve(t *testing.T) {
	t.Run("unique solution returns correct placement", func(t *testing.T) {
		// Arrange
		regionMap := validRegionMap5x5
		gridSize := 5
		markersPerUnit := 1

		// Act
		solution, unique := Solve(regionMap, gridSize, markersPerUnit)

		// Assert
		if !unique {
			t.Fatal("expected unique solution, got non-unique")
		}
		if solution == nil {
			t.Fatal("expected non-nil solution")
		}

		// Expected markers at: (0,2), (1,0), (2,3), (3,1), (4,4).
		expected := [][2]int{{0, 2}, {1, 0}, {2, 3}, {3, 1}, {4, 4}}
		for _, e := range expected {
			if !solution[e[0]][e[1]] {
				t.Errorf("expected marker at (%d,%d), but not found", e[0], e[1])
			}
		}

		// Verify exactly 5 markers placed.
		count := 0
		markers := make([][2]int, 0, 5)
		for r := range solution {
			for c := range solution[r] {
				if solution[r][c] {
					count++
					markers = append(markers, [2]int{r, c})
				}
			}
		}
		if count != 5 {
			t.Errorf("expected 5 markers, got %d", count)
		}

		// Verify all constraints: one per row, column, region, no adjacency.
		rows := make(map[int]bool)
		cols := make(map[int]bool)
		regions := make(map[int]bool)
		for _, m := range markers {
			r, c := m[0], m[1]
			if rows[r] {
				t.Errorf("duplicate marker in row %d", r)
			}
			rows[r] = true
			if cols[c] {
				t.Errorf("duplicate marker in column %d", c)
			}
			cols[c] = true
			rid := validRegionMap5x5[r][c]
			if regions[rid] {
				t.Errorf("duplicate marker in region %d", rid)
			}
			regions[rid] = true
		}

		// Check no two markers are adjacent.
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

	t.Run("impossible returns not unique and nil solution", func(t *testing.T) {
		// Arrange
		regionMap := [][]int{
			{0, 1, 2},
			{0, 1, 2},
			{0, 1, 2},
		}

		// Act
		solution, unique := Solve(regionMap, 3, 1)

		// Assert
		if unique {
			t.Fatal("expected non-unique, got unique")
		}
		if solution != nil {
			t.Fatal("expected nil solution for impossible puzzle")
		}
	})

	t.Run("multiple solutions returns not unique and nil", func(t *testing.T) {
		// Arrange
		regionMap := [][]int{
			{0, 0, 1, 1},
			{0, 0, 1, 1},
			{2, 2, 3, 3},
			{2, 2, 3, 3},
		}

		// Act
		solution, unique := Solve(regionMap, 4, 1)

		// Assert
		if unique {
			t.Fatal("expected non-unique for multi-solution puzzle")
		}
		if solution != nil {
			t.Fatal("expected nil solution for multi-solution puzzle")
		}
	})
}

func TestSolveDoubleQueens(t *testing.T) {
	// validateDoubleQueensSolution checks all Double Queens constraints.
	validateSolution := func(t *testing.T, solution [][]bool, regionMap [][]int, gridSize, markersPerUnit int) {
		t.Helper()

		// Collect all marker positions.
		var markers [][2]int
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					markers = append(markers, [2]int{r, c})
				}
			}
		}

		// Total markers = gridSize * markersPerUnit.
		expectedTotal := gridSize * markersPerUnit
		if len(markers) != expectedTotal {
			t.Errorf("expected %d total markers, got %d", expectedTotal, len(markers))
		}

		// Each row has exactly markersPerUnit markers.
		for r := 0; r < gridSize; r++ {
			rowCount := 0
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					rowCount++
				}
			}
			if rowCount != markersPerUnit {
				t.Errorf("row %d has %d markers, want %d", r, rowCount, markersPerUnit)
			}
		}

		// Each column has exactly markersPerUnit markers.
		for c := 0; c < gridSize; c++ {
			colCount := 0
			for r := 0; r < gridSize; r++ {
				if solution[r][c] {
					colCount++
				}
			}
			if colCount != markersPerUnit {
				t.Errorf("column %d has %d markers, want %d", c, colCount, markersPerUnit)
			}
		}

		// Each region has exactly markersPerUnit markers.
		regionCounts := make(map[int]int)
		for _, m := range markers {
			rid := regionMap[m[0]][m[1]]
			regionCounts[rid]++
		}
		for rid := 0; rid < gridSize; rid++ {
			if regionCounts[rid] != markersPerUnit {
				t.Errorf("region %d has %d markers, want %d", rid, regionCounts[rid], markersPerUnit)
			}
		}

		// No two markers are adjacent.
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
	}

	// For Double Queens tests we use an 8x8 grid with row-stripe regions.
	// Row-stripe means region[r][c] = r, so each region is an entire row.
	// With markersPerUnit=2, the region constraint (2 markers per region) is
	// automatically satisfied by placing 2 markers per row. This isolates
	// the row/column/adjacency logic for testing.
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

	t.Run("solve finds valid Double Queens solutions on 8x8 row-stripe grid", func(t *testing.T) {
		// Arrange
		gridSize := 8
		markersPerUnit := 2

		// Act
		solutions := solve(doubleQueensRowStripe8x8, gridSize, markersPerUnit, 2)

		// Assert
		if len(solutions) == 0 {
			t.Fatal("expected at least one Double Queens solution on 8x8 row-stripe grid")
		}
		for _, sol := range solutions {
			validateSolution(t, sol, doubleQueensRowStripe8x8, gridSize, markersPerUnit)
		}
	})

	t.Run("CountSolutions returns 2 for multi-solution Double Queens grid", func(t *testing.T) {
		// Arrange
		gridSize := 8
		markersPerUnit := 2

		// Act
		count := CountSolutions(doubleQueensRowStripe8x8, gridSize, markersPerUnit)

		// Assert
		if count != 2 {
			t.Errorf("CountSolutions() = %d, want 2 (capped)", count)
		}
	})

	t.Run("Double Queens solutions have correct total marker count", func(t *testing.T) {
		// Arrange
		gridSize := 8
		markersPerUnit := 2
		expectedTotal := gridSize * markersPerUnit // 16

		// Act
		solutions := solve(doubleQueensRowStripe8x8, gridSize, markersPerUnit, 1)

		// Assert
		if len(solutions) == 0 {
			t.Fatal("expected at least one solution")
		}
		sol := solutions[0]
		count := 0
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if sol[r][c] {
					count++
				}
			}
		}
		if count != expectedTotal {
			t.Errorf("total markers = %d, want %d", count, expectedTotal)
		}
	})

	t.Run("Double Queens with region constraint filters solutions", func(t *testing.T) {
		// Arrange
		// Use a diagonal-stripe 8x8 region map. Each cell's region = (r+c) % 8.
		// This creates diagonal bands of 8 cells each. The region constraint
		// adds real filtering beyond row/column/adjacency.
		gridSize := 8
		markersPerUnit := 2
		regionMap := make([][]int, gridSize)
		for r := 0; r < gridSize; r++ {
			regionMap[r] = make([]int, gridSize)
			for c := 0; c < gridSize; c++ {
				regionMap[r][c] = (r + c) % gridSize
			}
		}

		// Act
		solutions := solve(regionMap, gridSize, markersPerUnit, 2)

		// Assert
		// Whether or not solutions exist, any that are found must be valid.
		for _, sol := range solutions {
			validateSolution(t, sol, regionMap, gridSize, markersPerUnit)
		}
		t.Logf("Found %d solutions (capped at 2) for diagonal-stripe 8x8 Double Queens", len(solutions))
	})
}

func TestAdjacencySafeMultiMarker(t *testing.T) {
	tests := []struct {
		name       string
		markerCols [][]int
		row        int
		col        int
		want       bool
	}{
		{
			name:       "no previous markers - safe",
			markerCols: [][]int{{}, {}},
			row:        0,
			col:        3,
			want:       true,
		},
		{
			name:       "adjacent diagonally - unsafe",
			markerCols: [][]int{{1, 4}, {}},
			row:        1,
			col:        2,
			want:       false,
		},
		{
			name:       "not adjacent to any - safe",
			markerCols: [][]int{{0, 5}, {}},
			row:        1,
			col:        3,
			want:       true,
		},
		{
			name:       "adjacent to second marker in row - unsafe",
			markerCols: [][]int{{0, 3}, {}},
			row:        1,
			col:        4,
			want:       false,
		},
		{
			name:       "same column in same row (should not happen but tests adjacency)",
			markerCols: [][]int{{2}, {}},
			row:        1,
			col:        2,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (defined in table)

			// Act
			got := adjacencySafe(tt.markerCols, tt.row, tt.col)

			// Assert
			if got != tt.want {
				t.Errorf("adjacencySafe() = %v, want %v", got, tt.want)
			}
		})
	}
}
