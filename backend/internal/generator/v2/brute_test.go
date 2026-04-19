package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureFile mirrors the JSON shape under testdata/fixtures/.
type fixtureFile struct {
	Name                           string  `json:"name"`
	N                              int     `json:"n"`
	K                              int     `json:"k"`
	ExpectedSolutionCount          *int    `json:"expected_solution_count"`
	ExpectedSolutionCountCappedAt2 *int    `json:"expected_solution_count_capped_at_2"`
	RegionMap                      [][]int `json:"region_map"`
}

func loadFixture(t *testing.T, name string) fixtureFile {
	t.Helper()

	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parsing fixture %s: %v", path, err)
	}
	return f
}

func TestBruteSolve(t *testing.T) {
	t.Parallel()

	unique := loadFixture(t, "unique_5x5_k1.json")
	ambiguous := loadFixture(t, "ambiguous_4x4_k1.json")
	unsatisfiable := loadFixture(t, "unsatisfiable_3x3_k1.json")

	cases := []struct {
		name         string
		regionMap    [][]int
		n            int
		k            int
		maxSolutions int
		wantCount    int
	}{
		{
			name:         "known-unique 5x5 k=1 (fixture)",
			regionMap:    unique.RegionMap,
			n:            unique.N,
			k:            unique.K,
			maxSolutions: 2,
			wantCount:    *unique.ExpectedSolutionCount,
		},
		{
			name:         "ambiguous 4x4 k=1 (fixture) returns 2 when capped",
			regionMap:    ambiguous.RegionMap,
			n:            ambiguous.N,
			k:            ambiguous.K,
			maxSolutions: 2,
			wantCount:    *ambiguous.ExpectedSolutionCountCappedAt2,
		},
		{
			name:         "unsatisfiable 3x3 k=1 (fixture) returns 0",
			regionMap:    unsatisfiable.RegionMap,
			n:            unsatisfiable.N,
			k:            unsatisfiable.K,
			maxSolutions: 2,
			wantCount:    *unsatisfiable.ExpectedSolutionCount,
		},
		{
			name:         "ambiguous with maxSolutions=1 stops early",
			regionMap:    ambiguous.RegionMap,
			n:            ambiguous.N,
			k:            ambiguous.K,
			maxSolutions: 1,
			wantCount:    1,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange / Act
			got := bruteSolveAll(tc.regionMap, tc.n, tc.k, tc.maxSolutions)

			// Assert
			if len(got) != tc.wantCount {
				t.Fatalf("bruteSolveAll: got %d solutions, want %d", len(got), tc.wantCount)
			}
			for _, sol := range got {
				assertValidSolution(t, sol, tc.n, tc.k)
				assertRegionCounts(t, sol, tc.regionMap, tc.n, tc.k)
			}
		})
	}
}

// TestBruteSolve_K2_UniqueOn5x5 verifies k=2 brute solving: a 5x5 region map
// where the row-stripes force marks into specific rows; because k=2 is
// infeasible at N=5 under the full constraint set, the expected count is 0.
// This exercises the k=2 code path and the region constraint together.
func TestBruteSolve_K2_Infeasible5x5(t *testing.T) {
	t.Parallel()

	// Arrange
	rowStripes := [][]int{
		{0, 0, 0, 0, 0},
		{1, 1, 1, 1, 1},
		{2, 2, 2, 2, 2},
		{3, 3, 3, 3, 3},
		{4, 4, 4, 4, 4},
	}

	// Act
	got := bruteSolveAll(rowStripes, 5, 2, 2)

	// Assert
	if len(got) != 0 {
		t.Fatalf("expected 0 solutions for infeasible 5x5 k=2, got %d", len(got))
	}
}

// TestBruteSolve_K2_RowStripes8x8 verifies k=2 on a feasible grid.
func TestBruteSolve_K2_FeasibleOnRowStripes8x8(t *testing.T) {
	t.Parallel()

	// Arrange — row-stripe regions trivially satisfy region constraint when
	// k marks per row already satisfies it. Solver should find solutions.
	rowStripes := [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 1, 1, 1, 1, 1, 1},
		{2, 2, 2, 2, 2, 2, 2, 2},
		{3, 3, 3, 3, 3, 3, 3, 3},
		{4, 4, 4, 4, 4, 4, 4, 4},
		{5, 5, 5, 5, 5, 5, 5, 5},
		{6, 6, 6, 6, 6, 6, 6, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
	}

	// Act
	got := bruteSolveAll(rowStripes, 8, 2, 2)

	// Assert
	if len(got) == 0 {
		t.Fatal("expected at least 1 solution for feasible 8x8 k=2 row-stripes")
	}
	for _, sol := range got {
		assertValidSolution(t, sol, 8, 2)
		assertRegionCounts(t, sol, rowStripes, 8, 2)
	}
}

// TestBruteSolve_WorstCaseBudget asserts the brute solver completes under
// 100ms on a worst-case N=14 fixture.
//
// "Worst case" for the brute solver (in its real role — uniqueness check
// after Solved) is a region map where many near-solutions exist but the
// cap=2 short-circuit still has to walk a large sub-tree before finding
// the second solution (or proving no second exists).
//
// The fixtures below cover three representative shapes: row-stripes (many
// solutions, cap fires early), column-stripes (regions coincide with
// columns, tightly constrained), and row-stripes at k=2 (the largest
// fan-out observed in pre-implementation smoke).
func TestBruteSolve_WorstCaseBudget(t *testing.T) {
	t.Parallel()

	const n = 14

	makeRowStripes := func() [][]int {
		m := make([][]int, n)
		for r := range m {
			m[r] = make([]int, n)
			for c := range m[r] {
				m[r][c] = r
			}
		}
		return m
	}
	makeColStripes := func() [][]int {
		m := make([][]int, n)
		for r := range m {
			m[r] = make([]int, n)
			for c := range m[r] {
				m[r][c] = c
			}
		}
		return m
	}

	cases := []struct {
		name      string
		regionMap [][]int
		k         int
	}{
		{name: "row-stripes k=1", regionMap: makeRowStripes(), k: 1},
		{name: "col-stripes k=1", regionMap: makeColStripes(), k: 1},
		{name: "row-stripes k=2", regionMap: makeRowStripes(), k: 2},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			start := time.Now()
			got := bruteSolveAll(tc.regionMap, n, tc.k, 2)
			elapsed := time.Since(start)

			// Assert
			if elapsed > 100*time.Millisecond {
				t.Fatalf("brute solve (N=%d %s) took %v, want <100ms", n, tc.name, elapsed)
			}
			t.Logf("brute solve N=%d k=%d (%s, cap=2): %v, %d solutions",
				n, tc.k, tc.name, elapsed, len(got))
		})
	}
}

// assertRegionCounts verifies that each region has exactly k marks.
func assertRegionCounts(t *testing.T, marks []Mark, regionMap [][]int, n, k int) {
	t.Helper()

	count := make(map[int]int)
	for _, m := range marks {
		count[regionMap[m.Row][m.Col]]++
	}
	for rid, c := range count {
		if c != k {
			t.Errorf("region %d has %d marks, expected %d", rid, c, k)
		}
	}
	// Also verify every region is represented.
	seen := make(map[int]bool)
	for r := range n {
		for c := range n {
			seen[regionMap[r][c]] = true
		}
	}
	for rid := range seen {
		if count[rid] != k {
			t.Errorf("region %d has %d marks (expected %d)", rid, count[rid], k)
		}
	}
}
