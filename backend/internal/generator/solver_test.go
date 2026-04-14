package generator

import (
	"testing"
)

func TestCountSolutions(t *testing.T) {
	tests := []struct {
		name      string
		regionMap [][]int
		gridSize  int
		want      int
	}{
		{
			name:      "valid 5x5 with exactly 1 solution",
			regionMap: validRegionMap5x5,
			gridSize:  5,
			want:      1,
		},
		{
			name: "impossible - column regions force adjacent markers",
			regionMap: [][]int{
				{0, 1, 2},
				{0, 1, 2},
				{0, 1, 2},
			},
			gridSize: 3,
			want:     0,
		},
		{
			name: "multiple solutions - 4x4 grid with 2x2 blocks",
			regionMap: [][]int{
				{0, 0, 1, 1},
				{0, 0, 1, 1},
				{2, 2, 3, 3},
				{2, 2, 3, 3},
			},
			gridSize: 4,
			want:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountSolutions(tt.regionMap, tt.gridSize)
			if got != tt.want {
				t.Errorf("CountSolutions() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSolve(t *testing.T) {
	t.Run("unique solution returns correct placement", func(t *testing.T) {
		solution, unique := Solve(validRegionMap5x5, 5)
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
		regionMap := [][]int{
			{0, 1, 2},
			{0, 1, 2},
			{0, 1, 2},
		}
		solution, unique := Solve(regionMap, 3)
		if unique {
			t.Fatal("expected non-unique, got unique")
		}
		if solution != nil {
			t.Fatal("expected nil solution for impossible puzzle")
		}
	})

	t.Run("multiple solutions returns not unique and nil", func(t *testing.T) {
		regionMap := [][]int{
			{0, 0, 1, 1},
			{0, 0, 1, 1},
			{2, 2, 3, 3},
			{2, 2, 3, 3},
		}
		solution, unique := Solve(regionMap, 4)
		if unique {
			t.Fatal("expected non-unique for multi-solution puzzle")
		}
		if solution != nil {
			t.Fatal("expected nil solution for multi-solution puzzle")
		}
	})
}
