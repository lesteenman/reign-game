package generator

import (
	"fmt"
	"testing"
)

// fmtSample returns a short "N=%d k=%d sample=%d" label used in test
// error messages across the three corpus consumers.
func fmtSample(n, k, sample int) string {
	return fmt.Sprintf("N=%d k=%d sample=%d", n, k, sample)
}

// assertDeductiveBruteAgree runs both the brute solver and the deductive
// solver on a Puzzle and asserts they agree: brute finds exactly one
// solution, that solution matches p.Solution, and the deductive solver
// reaches OutcomeSolved. Used by property, soak, and corpus-roundtrip
// tests — the three suites that load a generated Puzzle and need the
// full cross-check.
//
// Does NOT assert structural partition invariants (region sizes, 4-
// connectedness). Those live in assertRegionPartition which soak_test
// uses directly.
func assertDeductiveBruteAgree(t *testing.T, label string, p *Puzzle) {
	t.Helper()

	sols, err := bruteSolveAll(p.Regions, p.N, p.MarksPerUnit, 2)
	if err != nil {
		t.Fatalf("%s: brute: %v", label, err)
	}
	if len(sols) != 1 {
		t.Fatalf("%s: brute returned %d solutions, want 1", label, len(sols))
	}
	if !marksEqualUnordered(sols[0], p.Solution) {
		t.Fatalf("%s: deductive vs brute mismatch", label)
	}

	var s solverState
	if err := s.initFromRegionMap(p.Regions, p.N, p.MarksPerUnit); err != nil {
		t.Fatalf("%s: initFromRegionMap: %v", label, err)
	}
	if solve(&s) != OutcomeSolved {
		t.Fatalf("%s: deductive solver no longer solves", label)
	}
}

// assertRegionPartition asserts a region map is a valid [N][N]
// partition: every cell holds an id in [0, N), every id appears at
// least once, and every region has >= regionMinSize cells (R-067b).
// Shared by the soak test's per-puzzle assertion and any other loader
// that needs the same invariant.
func assertRegionPartition(t *testing.T, label string, regions [][]int, n int) {
	t.Helper()

	if len(regions) != n {
		t.Fatalf("%s: regions rows=%d, want %d", label, len(regions), n)
	}
	sizes := make([]int, n)
	for r := range regions {
		if len(regions[r]) != n {
			t.Fatalf("%s: row %d has %d cols, want %d",
				label, r, len(regions[r]), n)
		}
		for _, gid := range regions[r] {
			if gid < 0 || gid >= n {
				t.Fatalf("%s: region id %d out of [0, %d)",
					label, gid, n)
			}
			sizes[gid]++
		}
	}
	for gid, sz := range sizes {
		if sz < regionMinSize {
			t.Fatalf("%s: region %d has %d cells (< min %d)",
				label, gid, sz, regionMinSize)
		}
	}
}
