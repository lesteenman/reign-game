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
// registry. tier4 is currently empty, so no puzzle classifies above Tier 3.
type ruleset struct {
	tier1 []ruleFunc
	tier2 []ruleFunc
	tier3 []ruleFunc
	tier4 []ruleFunc
}

// defaultRuleset returns the live R1..R5, R7 registry (tier4 is empty).
// Returns a *ruleset so callers can pass it to solveWith without the 96-byte
// value copy; the underlying rule-func slices are shared across calls
// (immutable).
func defaultRuleset() *ruleset {
	return &ruleset{
		tier1: []ruleFunc{ruleAdjacencyElimination, ruleCountSaturation, ruleForcedPlacement},
		tier2: []ruleFunc{ruleSingleLineRegion, ruleSingleRegionLine},
		tier3: []ruleFunc{ruleAdjacencyForcing},
		tier4: nil,
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
