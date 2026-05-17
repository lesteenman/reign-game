package generator

import (
	"math/bits"
)

// ruleFunc is the uniform signature for all deductive rules. A rule inspects
// *solverState, optionally mutates it, and reports whether it changed the
// state. No I/O, no globals, no allocations.
//
// Pure side effects: cands, marks, rowNeed, colNeed, regNeed,
// regCellsByRow, and (if recording is enabled) trace.
type ruleFunc func(s *solverState) bool

// ruleset is the ordered registry of rules by tier. solver.go's fixed-point
// loop walks tiers in ascending order and restarts from Tier 1 whenever any
// rule in any tier fires. A concrete rule-removal test (rules_test.go's
// necessity fixtures) calls solver.go's solveWith() to run against a pruned
// registry.
type ruleset struct {
	tier1 []ruleFunc
	tier2 []ruleFunc
	tier3 []ruleFunc
	tier4 []ruleFunc
}

// defaultRuleset returns the full R1..R9 registry. Returns a *ruleset so
// callers can pass it to solveWith without the 96-byte value copy; the
// underlying rule-func slices are shared across calls (immutable).
func defaultRuleset() *ruleset {
	return &ruleset{
		tier1: []ruleFunc{ruleAdjacencyElimination, ruleCountSaturation, ruleForcedPlacement},
		tier2: []ruleFunc{ruleSingleLineRegion, ruleSingleRegionLine},
		tier3: []ruleFunc{ruleLockedKSetInLine, ruleAdjacencyForcing},
		tier4: []ruleFunc{ruleKLineSubset, ruleRegionPairExclusion},
	}
}

// ---------------------------------------------------------------------------
// Tier 1
// ---------------------------------------------------------------------------

// ruleAdjacencyElimination (R1): for every placed mark, eliminate the 8
// neighbors from the candidate set. Idempotent — reruns after no new mark is
// placed do nothing.
func ruleAdjacencyElimination(s *solverState) bool {
	n := s.n
	changed := false
	for r := range n {
		marks := s.marks[r]
		if marks == 0 {
			continue
		}
		neigh := adjacentColumnsMask(marks, n)
		// Same row: eliminate adjacency-column candidates.
		if s.eliminateCandMask(r, neigh) {
			changed = true
		}
		// Row above.
		if r > 0 {
			if s.eliminateCandMask(r-1, neigh) {
				changed = true
			}
		}
		// Row below.
		if r+1 < n {
			if s.eliminateCandMask(r+1, neigh) {
				changed = true
			}
		}
	}
	if changed {
		s.record(ruleEvent{Rule: ruleR1})
	}
	return changed
}

// ruleCountSaturation (R2): a row/col/region with k marks already placed has
// no more marks to place; eliminate the rest of its candidates.
//
// K-parameterized: triggered when rowNeed/colNeed/regNeed reaches 0.
func ruleCountSaturation(s *solverState) bool {
	n := s.n
	changed := false

	for r := range n {
		if s.rowNeed[r] == 0 && s.cands[r] != 0 {
			if s.eliminateCandMask(r, s.cands[r]) {
				changed = true
			}
		}
	}
	for c := range n {
		if s.colNeed[c] != 0 {
			continue
		}
		bit := uint16(1) << uint(c)
		for r := range n {
			if s.cands[r]&bit == 0 {
				continue
			}
			if s.eliminateCand(r, c) {
				changed = true
			}
		}
	}
	for g := range n {
		if s.regNeed[g] != 0 {
			continue
		}
		for r := range n {
			gm := s.regCellsByRow[g][r]
			if gm == 0 {
				continue
			}
			if s.eliminateCandMask(r, gm) {
				changed = true
			}
		}
	}
	if changed {
		s.record(ruleEvent{Rule: ruleR2})
	}
	return changed
}

// ruleForcedPlacement (R3): a row/col/region that needs exactly m more marks
// and has exactly m candidates remaining: place all m.
//
// K-parameterized: "m" ranges over [1, k]; the rule is symmetric in k.
//
// Soundness (R-066 fix): at k=2 the m candidates might be pairwise 8-neighbor
// adjacent, which would make the "forced placement" INVALID. R3 now skips the
// firing when any pair of candidates-to-place is 8-adjacent. A separate rule
// (R1/R7) is expected to eliminate the offending cand; if no elimination is
// possible, the puzzle genuinely has no solution and another rule (or the
// contradicts() check) will flag it.
func ruleForcedPlacement(s *solverState) bool {
	n := s.n
	changed := false

	// Rows.
	for r := range n {
		need := int(s.rowNeed[r])
		if need == 0 {
			continue
		}
		cands := s.cands[r]
		if bits.OnesCount16(cands) != need {
			continue
		}
		// Soundness: reject if any pair of chosen columns is intra-row
		// adjacent. cands & (cands << 1) has bits where c AND c-1 are
		// both set — i.e. an adjacent pair exists.
		if cands&(cands<<1) != 0 {
			continue
		}
		m := cands
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if s.placeMark(r, c) {
				changed = true
			}
		}
	}

	// Columns.
	for c := range n {
		need := int(s.colNeed[c])
		if need == 0 {
			continue
		}
		cmask := s.colCandidateMask(c)
		if bits.OnesCount16(cmask) != need {
			continue
		}
		// Soundness: reject if any pair of chosen rows is intra-column
		// adjacent.
		if cmask&(cmask<<1) != 0 {
			continue
		}
		m := cmask
		for m != 0 {
			r := bits.TrailingZeros16(m)
			m &^= 1 << r
			if s.placeMark(r, c) {
				changed = true
			}
		}
	}

	// Regions.
	for g := range n {
		need := int(s.regNeed[g])
		if need == 0 {
			continue
		}
		total := 0
		for r := range n {
			total += bits.OnesCount16(s.regCellsByRow[g][r])
		}
		if total != need {
			continue
		}
		// Soundness (k>=2): reject if any two region cands are 8-adjacent.
		// For k=1 this is a no-op (single cell).
		if s.k >= 2 && regionHasAdjacentCands(s, g) {
			continue
		}
		for r := range n {
			mask := s.regCellsByRow[g][r]
			for mask != 0 {
				c := bits.TrailingZeros16(mask)
				mask &^= 1 << c
				if s.placeMark(r, c) {
					changed = true
				}
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR3})
	}
	return changed
}

// regionHasAdjacentCands returns true iff region g has any two remaining
// candidate cells that are 8-neighbor adjacent. Used by R3 at k>=2 to skip
// firing when placing all cands would create an adjacency violation.
func regionHasAdjacentCands(s *solverState, g int) bool {
	n := s.n
	for r := range n {
		row := s.regCellsByRow[g][r]
		if row == 0 {
			continue
		}
		// Intra-row adjacency within region cands.
		if row&(row<<1) != 0 {
			return true
		}
		// Next-row adjacency (vertical + diagonal).
		if r+1 < n {
			below := s.regCellsByRow[g][r+1]
			if below == 0 {
				continue
			}
			full := uint16(1)<<uint(n) - 1
			spread := row | (row << 1) | (row >> 1)
			spread &= full
			if below&spread != 0 {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Tier 2
// ---------------------------------------------------------------------------

// ruleSingleLineRegion (R4): if all of a region's remaining candidates lie
// in one row (resp. column), that row must host the region's marks.
// Eliminate the row's candidates that fall outside that region.
//
// K-parameterized: works identically for k in {1, 2}. The region still needs
// regNeed[g] marks; all of them come from that one line.
func ruleSingleLineRegion(s *solverState) bool {
	n := s.n
	changed := false

	for g := range n {
		if s.regNeed[g] == 0 {
			continue
		}

		// Find the row(s) that contain this region's remaining candidates.
		var theRow int
		rowCount := 0
		for r := range n {
			if s.regCellsByRow[g][r] != 0 {
				rowCount++
				theRow = r
			}
		}
		if rowCount == 1 {
			// All region candidates are on row `theRow`. Region cells on
			// that row must absorb regNeed[g] marks. Other candidates on
			// that row cannot be marks for this region — and since each row
			// has rowNeed[r] == regNeed[g]+other-region marks, the other
			// row candidates on the SAME row compete only if they are
			// themselves region g. But we already have all region-g
			// candidates on this row — so any cand on this row NOT in
			// regCellsByRow[g][theRow] is not in region g (trivially true).
			//
			// The real elimination target: candidates on OTHER rows in the
			// same COLUMN as a region-g cell on theRow? No — that's R5's
			// territory. R4 eliminates row-axis: candidates on theRow that
			// are NOT in region g can be eliminated... but wait, a row has
			// rowNeed marks total, some of which may belong to region g
			// and some not. The rule holds only when rowNeed[theRow] ==
			// regNeed[g]: then every row-mark IS a region-g mark, so we
			// can eliminate non-region-g candidates on that row.
			if int(s.rowNeed[theRow]) == int(s.regNeed[g]) {
				nonRegion := s.cands[theRow] &^ s.regCellsByRow[g][theRow]
				if nonRegion != 0 {
					if s.eliminateCandMask(theRow, nonRegion) {
						changed = true
					}
				}
			}
		}

		// Symmetric column check: all region candidates in one column.
		var theCol int
		colCount := 0
		var colOccupancy [nMax]uint16 // col -> mask of rows with region cand at this col
		for r := range n {
			mask := s.regCellsByRow[g][r]
			for mask != 0 {
				c := bits.TrailingZeros16(mask)
				mask &^= 1 << c
				colOccupancy[c] |= 1 << uint(r)
			}
		}
		for c := range n {
			if colOccupancy[c] != 0 {
				colCount++
				theCol = c
			}
		}
		if colCount == 1 {
			// All region candidates are in column `theCol`. Region cells in
			// that column must absorb regNeed[g] marks. If colNeed[theCol]
			// equals regNeed[g], non-region candidates in that column are
			// eliminable.
			if int(s.colNeed[theCol]) == int(s.regNeed[g]) {
				bitCol := uint16(1) << uint(theCol)
				for r := range n {
					if s.cands[r]&bitCol == 0 {
						continue
					}
					if s.regOf[r][theCol] == uint8(g) {
						continue
					}
					if s.eliminateCand(r, theCol) {
						changed = true
					}
				}
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR4})
	}
	return changed
}

// ruleSingleRegionLine (R5): if all of a row's (or column's) remaining
// candidates lie in one region, that region absorbs all the line's marks.
// Eliminate the region's candidates outside the line.
//
// Symmetric to R4. K-parameterized: works identically for k in {1, 2}.
func ruleSingleRegionLine(s *solverState) bool {
	n := s.n
	changed := false

	// Row axis: find rows whose candidates all lie in one region.
	for r := range n {
		if s.rowNeed[r] == 0 {
			continue
		}
		cands := s.cands[r]
		if cands == 0 {
			continue
		}
		firstBit := bits.TrailingZeros16(cands)
		targetRegion := s.regOf[r][firstBit]
		onlyOne := true
		m := cands &^ (1 << uint(firstBit))
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if s.regOf[r][c] != targetRegion {
				onlyOne = false
				break
			}
		}
		if !onlyOne {
			continue
		}
		// All row candidates are in region targetRegion. That region must
		// absorb rowNeed[r] of its marks from this row. Eliminate region
		// candidates on OTHER rows, when regNeed[targetRegion] ==
		// rowNeed[r]: then every region mark comes from this row, and
		// other-row candidates can be eliminated.
		if int(s.regNeed[targetRegion]) != int(s.rowNeed[r]) {
			continue
		}
		for rr := range n {
			if rr == r {
				continue
			}
			regMask := s.regCellsByRow[targetRegion][rr]
			if regMask == 0 {
				continue
			}
			if s.eliminateCandMask(rr, regMask) {
				changed = true
			}
		}
	}

	// Column axis: find columns whose candidates all lie in one region.
	for c := range n {
		if s.colNeed[c] == 0 {
			continue
		}
		cmask := s.colCandidateMask(c)
		if cmask == 0 {
			continue
		}
		firstRow := bits.TrailingZeros16(cmask)
		targetRegion := s.regOf[firstRow][c]
		onlyOne := true
		m := cmask &^ (1 << uint(firstRow))
		for m != 0 {
			r := bits.TrailingZeros16(m)
			m &^= 1 << r
			if s.regOf[r][c] != targetRegion {
				onlyOne = false
				break
			}
		}
		if !onlyOne {
			continue
		}
		if int(s.regNeed[targetRegion]) != int(s.colNeed[c]) {
			continue
		}
		// Eliminate region-targetRegion candidates in columns != c.
		bitC := uint16(1) << uint(c)
		for rr := range n {
			regMask := s.regCellsByRow[targetRegion][rr] &^ bitC
			if regMask == 0 {
				continue
			}
			if s.eliminateCandMask(rr, regMask) {
				changed = true
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR5})
	}
	return changed
}

// ---------------------------------------------------------------------------
// Tier 3
// ---------------------------------------------------------------------------

// ruleLockedKSetInLine (R6): a row (or column) whose only candidates are
// exactly k cells AND those cells all belong to the same region. Those k
// cells are the row's marks — place them, and eliminate the rest of that
// region's candidates outside the line.
//
// K-parameterized: the "locked pair" generalizes to "locked k-set". For k=1
// this is the traditional naked single (which R3 also handles); R6 adds the
// region-sharing inference for k=1 as well (when a single cand exists and
// shares a region with... itself, trivially — the non-trivial content is
// k>=2).
//
// Subsumption finding (R-064): R6's precondition (rowNeed==k AND cands
// count==k AND all k cands share a region with regNeed==k) is a STRICT
// superset of R3's row-axis precondition (rowNeed==cands count). R3 (Tier
// 1) always fires first in the fixed-point loop; after R3 places the k
// marks, regNeed[target] drops to 0 and R2 eliminates the region's other
// cells. This subsumption holds at both k=1 and k=2. R6's deductive
// content is not observable in end-state at any reachable fixture; its
// necessity test asserts trace-level absence instead. Retained for spec
// clarity and potential future weakening (e.g., fire when cands > rowNeed
// with shared region).
func ruleLockedKSetInLine(s *solverState) bool {
	n := s.n
	k := s.k
	changed := false

	for r := range n {
		if int(s.rowNeed[r]) != k {
			continue
		}
		cands := s.cands[r]
		if bits.OnesCount16(cands) != k {
			continue
		}
		// Check all k candidates share a region.
		firstBit := bits.TrailingZeros16(cands)
		targetRegion := s.regOf[r][firstBit]
		same := true
		m := cands &^ (1 << uint(firstBit))
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if s.regOf[r][c] != targetRegion {
				same = false
				break
			}
		}
		if !same {
			continue
		}
		if int(s.regNeed[targetRegion]) != k {
			continue
		}
		// Eliminate region candidates outside this row. We deliberately do
		// NOT place the marks here (R3 will pick them up next pass once
		// other regions' candidates outside this row are eliminated). The
		// elimination IS the deductive content of R6 that R3 alone cannot
		// derive.
		for rr := range n {
			if rr == r {
				continue
			}
			if s.eliminateCandMask(rr, s.regCellsByRow[targetRegion][rr]) {
				changed = true
			}
		}
	}

	// Column axis.
	for c := range n {
		if int(s.colNeed[c]) != k {
			continue
		}
		cmask := s.colCandidateMask(c)
		if bits.OnesCount16(cmask) != k {
			continue
		}
		firstRow := bits.TrailingZeros16(cmask)
		targetRegion := s.regOf[firstRow][c]
		same := true
		m := cmask &^ (1 << uint(firstRow))
		for m != 0 {
			r := bits.TrailingZeros16(m)
			m &^= 1 << r
			if s.regOf[r][c] != targetRegion {
				same = false
				break
			}
		}
		if !same {
			continue
		}
		if int(s.regNeed[targetRegion]) != k {
			continue
		}
		bitC := uint16(1) << uint(c)
		for rr := range n {
			regMask := s.regCellsByRow[targetRegion][rr] &^ bitC
			if regMask == 0 {
				continue
			}
			if s.eliminateCandMask(rr, regMask) {
				changed = true
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR6})
	}
	return changed
}

// ruleAdjacencyForcing (R7): if placing a mark at cell X would force (via
// R3) two 8-neighbor-adjacent marks in some unit, eliminate X.
//
// Detection strategy: for each candidate X, check each row/col/region that
// would react to X being placed. If any reaction forces another cell within
// 8-neighbor distance of X, X is untenable.
//
// Concretely: placing X at (r, c) updates rowNeed[r]--, colNeed[c]--,
// regNeed[regOf[r][c]]--. For each other unit that shares a cell with the
// 8-neighborhood of X, check whether that unit would then reduce to a
// needs-count that matches its candidate count MINUS the neighbors of X
// (which R1 would eliminate). If the match is exact AND the only-remaining
// candidates include any 8-neighbor of X, placing X forces its neighbor —
// contradiction. Eliminate X.
//
// K-parameterized via the needs/candidate comparison.
func ruleAdjacencyForcing(s *solverState) bool {
	n := s.n
	changed := false

	for r := range n {
		cands := s.cands[r]
		m := cands
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			if forcesAdjacentMark(s, r, c) {
				if s.eliminateCand(r, c) {
					changed = true
				}
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR7})
	}
	return changed
}

// forcesAdjacentMark returns true iff placing a mark at (r, c) would force
// the solver to subsequently place a mark at a cell 8-neighbor-adjacent to
// (r, c), which is a contradiction.
//
// We simulate the immediate one-step consequence of placing X: the
// 8-neighborhood of X becomes ineligible as candidates. For every unit
// (row, column, region) that needs marks, check whether after the
// elimination the unit would be forced (remaining candidates = needs) and
// the forced candidates include a neighbor of X.
func forcesAdjacentMark(s *solverState, r, c int) bool {
	n := s.n

	// Build the 8-neighborhood of (r, c) as a column-spread mask covering
	// cols c-1, c, c+1 (bounded to [0, n)).
	colSpread := uint16(1) << uint(c)
	colSpread |= colSpread << 1
	colSpread |= colSpread >> 1
	full := uint16(1)<<uint(n) - 1
	colSpread &= full
	// Exclude (r, c) itself — it's becoming a mark, not a candidate.
	selfMask := uint16(1) << uint(c)

	// Column c: placing X contributes one mark. colNeed--, and column-c
	// candidates in rows r±1 are eliminated (neighbors).
	colRows := s.colCandidateMask(c)
	colRows &^= 1 << uint(r) // remove X
	neighRowMask := uint16(0)
	if r > 0 {
		neighRowMask |= 1 << uint(r-1)
	}
	if r+1 < n {
		neighRowMask |= 1 << uint(r+1)
	}
	colRemaining := colRows &^ neighRowMask
	newColNeed := int(s.colNeed[c]) - 1

	// Column infeasibility check: placing X + adjacency-elimination of
	// column-c cells on rows r±1 leaves fewer candidates than the
	// column still needs. That's a hard contradiction.
	if newColNeed > 0 && bits.OnesCount16(colRemaining) < newColNeed {
		return true
	}

	// Region regOf[r][c]: placing X contributes one mark. regNeed--, and the
	// 8-neighborhood of X within that region becomes ineligible (R1).
	// Similar infeasibility check.
	rid := int(s.regOf[r][c])
	newRegNeed := int(s.regNeed[rid]) - 1
	if newRegNeed > 0 {
		remaining := 0
		for rr := range n {
			mask := s.regCellsByRow[rid][rr]
			switch rr {
			case r:
				mask &^= selfMask
				mask &^= colSpread
			case r - 1, r + 1:
				mask &^= colSpread
			}
			remaining += bits.OnesCount16(mask)
		}
		if remaining < newRegNeed {
			return true // region infeasible — placing X leaves too few cells
		}
	}

	// Row r check: rowNeed[r] decremented by 1; remaining cands on row
	// r = cands[r] &^ colSpread &^ selfMask. If that's < newRowNeed,
	// infeasible.
	rowRemaining := s.cands[r] &^ colSpread
	rowRemaining &^= selfMask
	newRowNeed := int(s.rowNeed[r]) - 1
	if newRowNeed > 0 && bits.OnesCount16(rowRemaining) < newRowNeed {
		return true
	}

	// Row r-1 and r+1: their cands lose colSpread. If rowNeed now exceeds
	// remaining cands, the row is infeasible. This is the classical
	// pattern that R7 catches: a neighbor row has k candidates all within
	// X's colSpread → placing X kills all of them, row is stranded.
	for _, rr := range [2]int{r - 1, r + 1} {
		if rr < 0 || rr >= n {
			continue
		}
		if s.rowNeed[rr] == 0 {
			continue
		}
		rem := s.cands[rr] &^ colSpread
		need := int(s.rowNeed[rr])
		if bits.OnesCount16(rem) < need {
			return true
		}
	}

	// Column c±1: placing X eliminates rows r-1..r+1 from their candidate
	// pool (8-neighbors). If the column's need now exceeds remaining
	// candidate rows, infeasible.
	for _, cc := range [2]int{c - 1, c + 1} {
		if cc < 0 || cc >= n {
			continue
		}
		if s.colNeed[cc] == 0 {
			continue
		}
		cmask := s.colCandidateMask(cc)
		// X's 8-neighbors in column cc are rows r-1, r, r+1 — all in
		// colSpread via column cc. Strip those from cmask.
		strip := uint16(1) << uint(r)
		if r > 0 {
			strip |= 1 << uint(r-1)
		}
		if r+1 < n {
			strip |= 1 << uint(r+1)
		}
		cmask &^= strip
		if bits.OnesCount16(cmask) < int(s.colNeed[cc]) {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// Tier 4
// ---------------------------------------------------------------------------

// ruleKLineSubset (R8): across K rows whose combined remaining candidates
// span only K columns, those K columns hold all marks for those rows.
// Eliminate candidates in those columns from OTHER rows.
//
// For k=1, K=1: single row with a single candidate — degenerate to R3.
// For k=2, K=2: the classical X-wing. Two rows each needing 2 marks; their
// combined candidates span exactly 2 columns (= K*k marks). Symmetric for
// columns.
//
// The "K rows" size is k (the spec's marksPerUnit). So at k=1 this rule
// degenerates. We still run it for completeness.
//
// Subsumption finding (R-064): R8's precondition (rowNeed[r1]==k AND
// rowNeed[r2]==k AND |cands[r1] | cands[r2]| == k) forces each row's cand
// count to be at most k; with rowNeed==k, each row's cands count ==
// rowNeed exactly, which is R3's row-axis firing precondition. R3 (Tier
// 1) always fires first; R8 (Tier 4) never contributes distinct progress
// in the fixed-point loop. Necessity test asserts trace-level absence of
// ruleR8 events when R8 is pruned. Retained for spec clarity and potential
// future strengthening (e.g., allow cands > rowNeed when marks pre-placed,
// or move to Tier 1).
func ruleKLineSubset(s *solverState) bool {
	n := s.n
	k := s.k
	changed := false

	if k < 2 {
		// At k=1 R8 is strictly weaker than R3 (since K=1 and single row
		// with single cand is R3 territory). Return false — no k=1
		// contribution. This matches the design intent: R8 is Tier 4
		// reasoning and contributes meaningfully only at k>=2.
		return false
	}

	// Row axis: find K rows whose combined candidates span only K columns.
	// For k=2 we scan all pairs of rows both needing 2 marks.
	for r1 := 0; r1 < n-1; r1++ {
		if s.rowNeed[r1] != uint8(k) {
			continue
		}
		for r2 := r1 + 1; r2 < n; r2++ {
			if s.rowNeed[r2] != uint8(k) {
				continue
			}
			combined := s.cands[r1] | s.cands[r2]
			if bits.OnesCount16(combined) != k {
				continue
			}
			// Soundness (R-066): at k=2, the two columns must hold both
			// rows' k marks simultaneously. If the two columns are
			// intra-row adjacent, no valid placement exists (8-adjacency
			// violation in each row). If r2=r1+1 and adjacent columns
			// exist, diagonal adjacency blocks it too. Skip in these
			// cases — the rule would otherwise make unsound eliminations
			// on the "other rows" whose cands this fixture can't
			// actually constrain.
			if combined&(combined<<1) != 0 {
				continue
			}
			// Those K columns hold every mark for r1, r2. Eliminate those
			// columns' candidates in other rows.
			for rr := range n {
				if rr == r1 || rr == r2 {
					continue
				}
				if s.eliminateCandMask(rr, combined) {
					changed = true
				}
			}
		}
	}

	// Column axis: K columns whose combined candidate rows span only K rows.
	for c1 := 0; c1 < n-1; c1++ {
		if s.colNeed[c1] != uint8(k) {
			continue
		}
		m1 := s.colCandidateMask(c1)
		for c2 := c1 + 1; c2 < n; c2++ {
			if s.colNeed[c2] != uint8(k) {
				continue
			}
			m2 := s.colCandidateMask(c2)
			combined := m1 | m2
			if bits.OnesCount16(combined) != k {
				continue
			}
			// Soundness (R-066): symmetric adjacency check on the rows.
			if combined&(combined<<1) != 0 {
				continue
			}
			// Those K rows hold every mark for c1, c2. Eliminate (other)
			// columns' cands in those rows. We can only eliminate cands on
			// rows in `combined` and columns OTHER than c1, c2.
			rowMask := combined
			for rowMask != 0 {
				r := bits.TrailingZeros16(rowMask)
				rowMask &^= 1 << r
				strip := s.cands[r]
				strip &^= 1 << uint(c1)
				strip &^= 1 << uint(c2)
				if strip == 0 {
					continue
				}
				if s.eliminateCandMask(r, strip) {
					changed = true
				}
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR8})
	}
	return changed
}

// ruleRegionPairExclusion (R9): if two regions' combined candidates in a
// single row (or column) span only K cells, those cells hold the region
// marks contributed to that line — no OTHER region's marks in that line
// sit outside those cells.
//
// For k=1, K=1: one region contributes one cell in a line; single region
// coverage — R5 territory. R9 is non-trivial at k>=2.
//
// DISABLED at k=2 (R-066 finding): the rule as implemented is unsound.
// Its preconditions assert "g1 and g2 together supply ALL k marks in
// this row" but the implementation only checks that g1 and g2's combined
// row cands == k — which can fire when g1/g2 put their marks in OTHER
// rows entirely (leaving this row's marks to come from other regions).
// Soundness repair is deferred; disabling R9 is safe because it is
// subsumed in practice (like R6/R8 — see design.md §4.2). The solver
// still reaches unique solutions via R1..R8 at k=2.
func ruleRegionPairExclusion(s *solverState) bool {
	// Temporarily disabled (R-066): unsound precondition at k=2.
	// See comment above.
	_ = s
	return false
}

// ruleRegionPairExclusionOriginal preserves the original (unsound) R9
// implementation. Retained as a live test fixture (see
// TestRuleR9_RegionPairExclusion_Original_Unsound in rules_test.go, which
// pins the demonstrated unsoundness) AND as the starting point for a
// future soundness repair.
//
// TODO: either repair R9's precondition (likely: tighten to require no
// adjacency between the K combined candidate cells across the two regions)
// or delete this function plus the fixture test. Leaving it as an
// unreferenced shadow would rot; tracking it as a TODO is honest
// documentation that Tier 4 classification is currently empty.
//
// Not registered in defaultRuleset.
func ruleRegionPairExclusionOriginal(s *solverState) bool {
	n := s.n
	k := s.k
	changed := false

	if k < 2 {
		return false
	}

	// Row axis.
	for r := range n {
		// Find all regions with candidates on this row.
		var regionsOnRow [nMax]int
		rowRegionCount := 0
		seen := [nMax]bool{}
		for c := 0; c < n; c++ {
			if s.cands[r]&(uint16(1)<<uint(c)) == 0 {
				continue
			}
			rid := s.regOf[r][c]
			if seen[rid] {
				continue
			}
			seen[rid] = true
			regionsOnRow[rowRegionCount] = int(rid)
			rowRegionCount++
		}
		if rowRegionCount < 2 {
			continue
		}
		// Try pairs of regions.
		for i := 0; i < rowRegionCount-1; i++ {
			g1 := regionsOnRow[i]
			for j := i + 1; j < rowRegionCount; j++ {
				g2 := regionsOnRow[j]
				combined := s.regCellsByRow[g1][r] | s.regCellsByRow[g2][r]
				if bits.OnesCount16(combined) != k {
					continue
				}
				// The only cands in row r belonging to regions {g1,g2} are
				// the k cells in `combined`. If g1 and g2 together MUST
				// contribute all k marks on this row (i.e., no other
				// region has cands on this row), then no OTHER cell on
				// this row outside `combined` can be a mark. Eliminate
				// cands on r outside combined if they exist.
				rowCandsOther := s.cands[r] &^ combined
				if rowCandsOther == 0 {
					continue
				}
				// Only fires if every cand on row r belongs to g1 or g2.
				// That's the case iff rowCandsOther & (cells of other
				// regions in row r) == rowCandsOther. By construction
				// rowCandsOther = cands[r] &^ combined, and combined is
				// already g1∪g2's region cells on row r. So rowCandsOther
				// is exactly the cands on row r belonging to OTHER regions.
				// If non-zero, R9 fires if those cands cannot hold a mark
				// — i.e., if the two regions g1, g2 together supply the
				// full row need. That means rowNeed[r] == (k contributions
				// from g1) + (k contributions from g2) = 2k for k=2, but
				// rowNeed[r] is always k. So R9's row-axis form only
				// fires when `combined` accounts for ALL the row's marks,
				// which means EITHER g1's or g2's contribution is zero
				// on that row. For k=2 this means the two regions
				// together contribute only k marks in row r where
				// rowNeed == k: their combined cells (which are k) are
				// the ONLY cells on that row — so other-region cands can
				// be eliminated.
				if s.eliminateCandMask(r, rowCandsOther) {
					changed = true
				}
			}
		}
	}

	// Column axis.
	for c := range n {
		var regionsOnCol [nMax]int
		colRegionCount := 0
		seen := [nMax]bool{}
		for r := 0; r < n; r++ {
			if s.cands[r]&(uint16(1)<<uint(c)) == 0 {
				continue
			}
			rid := s.regOf[r][c]
			if seen[rid] {
				continue
			}
			seen[rid] = true
			regionsOnCol[colRegionCount] = int(rid)
			colRegionCount++
		}
		if colRegionCount < 2 {
			continue
		}
		bitC := uint16(1) << uint(c)
		for i := 0; i < colRegionCount-1; i++ {
			g1 := regionsOnCol[i]
			for j := i + 1; j < colRegionCount; j++ {
				g2 := regionsOnCol[j]
				var rowUnion uint16
				for r := range n {
					if (s.regCellsByRow[g1][r]|s.regCellsByRow[g2][r])&bitC != 0 {
						rowUnion |= 1 << uint(r)
					}
				}
				if bits.OnesCount16(rowUnion) != k {
					continue
				}
				// Eliminate other-region cands in column c outside rowUnion.
				colMask := s.colCandidateMask(c)
				colOther := colMask &^ rowUnion
				if colOther == 0 {
					continue
				}
				m := colOther
				for m != 0 {
					r := bits.TrailingZeros16(m)
					m &^= 1 << r
					rid := s.regOf[r][c]
					if rid == uint8(g1) || rid == uint8(g2) {
						continue
					}
					if s.eliminateCand(r, c) {
						changed = true
					}
				}
			}
		}
	}

	if changed {
		s.record(ruleEvent{Rule: ruleR9})
	}
	return changed
}
