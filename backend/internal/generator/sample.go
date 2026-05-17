package generator

import (
	"math/bits"
)

// sampleSolution returns a valid N*k-mark placement satisfying row, column,
// region (implicit — regions are generated later in R-065), and 8-neighbor
// adjacency constraints. Returns (marks, true) on success; (nil, false) only
// if the search space is exhausted (genuine unsatisfiability for this Generator's
// (n, k) — e.g. k=2 with n<8 is known-infeasible per bench/n-feasibility.md).
//
// Approach: row-by-row backtracking in grid order using uint16 column
// bitmasks. Diversity comes from per-row shuffling of the filtered
// k-combinations before recursion; the RNG state (from WithSeed or a
// time-seeded default) makes distinct Generate calls produce distinct
// solutions.
//
// Spec deviation from input-spec.md §4.1 (which calls for randomized row
// *visit* order): visiting rows out of grid order breaks the prev-row
// adjacency pruning in adjacentColumnsMask, because "previous row" then
// refers to a row that may not be grid-adjacent. At N=13 k=2 this caused
// multi-minute hangs during sampler smoke. Grid-order visiting plus combo
// shuffling preserves the spec's diversity goal without the pruning
// regression; see design.md §4 "Implementation note — grid-order visiting"
// for the full rationale.
//
// No heap allocation inside the backtracker: all scratch storage lives on
// the Generator and is reused across calls. The returned slice is the one
// allocation per Generate call (unavoidable per design §5).
func (g *Generator) sampleSolution() ([]Mark, bool) {
	n := g.n

	for i := range n {
		g.rowMarks[i] = 0
		g.colCount[i] = 0
	}

	if !g.sampleBacktrack(0) {
		return nil, false
	}

	g.solBuf = g.solBuf[:0]
	for r := range n {
		mask := g.rowMarks[r]
		for mask != 0 {
			c := bits.TrailingZeros16(mask)
			mask &^= 1 << c
			g.solBuf = append(g.solBuf, Mark{Row: r, Col: c})
		}
	}
	out := make([]Mark, len(g.solBuf))
	copy(out, g.solBuf)
	return out, true
}

// sampleBacktrack fills rows 0..n-1 in grid order. At each depth (== row) it
// enumerates k-column bitmasks, filters against column-count caps, vertical
// and diagonal adjacency with the previous row, and forward-checking.
// Returns true on a complete solution.
func (g *Generator) sampleBacktrack(row int) bool {
	n := g.n
	if row == n {
		return true
	}

	rowsRemaining := n - row

	var forbid uint16
	if row > 0 {
		forbid = adjacentColumnsMask(g.rowMarks[row-1], n)
	}

	full := uint16(1)<<uint(n) - 1
	available := full &^ forbid

	combos := g.rowCombos[row][:0]
	combos = appendKCombos(combos, n, g.k, available, uint8(g.k), &g.colCount)
	if len(combos) == 0 {
		return false
	}

	g.rng.Shuffle(len(combos), func(i, j int) {
		combos[i], combos[j] = combos[j], combos[i]
	})

	for _, mask := range combos {
		g.rowMarks[row] = mask
		m := mask
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			g.colCount[c]++
		}

		if g.forwardCheck(rowsRemaining - 1) {
			if g.sampleBacktrack(row + 1) {
				return true
			}
		}

		m = mask
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			g.colCount[c]--
		}
		g.rowMarks[row] = 0
	}
	return false
}

// forwardCheck returns true iff every column still has enough remaining rows
// to reach its required k-count. rowsAfter is the number of rows not yet
// placed AFTER the current placement — matches forwardCheckBrute's argument
// convention exactly.
func (g *Generator) forwardCheck(rowsAfter int) bool {
	need := uint8(g.k)
	budget := uint8(rowsAfter)
	for c := 0; c < g.n; c++ {
		if g.colCount[c] > need {
			return false
		}
		if need-g.colCount[c] > budget {
			return false
		}
	}
	return true
}

// adjacentColumnsMask expands a row's mark mask into the columns that would
// collide diagonally/vertically with cells in an adjacent grid row: each set
// bit at column c forbids columns c-1, c, c+1 in the neighbor.
func adjacentColumnsMask(rowMask uint16, n int) uint16 {
	full := uint16(1)<<uint(n) - 1
	spread := rowMask | (rowMask << 1) | (rowMask >> 1)
	return spread & full
}
