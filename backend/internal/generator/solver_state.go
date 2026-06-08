package generator

import (
	"errors"
	"fmt"
	"math/bits"
)

// ErrSolverInvalidInput signals that initFromRegionMap was called with inputs
// that violate the documented contract (n out of range, unsupported k,
// malformed region map, or a region id outside [0, n)).
var ErrSolverInvalidInput = errors.New("solverState: invalid input")

// ruleID identifies one of the live deductive rules. The numeric values are
// stable and match the spec names (R1..R5, R7). Index into ruleTier below to
// recover the tier for trace aggregation / classification.
type ruleID uint8

const (
	ruleR1 ruleID = 1 // Adjacency elimination (Tier 1)
	ruleR2 ruleID = 2 // Count saturation      (Tier 1)
	ruleR3 ruleID = 3 // Forced placement      (Tier 1)
	ruleR4 ruleID = 4 // Single-line region    (Tier 2)
	ruleR5 ruleID = 5 // Single-region line    (Tier 2)
	ruleR7 ruleID = 7 // Adjacency forcing     (Tier 3)
)

// ruleTier maps a ruleID to its classifier tier (1..4). Index 0 and the
// indices of retired rules are unused (zero). No live rule maps to tier 4,
// so MaxTier caps at 3. Consumed by the R-065 classifier for TierCounts /
// MaxTier computation; declared here so ruleID and its tier-grouping live
// together.
var ruleTier = [10]int{ //nolint:unused // consumed by R-065 classifier
	0, // unused
	1, // R1
	1, // R2
	1, // R3
	2, // R4
	2, // R5
	0, // unused
	3, // R7
	0, // unused
	0, // unused
}

// ruleEvent is a single rule-application record. Minimally a rule id plus the
// row affected — subsequent rules aggregate via the trace's count per tier.
// Kept narrow (two bytes + one int) so the trace buffer stays cache-friendly.
type ruleEvent struct {
	Rule ruleID
	// Row is the primary row/line the rule acted on (0..n-1). For rules that
	// act on a column or region, Row is the column/region id; the Rule field
	// disambiguates. Used only by classifier + tests.
	Row uint8
}

// ruleTrace is the ordered list of rule applications during one solve.
// Backed by a slice owned by the Generator (reused across Generate calls via
// [:0]-reset) so the solve loop allocates zero after warm-up.
type ruleTrace []ruleEvent

// solverState is the working state of the deductive solver for one partial
// puzzle. All hot-path fields are fixed-size [nMax] arrays and uint16
// bitmasks — a *dst = *src value copy is a cheap O(nMax²) operation that
// preserves every invariant, which is exactly what the region-grower scoring
// loop (R-066) needs.
//
// The trace slice header is a three-word reference; value-copying a
// solverState aliases the backing array with the source. That is safe for
// the region-grower pattern (clone, read-only solve, discard) and harmful
// for any parallel mutation pattern. Document the contract; do not enforce
// it.
type solverState struct {
	n, k int

	cands         [nMax]uint16       // candidate column mask per row
	marks         [nMax]uint16       // confirmed mark mask per row
	rowNeed       [nMax]uint8        // marks still needed per row (init k)
	colNeed       [nMax]uint8        // marks still needed per column
	regNeed       [nMax]uint8        // marks still needed per region
	regCellsByRow [nMax][nMax]uint16 // [regionID][row] -> column mask of remaining-candidate cells in that region
	regOf         [nMax][nMax]uint8  // (row, col) -> region id

	// trace is an optional recording of rule firings. nil means "do not
	// record". When recording, appending never reallocates because the
	// generator pre-allocates enough capacity up front.
	trace ruleTrace
}

// initFromRegionMap populates every field from (regionMap, n, k). All
// invariants hold after return:
//
//   - cands[r] has exactly n bits set (mask 0..n-1) — every cell is a candidate.
//   - marks[r] == 0 for all rows.
//   - rowNeed[r] == colNeed[c] == regNeed[g] == k.
//   - regCellsByRow[g][r] is the candidate-mask of cells in region g at row r.
//   - regOf[r][c] == regionMap[r][c].
//
// Validation: n in [1, nMax], k in {1, 2}, regionMap shaped exactly [n][n],
// region ids in [0, n). Violations return ErrSolverInvalidInput.
//
// Trace recording is left alone: callers flip s.trace explicitly before
// calling solve (nil disables recording; a non-nil slice enables it).
func (s *solverState) initFromRegionMap(regionMap [][]int, n, k int) error {
	if n < 1 || n > nMax {
		return fmt.Errorf("n=%d (must be in [1, %d]): %w", n, nMax, ErrSolverInvalidInput)
	}
	if k != 1 && k != 2 {
		return fmt.Errorf("k=%d (must be 1 or 2): %w", k, ErrSolverInvalidInput)
	}
	if len(regionMap) != n {
		return fmt.Errorf("regionMap has %d rows, want %d: %w", len(regionMap), n, ErrSolverInvalidInput)
	}

	s.n = n
	s.k = k

	full := uint16(1)<<uint(n) - 1

	for r := range nMax {
		s.cands[r] = 0
		s.marks[r] = 0
		s.rowNeed[r] = 0
		s.colNeed[r] = 0
		s.regNeed[r] = 0
		for g := range nMax {
			s.regCellsByRow[g][r] = 0
			s.regOf[r][g] = 0
		}
	}

	for r := range n {
		row := regionMap[r]
		if len(row) != n {
			return fmt.Errorf("regionMap[%d] has %d cols, want %d: %w", r, len(row), n, ErrSolverInvalidInput)
		}
		s.cands[r] = full
		s.rowNeed[r] = uint8(k)
		for c, rid := range row {
			if rid < 0 || rid >= n {
				return fmt.Errorf("regionMap[%d][%d]=%d out of [0, %d): %w", r, c, rid, n, ErrSolverInvalidInput)
			}
			s.regOf[r][c] = uint8(rid)
			s.regCellsByRow[rid][r] |= 1 << uint(c)
		}
	}

	for c := range n {
		s.colNeed[c] = uint8(k)
	}
	for g := range n {
		s.regNeed[g] = uint8(k)
	}

	if s.trace != nil {
		s.trace = s.trace[:0]
	}

	return nil
}

// initFromRegionOf is the hot-path sibling of initFromRegionMap: it
// populates the solver state directly from an internal [nMax][nMax]int8
// region array, skipping the [][]int slice allocation that
// convertRegionsToSlices would otherwise produce.
//
// Semantics match initFromRegionMap exactly — callers may use either.
// initFromRegionMap is the audited public API surface (accepts the shape
// the brute solver and tests use); initFromRegionOf is reserved for
// generator-internal scoring/swap probes where per-probe allocations
// would dominate.
//
// Validation: n in [1, nMax], k in {1, 2}, region ids in [0, n). Cells
// outside [0, n)x[0, n) are not inspected.
func (s *solverState) initFromRegionOf(src *[nMax][nMax]int8, n, k int) error {
	if n < 1 || n > nMax {
		return fmt.Errorf("n=%d (must be in [1, %d]): %w", n, nMax, ErrSolverInvalidInput)
	}
	if k != 1 && k != 2 {
		return fmt.Errorf("k=%d (must be 1 or 2): %w", k, ErrSolverInvalidInput)
	}

	s.n = n
	s.k = k

	full := uint16(1)<<uint(n) - 1

	for r := range nMax {
		s.cands[r] = 0
		s.marks[r] = 0
		s.rowNeed[r] = 0
		s.colNeed[r] = 0
		s.regNeed[r] = 0
		for g := range nMax {
			s.regCellsByRow[g][r] = 0
			s.regOf[r][g] = 0
		}
	}

	for r := range n {
		s.cands[r] = full
		s.rowNeed[r] = uint8(k)
		for c := range n {
			rid := int(src[r][c])
			if rid < 0 || rid >= n {
				return fmt.Errorf("src[%d][%d]=%d out of [0, %d): %w", r, c, rid, n, ErrSolverInvalidInput)
			}
			s.regOf[r][c] = uint8(rid)
			s.regCellsByRow[rid][r] |= 1 << uint(c)
		}
	}

	for c := range n {
		s.colNeed[c] = uint8(k)
	}
	for g := range n {
		s.regNeed[g] = uint8(k)
	}

	if s.trace != nil {
		s.trace = s.trace[:0]
	}

	return nil
}

// colCandidateMask returns the mask of rows in column c that currently hold
// that column as a candidate. Callers use this for column-axis rule logic
// symmetric to the row-axis logic (which already has cands[r] directly).
//
// O(n) via the popcount-friendly bit-extracted form.
func (s *solverState) colCandidateMask(c int) uint16 {
	var out uint16
	bitC := uint16(1) << uint(c)
	for r := range s.n {
		if s.cands[r]&bitC != 0 {
			out |= 1 << uint(r)
		}
	}
	return out
}

// regionCandidateCellCount returns the total number of remaining
// candidate cells across all rows in region g. Sums per-row popcounts
// rather than popcounting the ORed column union — critical at k>=2
// where a region can legitimately have two row-distinct cells in the
// same column (so the union would double-undercount them as one).
func (s *solverState) regionCandidateCellCount(g int) int {
	count := 0
	for r := range s.n {
		count += bits.OnesCount16(s.regCellsByRow[g][r])
	}
	return count
}

// placeMark transitions cell (r, c) from "candidate" to "confirmed mark":
// clears the bit from cands and region-cell masks, sets the bit in marks,
// and decrements row/col/region need. Does NOT eliminate neighbors —
// callers (specifically R1) do that.
//
// Safe no-op if the bit is not a candidate (defensive): returns false in
// that case. Returns true if a new mark was placed.
func (s *solverState) placeMark(r, c int) bool {
	bit := uint16(1) << uint(c)
	if s.marks[r]&bit != 0 {
		return false
	}
	if s.cands[r]&bit == 0 {
		return false
	}
	s.marks[r] |= bit
	s.cands[r] &^= bit
	s.rowNeed[r]--
	s.colNeed[c]--
	rid := s.regOf[r][c]
	s.regNeed[rid]--
	s.regCellsByRow[rid][r] &^= bit
	return true
}

// eliminateCand clears the candidate bit at (r, c) if present. No-op if the
// bit is not a candidate. Returns true if something changed.
func (s *solverState) eliminateCand(r, c int) bool {
	bit := uint16(1) << uint(c)
	if s.cands[r]&bit == 0 {
		return false
	}
	s.cands[r] &^= bit
	rid := s.regOf[r][c]
	s.regCellsByRow[rid][r] &^= bit
	return true
}

// eliminateCandMask clears every candidate bit in `mask` from row r. Returns
// true if anything changed. Faster than calling eliminateCand per-bit when
// the caller already has a mask in hand.
func (s *solverState) eliminateCandMask(r int, mask uint16) bool {
	hit := s.cands[r] & mask
	if hit == 0 {
		return false
	}
	s.cands[r] &^= hit
	// Update per-region masks — walk the actually-eliminated bits (hit).
	m := hit
	for m != 0 {
		c := bits.TrailingZeros16(m)
		m &^= 1 << c
		rid := s.regOf[r][c]
		s.regCellsByRow[rid][r] &^= 1 << uint(c)
	}
	return true
}

// record appends a ruleEvent to the trace if recording is enabled. No-op
// otherwise (cheap nil check; branch predictor friendly).
func (s *solverState) record(ev ruleEvent) {
	if s.trace != nil {
		s.trace = append(s.trace, ev)
	}
}

// Trace recording is toggled by callers setting s.trace directly:
// nil disables recording; a non-nil (possibly zero-length) slice enables
// it. The R-065 region-grower scoring loop sets it to nil; the R-065
// classifier sets it to a pre-allocated Generator-owned buffer. The
// rules' record() helper checks the nil flag per call.

// contradicts returns true iff any row/col/region has more outstanding need
// than its remaining candidates can satisfy (or already has more marks than
// k, which should never happen under correct rules but is cheap to check).
func (s *solverState) contradicts() bool {
	for r := range s.n {
		if int(s.rowNeed[r]) > bits.OnesCount16(s.cands[r]) {
			return true
		}
	}
	for c := range s.n {
		need := int(s.colNeed[c])
		have := bits.OnesCount16(s.colCandidateMask(c))
		if need > have {
			return true
		}
	}
	for g := range s.n {
		need := int(s.regNeed[g])
		have := s.regionCandidateCellCount(g)
		if need > have {
			return true
		}
	}
	return false
}

// solved returns true iff every row/col/region has zero need AND the placed
// marks satisfy the 8-neighbor adjacency constraint. The adjacency check is
// the fail-safe: if R1 is pruned from the ruleset (necessity-test mode), a
// downstream rule could place adjacent marks; solved() must not lie about
// that and report Solved on an invalid state.
func (s *solverState) solved() bool {
	for r := range s.n {
		if s.rowNeed[r] != 0 {
			return false
		}
	}
	for c := range s.n {
		if s.colNeed[c] != 0 {
			return false
		}
	}
	for g := range s.n {
		if s.regNeed[g] != 0 {
			return false
		}
	}
	// Adjacency sanity: no two placed marks may be 8-neighbor adjacent.
	for r := range s.n {
		marks := s.marks[r]
		if marks == 0 {
			continue
		}
		// Intra-row adjacency: any two bits within distance 1 fails.
		if marks&(marks<<1) != 0 {
			return false
		}
		// Next-row adjacency (covers vertical + diagonal).
		if r+1 < s.n {
			spread := marks | (marks << 1) | (marks >> 1)
			full := uint16(1)<<uint(s.n) - 1
			spread &= full
			if s.marks[r+1]&spread != 0 {
				return false
			}
		}
	}
	return true
}

// appendSolutionMarks appends the current marks in row-major order to dst and
// returns the resulting slice. Passing a dst pre-sized to the solution cap
// (n*k) lets the caller reuse the backing array across Generate calls — the
// R-065 orchestrator will borrow a buffer from the Generator, so this API
// must not allocate on its own hot path.
func (s *solverState) appendSolutionMarks(dst []Mark) []Mark {
	for r := range s.n {
		m := s.marks[r]
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			dst = append(dst, Mark{Row: r, Col: c})
		}
	}
	return dst
}
