package generator

import (
	"errors"
	"fmt"
	"math/bits"
)

// ErrBruteInvalidInput signals that bruteSolveAll was called with inputs that
// violate the documented contract (n out of range, unsupported k, malformed
// region map, or a region id outside [0, n)). Caller bugs — we fail loudly
// here rather than silently returning garbage.
var ErrBruteInvalidInput = errors.New("bruteSolveAll: invalid input")

// bruteSolveAll enumerates up to maxSolutions valid mark placements on the
// given region map. Each returned solution is a slice of n*k Marks.
//
// The search is row-by-row backtracking using uint16 column bitmasks. Unlike
// the sampler in sample.go, the brute solver is deterministic (no shuffling):
// its job is completeness (count up to maxSolutions), not diversity.
//
// Pure function. No Generator state. No RNG. The caller may run it on any
// goroutine at any time.
//
// Constraints checked:
//   - Each row has exactly k marks.
//   - Each column has exactly k marks.
//   - Each region (regionMap[r][c] value) has exactly k marks.
//   - No two marks are 8-neighbor adjacent.
//   - k-marks inside a single row have pairwise column gap >= 2.
//
// Input contract: n in [1, nMax], k in {1, 2}, maxSolutions > 0, regionMap
// shaped exactly [n][n] with region ids in [0, n). Violations return
// ErrBruteInvalidInput.
//
// TODO(R-065): the per-call regOf initialization + per-recursion-level
// bruteKCombos allocation dominate when the region-grower scores many
// candidate maps. Pool state + share the combo enumerator with sample.go's
// enumerateKCombos when the deductive solver lands in R-064.
func bruteSolveAll(regionMap [][]int, n, k, maxSolutions int) ([][]Mark, error) {
	if n < 1 || n > nMax {
		return nil, fmt.Errorf("n=%d (must be in [1, %d]): %w", n, nMax, ErrBruteInvalidInput)
	}
	if k != 1 && k != 2 {
		return nil, fmt.Errorf("k=%d (must be 1 or 2): %w", k, ErrBruteInvalidInput)
	}
	if maxSolutions <= 0 {
		return nil, fmt.Errorf("maxSolutions=%d (must be > 0): %w", maxSolutions, ErrBruteInvalidInput)
	}
	if len(regionMap) != n {
		return nil, fmt.Errorf("regionMap has %d rows, want %d: %w", len(regionMap), n, ErrBruteInvalidInput)
	}
	for r, row := range regionMap {
		if len(row) != n {
			return nil, fmt.Errorf("regionMap[%d] has %d cols, want %d: %w", r, len(row), n, ErrBruteInvalidInput)
		}
		for c, rid := range row {
			if rid < 0 || rid >= n {
				return nil, fmt.Errorf("regionMap[%d][%d]=%d out of [0, %d): %w", r, c, rid, n, ErrBruteInvalidInput)
			}
		}
	}

	var (
		rowMarks [nMax]uint16
		colCount [nMax]uint8
		regCount [nMax]uint8
		regOf    [nMax][nMax]uint8
	)
	for r := range n {
		for c := range n {
			regOf[r][c] = uint8(regionMap[r][c])
		}
	}

	solutions := make([][]Mark, 0, maxSolutions)

	var recurse func(row int)
	recurse = func(row int) {
		if len(solutions) >= maxSolutions {
			return
		}
		if row == n {
			for c := range n {
				if colCount[c] != uint8(k) {
					return
				}
			}
			for r := range n {
				if regCount[r] != uint8(k) {
					return
				}
			}
			sol := collectMarks(rowMarks[:n])
			solutions = append(solutions, sol)
			return
		}

		rowsAfter := n - row - 1
		full := uint16(1)<<uint(n) - 1

		// Adjacency from the previous row (already fully placed).
		var forbid uint16
		if row > 0 {
			forbid = adjacentColumnsMask(rowMarks[row-1], n)
		}
		available := full &^ forbid

		for _, mask := range bruteKCombos(available, n, k) {
			// Column-budget check.
			ok := true
			tmp := mask
			for tmp != 0 {
				c := bits.TrailingZeros16(tmp)
				tmp &^= 1 << c
				if colCount[c] >= uint8(k) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			// Region-budget check.
			tmp = mask
			ok = true
			for tmp != 0 {
				c := bits.TrailingZeros16(tmp)
				tmp &^= 1 << c
				rid := regOf[row][c]
				if regCount[rid] >= uint8(k) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}

			// Apply.
			rowMarks[row] = mask
			tmp = mask
			for tmp != 0 {
				c := bits.TrailingZeros16(tmp)
				tmp &^= 1 << c
				colCount[c]++
				regCount[regOf[row][c]]++
			}

			if forwardCheckBrute(colCount[:], regCount[:], n, k, rowsAfter) {
				recurse(row + 1)
			}

			// Undo.
			tmp = mask
			for tmp != 0 {
				c := bits.TrailingZeros16(tmp)
				tmp &^= 1 << c
				colCount[c]--
				regCount[regOf[row][c]]--
			}
			rowMarks[row] = 0

			if len(solutions) >= maxSolutions {
				return
			}
		}
	}
	recurse(0)
	return solutions, nil
}

// bruteKCombos returns an iterator sequence of k-column bitmasks over
// [0, n) drawn from the `available` mask with pairwise column gap >= 2.
// Implemented as a slice to keep code simple; the slice is small (<= 120) and
// the brute solver is not on the release hot path.
func bruteKCombos(available uint16, n, k int) []uint16 {
	if k == 1 {
		out := make([]uint16, 0, bits.OnesCount16(available))
		m := available
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			out = append(out, uint16(1)<<uint(c))
		}
		return out
	}
	// k == 2.
	out := make([]uint16, 0, 128)
	for c1 := 0; c1 < n-1; c1++ {
		b1 := uint16(1) << uint(c1)
		if available&b1 == 0 {
			continue
		}
		for c2 := c1 + 2; c2 < n; c2++ {
			b2 := uint16(1) << uint(c2)
			if available&b2 == 0 {
				continue
			}
			out = append(out, b1|b2)
		}
	}
	return out
}

// forwardCheckBrute returns true iff every column and region still has
// enough remaining rows to reach its required k-count. rowsAfter is the
// number of rows not yet placed AFTER the current placement — matches
// (g *Generator).forwardCheck's argument convention exactly.
func forwardCheckBrute(colCount, regCount []uint8, n, k, rowsAfter int) bool {
	need := uint8(k)
	budget := uint8(rowsAfter)
	for c := 0; c < n; c++ {
		if colCount[c] > need {
			return false
		}
		if need-colCount[c] > budget {
			return false
		}
	}
	for r := 0; r < n; r++ {
		if regCount[r] > need {
			return false
		}
	}
	return true
}

// collectMarks converts a per-row bitmask slice into a flat []Mark.
func collectMarks(rowMasks []uint16) []Mark {
	total := 0
	for _, m := range rowMasks {
		total += bits.OnesCount16(m)
	}
	out := make([]Mark, 0, total)
	for r, m := range rowMasks {
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			out = append(out, Mark{Row: r, Col: c})
		}
	}
	return out
}
