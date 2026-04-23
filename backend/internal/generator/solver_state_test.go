package generator

import "testing"

// TestContradicts_K2_VerticallyStripedRegion is the regression test for the
// R-066 `contradicts()` bug.
//
// Pre-R-066, contradicts() used popcount of the per-region column-union
// mask as the "cells available" count. For a region whose candidates span
// multiple rows in the SAME column (common at k=2 with vertical stripe
// region maps), that undercount made contradicts() flag a fresh state as
// contradictory — immediately killing every k=2 grow attempt (N=9 k=2
// success rate was 0% end-to-end because the solver bailed at step 0).
//
// The fix sums per-row cell counts across the region, which is the true
// cell capacity. This test pins that behavior: on a vertically-striped
// 4x4 k=2 region map, a fresh state MUST NOT contradict.
func TestContradicts_K2_VerticallyStripedRegion(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 k=2 region map with 4 vertical stripes (one region per
	// column, 4 cells each). regNeed starts at 2 for every region, with 4
	// candidate cells per region → clearly not contradictory.
	regionMap := [][]int{
		{0, 1, 2, 3},
		{0, 1, 2, 3},
		{0, 1, 2, 3},
		{0, 1, 2, 3},
	}
	s := newSolverState(regionMap, 4, 2)

	// Act
	got := s.contradicts()

	// Assert
	if got {
		t.Error("contradicts(): returned true on a fresh 4x4 k=2 state with vertical-stripe regions. " +
			"This is the pre-R-066 regression — old code undercounted region cells by popcounting " +
			"the column-union instead of summing per-row cells.")
	}
}
