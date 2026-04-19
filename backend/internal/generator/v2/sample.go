package generator

import "math/bits"

// sampleSolution returns a valid N*k-mark placement satisfying row, column, and
// 8-neighbor adjacency constraints. Returns (marks, true) on success; (nil,
// false) only if the search space is exhausted (genuine unsatisfiability for
// the current shuffling).
//
// Approach: row-by-row backtracking in grid order using uint16 column
// bitmasks. Diversity of output comes from (a) a pre-shuffle of the
// column index permutation used to break ties during combo enumeration, and
// (b) per-row shuffling of the filtered k-combinations before recursion. This
// matches input-spec.md §4.1 generalized on k per locked decision #1.
//
// No heap allocation inside the backtracker: all scratch storage lives on
// the Generator and is reused across calls.
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

	combos := g.rowCombos[row][:0]
	combos = enumerateKCombos(combos, n, g.k, forbid, g.colCount)
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
// to reach its required k-count. rowsLeft is the number of rows not yet placed
// AFTER the current placement.
func (g *Generator) forwardCheck(rowsLeft int) bool {
	need := uint8(g.k)
	budget := uint8(rowsLeft)
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

// enumerateKCombos appends every k-column bitmask over [0, n) that (a) avoids
// the forbidden columns, (b) has pairwise column gap >= 2 (no intra-row
// adjacency), and (c) does not push any column over its k-budget.
//
// For k=1 it emits at most N single-bit masks; for k=2 it emits at most C(N,2)
// two-bit masks with gap >= 2. Pure function, no allocation (appends to a
// caller-owned slice).
func enumerateKCombos(dst []uint16, n, k int, forbid uint16, colCount [nMax]uint8) []uint16 {
	full := uint16(1)<<uint(n) - 1
	available := full &^ forbid

	if k == 1 {
		m := available
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if colCount[c] >= 1 {
				continue
			}
			dst = append(dst, uint16(1)<<uint(c))
		}
		return dst
	}

	// k == 2: enumerate ordered pairs (c1 < c2) with c2 - c1 >= 2.
	for c1 := 0; c1 < n-1; c1++ {
		bit1 := uint16(1) << uint(c1)
		if available&bit1 == 0 {
			continue
		}
		if colCount[c1] >= 2 {
			continue
		}
		for c2 := c1 + 2; c2 < n; c2++ {
			bit2 := uint16(1) << uint(c2)
			if available&bit2 == 0 {
				continue
			}
			if colCount[c2] >= 2 {
				continue
			}
			dst = append(dst, bit1|bit2)
		}
	}
	return dst
}
