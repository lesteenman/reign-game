package generator

import (
	"fmt"
	"math/bits"
)

// appendKCombos appends every k-column bitmask over [0, n) drawn from the
// `available` mask with pairwise column gap >= 2 (no intra-row adjacency) and
// subject to a per-column budget: a column c is skipped if
// perColCount[c] >= perColCap.
//
// Callers that do not want a per-column budget pass perColCap = uint8(k) and
// a zeroed (or nil-equivalent) counts array — a budget of k is never
// saturated by a single k-combination, so it is a no-op.
//
// Pure function. The returned slice is the caller-owned slice with the new
// masks appended. If appending would exceed cap(dst), panic — undersized
// scratch buffers surface as loud bugs instead of silent heap reallocation
// that would defeat the zero-alloc hot loops.
//
// This is the single canonical k-combo enumerator for the package. Historical
// separate implementations in sample.go (enumerateKCombos) and brute.go
// (bruteKCombos) converge here as of R-064 — the three callers are:
//
//   - sampler (sample.go, g.sampleBacktrack): enumerate per-row candidates
//     against a forbid mask derived from the previous grid row; per-column
//     budget = k (saturating columns are excluded).
//   - brute solver (brute.go, recurse): per-row candidates without a column
//     filter, then a separate column/region budget check in the backtracker.
//   - deductive solver (rules.go, R3 forced placement): not currently a
//     caller, but the shared shape leaves the door open if a rule needs to
//     enumerate k-subsets of a row's candidate mask.
func appendKCombos(dst []uint16, n, k int, available uint16, perColCap uint8, perColCount *[nMax]uint8) []uint16 {
	// Hot loop: the capacity guard is inlined at each append to keep
	// appendKCombos from closing over `dst` (a closure forces the slice
	// header onto the heap and costs ~2-3% across the sampler benchmark).
	switch k {
	case 1:
		m := available
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if perColCount != nil && perColCount[c] >= perColCap {
				continue
			}
			if len(dst) == cap(dst) {
				panic(fmt.Sprintf("appendKCombos: dst capacity %d exceeded (n=%d, k=%d)", cap(dst), n, k))
			}
			dst = append(dst, uint16(1)<<uint(c))
		}
		return dst
	case 2:
		for c1 := 0; c1 < n-1; c1++ {
			bit1 := uint16(1) << uint(c1)
			if available&bit1 == 0 {
				continue
			}
			if perColCount != nil && perColCount[c1] >= perColCap {
				continue
			}
			for c2 := c1 + 2; c2 < n; c2++ {
				bit2 := uint16(1) << uint(c2)
				if available&bit2 == 0 {
					continue
				}
				if perColCount != nil && perColCount[c2] >= perColCap {
					continue
				}
				if len(dst) == cap(dst) {
					panic(fmt.Sprintf("appendKCombos: dst capacity %d exceeded (n=%d, k=%d)", cap(dst), n, k))
				}
				dst = append(dst, bit1|bit2)
			}
		}
		return dst
	default:
		panic(fmt.Sprintf("appendKCombos: unsupported k=%d (only k in {1,2})", k))
	}
}
