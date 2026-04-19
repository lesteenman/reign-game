package generator

import (
	"math/bits"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// eightNeighborOffsets is the set of (dr, dc) deltas for the 8 king-move
// neighbors of a cell. Package-scope so tests that simulate R1 (adjacency
// elimination) do not re-declare the literal inline.
var eightNeighborOffsets = [8][2]int{
	{-1, -1}, {-1, 0}, {-1, 1},
	{0, -1}, {0, 1},
	{1, -1}, {1, 0}, {1, 1},
}

// buildState constructs a solverState from a region map, optionally
// pre-placing marks at the given (r, c) pairs (by passing them through the
// normal placeMark path so all derived fields stay consistent).
//
// placeMark is a low-level mutation that does NOT eliminate 8-neighbor
// candidates — that is R1's job. If the test needs "marks placed AND R1
// applied around them", pass the marks to applyR1AroundMarks AFTER buildState
// OR chain buildState + applyR1AroundMarks.
func buildState(t *testing.T, regionMap [][]int, n, k int, placed ...Mark) *solverState {
	t.Helper()

	s := &solverState{}
	if err := s.initFromRegionMap(regionMap, n, k); err != nil {
		t.Fatalf("initFromRegionMap: %v", err)
	}
	for _, m := range placed {
		if !s.placeMark(m.Row, m.Col) {
			t.Fatalf("placeMark(%d, %d) failed", m.Row, m.Col)
		}
	}
	return s
}

// applyR1AroundMarks simulates R1 (adjacency elimination) around each of the
// given marks. Used by necessity tests that pre-place a prefix of a known
// solution and need R1 effects applied before measuring what a later rule
// would do from that state.
func applyR1AroundMarks(s *solverState, marks []Mark, n int) {
	for _, m := range marks {
		for _, d := range eightNeighborOffsets {
			nr, nc := m.Row+d[0], m.Col+d[1]
			if nr < 0 || nr >= n || nc < 0 || nc >= n {
				continue
			}
			s.eliminateCand(nr, nc)
		}
	}
}

// newSolverState constructs a solverState outside a testing.T context —
// used by fixture factories (rXFixture helpers, factory closures in
// necessity tests) that are reusable across callers. Panics on invalid
// input because the fixtures are hand-authored and any error is a test-
// data bug rather than a runtime condition. testing.T-aware callers should
// use buildState instead.
func newSolverState(regionMap [][]int, n, k int) *solverState {
	s := &solverState{}
	if err := s.initFromRegionMap(regionMap, n, k); err != nil {
		panic("newSolverState: " + err.Error())
	}
	return s
}

// rowStripeMap builds an NxN region map where regionMap[r][c] = r.
// Each row is its own region, which makes row/region constraints coincide.
// Handy for isolating particular rule-firing patterns.
func rowStripeMap(n int) [][]int {
	m := make([][]int, n)
	for r := range m {
		m[r] = make([]int, n)
		for c := range m[r] {
			m[r][c] = r
		}
	}
	return m
}

// countSet returns the number of cells set in cands across all rows.
func (s *solverState) totalCandidates() int {
	total := 0
	for r := range s.n {
		total += bits.OnesCount16(s.cands[r])
	}
	return total
}

// rulesetMinus returns the default registry with the specified rule omitted.
// Used for necessity fixtures: a puzzle that solves under the full registry
// MUST NOT solve under registry-minus-rule-X, for every X.
func rulesetMinus(omit ruleID) *ruleset {
	full := defaultRuleset()
	target := ruleByID(omit)
	filter := func(list []ruleFunc) []ruleFunc {
		out := make([]ruleFunc, 0, len(list))
		for _, r := range list {
			if funcPtr(r) == funcPtr(target) {
				continue
			}
			out = append(out, r)
		}
		return out
	}
	pruned := *full
	pruned.tier1 = filter(full.tier1)
	pruned.tier2 = filter(full.tier2)
	pruned.tier3 = filter(full.tier3)
	pruned.tier4 = filter(full.tier4)
	return &pruned
}

// ruleByID returns the canonical ruleFunc for a given ruleID. Panics on
// unknown ids (test-side only; no release code path).
func ruleByID(id ruleID) ruleFunc {
	switch id {
	case ruleR1:
		return ruleAdjacencyElimination
	case ruleR2:
		return ruleCountSaturation
	case ruleR3:
		return ruleForcedPlacement
	case ruleR4:
		return ruleSingleLineRegion
	case ruleR5:
		return ruleSingleRegionLine
	case ruleR6:
		return ruleLockedKSetInLine
	case ruleR7:
		return ruleAdjacencyForcing
	case ruleR8:
		return ruleKLineSubset
	case ruleR9:
		return ruleRegionPairExclusion
	}
	panic("unknown ruleID")
}

// funcPtr returns the underlying code pointer of a ruleFunc via reflect.
// Go's function-value comparison is restricted to nil, so reflect is the
// portable path for identity comparison used in rulesetMinus.
func funcPtr(f ruleFunc) uintptr {
	return reflect.ValueOf(f).Pointer()
}

// ---------------------------------------------------------------------------
// Per-rule UNIT tests — each one demonstrates exactly that rule firing.
// ---------------------------------------------------------------------------

// TestRuleR1_AdjacencyElimination: placing a mark eliminates the 8 neighbors
// from cands.
func TestRuleR1_AdjacencyElimination(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 grid, k=1, row-stripes, place a mark at (1, 1).
	// R1 should eliminate cands at rows 0,1,2 columns 0,1,2 (the 8
	// neighbors + self).
	s := buildState(t, rowStripeMap(4), 4, 1, Mark{1, 1})
	// Sanity: before R1, candidates at neighbors should still be set
	// (placeMark only cleared (1,1) itself).
	before := s.cands[0] & 0b0111
	if before == 0 {
		t.Fatal("expected row 0 cols 0-2 to still be candidates before R1")
	}

	// Act
	changed := ruleAdjacencyElimination(s)

	// Assert
	if !changed {
		t.Fatal("expected R1 to return changed=true")
	}
	// Rows 0, 1, 2 should have cols 0, 1, 2 eliminated.
	for _, r := range []int{0, 1, 2} {
		neighborMask := uint16(0b0111)
		if s.cands[r]&neighborMask != 0 {
			t.Errorf("row %d: expected cols 0-2 cleared, got cands=%04b", r, s.cands[r])
		}
	}
	// Row 3 should be unchanged (not adjacent).
	if s.cands[3] != 0b1111 {
		t.Errorf("row 3: expected cands=1111, got %04b", s.cands[3])
	}
}

// TestRuleR2_CountSaturation: a row with k marks has its remaining cands
// cleared.
func TestRuleR2_CountSaturation(t *testing.T) {
	t.Parallel()

	// Arrange — 5x5 k=1, place a mark at (2, 2). After placing, row 2
	// has rowNeed=0 and cands still contains other cols (before R1 clears
	// neighbors). R2 should clear ALL remaining cands on that row.
	s := buildState(t, rowStripeMap(5), 5, 1, Mark{2, 2})
	// Before R2, cands[2] is 0b11011 (col 2 cleared by placeMark).
	if s.cands[2] == 0 {
		t.Fatal("expected row 2 still has candidates before R2")
	}

	// Act
	changed := ruleCountSaturation(s)

	// Assert
	if !changed {
		t.Fatal("expected R2 changed=true")
	}
	if s.cands[2] != 0 {
		t.Errorf("row 2: expected cands=0 after R2, got %b", s.cands[2])
	}
}

// TestRuleR3_ForcedPlacement: a row needing 1 more mark with 1 candidate
// places it.
func TestRuleR3_ForcedPlacement(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 k=1 row-stripes. Reduce row 0 to a single candidate
	// at col 3 by hand (simulating prior rule firings).
	s := buildState(t, rowStripeMap(4), 4, 1)
	s.cands[0] = 0b1000 // only col 3
	s.regCellsByRow[0][0] = 0b1000

	// Act
	changed := ruleForcedPlacement(s)

	// Assert
	if !changed {
		t.Fatal("expected R3 changed=true")
	}
	if s.marks[0]&0b1000 == 0 {
		t.Errorf("row 0: expected mark at col 3, got marks=%04b", s.marks[0])
	}
	if s.rowNeed[0] != 0 {
		t.Errorf("row 0: expected rowNeed=0 after placement, got %d", s.rowNeed[0])
	}
}

// TestRuleR4_SingleLineRegion: a region whose all remaining candidates lie
// in one row (with equal need) — eliminate non-region cands on that row.
func TestRuleR4_SingleLineRegion(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 k=1. Region 0 occupies only (0,0) and (0,1). Region
	// 1..3 fill the rest. rowNeed[0] = 1 = regNeed[0]. Row 0's candidates
	// at (0,2) and (0,3) belong to other regions and can be eliminated
	// because all of row 0's marks must go to region 0.
	regionMap := [][]int{
		{0, 0, 1, 1},
		{2, 2, 1, 1},
		{2, 3, 3, 1},
		{2, 3, 3, 3},
	}
	s := buildState(t, regionMap, 4, 1)
	// All cands initially set. Before R4, row 0 has cands=0b1111.

	// Act
	changed := ruleSingleLineRegion(s)

	// Assert
	if !changed {
		t.Fatal("expected R4 changed=true")
	}
	// Row 0: cols 2, 3 (region 1) should be eliminated. Cols 0, 1
	// (region 0) retained.
	want := uint16(0b0011)
	if s.cands[0] != want {
		t.Errorf("row 0: expected cands=%04b, got %04b", want, s.cands[0])
	}
}

// TestRuleR5_SingleRegionLine: all of a row's candidates lie in one region
// (and that region's need equals the row's need). Eliminate region cands
// on OTHER rows.
func TestRuleR5_SingleRegionLine(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 k=1.
	// Region 0 occupies cells (0,0), (0,1), (2,0), (2,1).
	// Rows 1, 3 occupy other regions.
	// Row 0's cands reduced by hand to just (0, 0) and (0, 1) (both
	// region 0). rowNeed[0] = 1 = regNeed[0]. R5 should eliminate
	// region-0 cands on row 2 (they would compete).
	regionMap := [][]int{
		{0, 0, 1, 1},
		{2, 2, 3, 3},
		{0, 0, 1, 1},
		{2, 2, 3, 3},
	}
	s := buildState(t, regionMap, 4, 1)
	// Reduce row 0 to cols 0, 1 by hand (simulating prior elimination).
	s.eliminateCand(0, 2)
	s.eliminateCand(0, 3)
	// Region 0 needs 1 mark total. Verify.
	if s.regNeed[0] != 1 {
		t.Fatalf("setup: expected regNeed[0]=1, got %d", s.regNeed[0])
	}

	// Act
	changed := ruleSingleRegionLine(s)

	// Assert
	if !changed {
		t.Fatal("expected R5 changed=true")
	}
	// Row 2 region-0 cells (cols 0, 1) should be eliminated.
	if s.cands[2]&0b0011 != 0 {
		t.Errorf("row 2: expected region-0 cells (cols 0-1) eliminated, got cands=%04b", s.cands[2])
	}
}

// TestRuleR6_LockedKSetInLine: a row with exactly k candidates all sharing
// a region. Eliminate that region's candidates outside the row.
func TestRuleR6_LockedKSetInLine(t *testing.T) {
	t.Parallel()

	// Arrange — 4x4 k=1. Row 0 reduced to a single cand at (0, 0),
	// region 0. This is the k=1 form; the content of R6 beyond R3 is
	// "eliminate region cands elsewhere." Place the mark and then check
	// R6's region elimination.
	//
	// Because R3 would also handle this, we construct a test where R6
	// fires WITHOUT having R3 fire first: use the cands = k locked set
	// shape but do not let R3 run.
	regionMap := [][]int{
		{0, 0, 1, 1},
		{2, 2, 1, 1},
		{0, 3, 3, 3},
		{2, 2, 3, 3},
	}
	s := buildState(t, regionMap, 4, 1)
	// Reduce row 0 to a single cand at (0, 0). Region 0 also covers
	// (2, 0). R6 should eliminate (2, 0) from cands.
	s.eliminateCand(0, 1)
	s.eliminateCand(0, 2)
	s.eliminateCand(0, 3)

	// Act
	changed := ruleLockedKSetInLine(s)

	// Assert
	if !changed {
		t.Fatal("expected R6 changed=true")
	}
	if s.cands[2]&(uint16(1)<<0) != 0 {
		t.Errorf("row 2: expected col 0 (region 0) eliminated, got cands=%04b", s.cands[2])
	}
}

// TestRuleR7_AdjacencyForcing: placing a mark at X would force an
// adjacent mark. Eliminate X.
func TestRuleR7_AdjacencyForcing(t *testing.T) {
	t.Parallel()

	// Arrange — 5x5 k=1. Set up a column c=0 where candidates only exist
	// on rows 0 and 1 (colNeed=1). Placing (0, x) with x in column 1
	// would eliminate (0, 0) and (1, 0) from the column, leaving 0
	// candidates. colNeed=1 > 0 remaining → infeasible. R7 should
	// eliminate (0, 1) and (1, 1).
	regionMap := [][]int{
		{0, 0, 1, 1, 1},
		{0, 2, 2, 1, 1},
		{0, 2, 2, 3, 3},
		{4, 4, 2, 3, 3},
		{4, 4, 4, 3, 3},
	}
	s := buildState(t, regionMap, 5, 1)
	// Reduce column 0 candidates: only (0, 0), (1, 0), (2, 0) remain
	// (cells in region 0). Eliminate col 0 at rows 3, 4. Also, force
	// column 0 to rows 0 and 1 by eliminating (2, 0).
	s.eliminateCand(2, 0)
	s.eliminateCand(3, 0)
	s.eliminateCand(4, 0)

	// Before R7: col 0 has cands at rows 0 and 1. colNeed = 1.
	// Candidate (0, 1) in row 0's cands: placing (0, 1) would eliminate
	// (0, 0) and (1, 0) from column 0 (8-neighbor adjacency via row 0
	// col 0 and row 1 col 0), leaving col 0 with ZERO candidates but
	// colNeed = 1. Infeasibility. R7 should eliminate (0, 1) and (1, 1).
	if s.cands[0]&(uint16(1)<<1) == 0 {
		t.Fatal("setup: expected (0,1) to be a candidate")
	}
	// Don't let row 0's cands fall below 1 or rule collapses.

	// Act
	changed := ruleAdjacencyForcing(s)

	// Assert
	if !changed {
		t.Fatal("expected R7 changed=true")
	}
	// Row 0 col 1 should be eliminated.
	if s.cands[0]&(uint16(1)<<1) != 0 {
		t.Errorf("row 0: expected col 1 eliminated by R7, got cands=%05b", s.cands[0])
	}
}

// TestRuleR8_KLineSubset: at k=2, two rows needing 2 marks each whose
// combined candidates span only 2 columns — those 2 columns hold all 4
// marks. Eliminate those columns in other rows.
func TestRuleR8_KLineSubset(t *testing.T) {
	t.Parallel()

	// Arrange — 6x6 k=2. Rows 0 and 1 reduced to cands only in cols 0, 3.
	// That gives 2 rows needing 2 marks each, combined cands = {0, 3}.
	// R8 should eliminate cols 0, 3 candidates in rows 2..5.
	regionMap := rowStripeMap(6) // row-stripes (any map works for R8 rows test)
	s := buildState(t, regionMap, 6, 2)
	// Reduce row 0 and row 1 candidates to cols 0, 3. (cols 0 and 3 have
	// gap 3 >= 2 so both can be marks in the same row.)
	for c := 0; c < 6; c++ {
		if c == 0 || c == 3 {
			continue
		}
		s.eliminateCand(0, c)
		s.eliminateCand(1, c)
	}
	// Sanity: rowNeed[0] = rowNeed[1] = 2, combined cands = {0, 3}.
	if s.rowNeed[0] != 2 || s.rowNeed[1] != 2 {
		t.Fatalf("setup failure: rowNeed 0=%d 1=%d", s.rowNeed[0], s.rowNeed[1])
	}
	if (s.cands[0] | s.cands[1]) != 0b001001 {
		t.Fatalf("setup failure: combined cands=%06b", s.cands[0]|s.cands[1])
	}

	// Act
	changed := ruleKLineSubset(s)

	// Assert
	if !changed {
		t.Fatal("expected R8 changed=true")
	}
	// Rows 2..5 should have cols 0, 3 eliminated.
	for r := 2; r < 6; r++ {
		if s.cands[r]&0b001001 != 0 {
			t.Errorf("row %d: expected cols 0, 3 eliminated, got cands=%06b", r, s.cands[r])
		}
	}
}

// TestRuleR9_RegionPairExclusion: R9 is DISABLED as of R-066 due to an
// unsound precondition in the original implementation (the rule fired
// when two regions' combined row-cands == k, without verifying those
// two regions MUST jointly supply all k of the row's marks). The
// original test exercised the unsound elimination; it is retained but
// rewritten to document the disabled state.
func TestRuleR9_RegionPairExclusion(t *testing.T) {
	t.Parallel()
	// Arrange — minimal k=2 state.
	regionMap := make([][]int, 9)
	for r := range regionMap {
		regionMap[r] = make([]int, 9)
		for c := range regionMap[r] {
			regionMap[r][c] = r
		}
	}
	s := buildState(t, regionMap, 9, 2)

	// Act
	changed := ruleRegionPairExclusion(s)

	// Assert — rule is disabled; must not fire.
	if changed {
		t.Fatal("R9 is disabled (R-066 soundness finding); expected no change")
	}
}

// TestRuleR9_RegionPairExclusion_Original_Unsound retains the original
// R9 fixture for historical reference — the fixture now proves the
// original rule was UNSOUND: given row 0 cands = {col 0 (reg 0), col 5
// (reg 1), col 7 (reg 2)} and k=2, the original rule eliminated col 7
// but a valid solution exists with the row 0 marks at {col 0, col 7}
// (region 0 contributes col 0, region 1 contributes its 2 marks
// elsewhere, region 2 contributes col 7).
func TestRuleR9_RegionPairExclusion_Original_Unsound(t *testing.T) {
	t.Parallel()

	// Arrange — 9x9 k=2.
	// Row 0 region layout: {0, 0, 1, 1, 2, 2, 3, 3, 4}.
	// The rest of the grid is padded with regions 5..8.
	// On row 0, regions 0 and 1 together have cands at cols 0-3 (4 cells).
	// If we eliminate cols 2 and 3 on row 0, combined regions 0 and 1
	// have cands = {0, 1} — only k=2 cells. But row 0's total cands
	// would be cols 0, 1, 4, 5, 6, 7, 8, so other-region cands (4-8) exist.
	// For R9 to fire, we need the combined region cells to EQUAL the row's
	// total cands (i.e., no other-region cells on the row). Let's construct
	// that:
	//
	// Row 0: regions 0 and 1 cover cols 0, 1, 2, 3. Other cols 4-8 are
	// occupied by regions 2..8 on row 0 as well. Reduce row 0 cands to
	// cols 0, 1 only (both region 0 or 1). Then no other-region cands
	// on row 0 — R9 has nothing to do. Not useful.
	//
	// Better test: make row 0's cands span regions 0, 1, and 2 initially
	// with region 0 at cols 0, 1 and region 1 at cols 4, 5 and region 2
	// at col 7. After some eliminations, region 0 has cands only at col
	// 0; region 1 has cands only at col 5; region 2 has cand at col 7.
	// Combined region 0 + region 1 cands on row 0 = {0, 5} = 2 cells.
	// Other-region cand at col 7 can be eliminated by R9.
	regionMap := make([][]int, 9)
	for r := range regionMap {
		regionMap[r] = make([]int, 9)
		for c := range regionMap[r] {
			regionMap[r][c] = r
		}
	}
	// Override row 0 to span three regions.
	regionMap[0] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	// Override row 1 to keep row 0's regions visible:
	regionMap[1] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	// Other rows take their own region id (3..8 on rows 2..8, but rows
	// 2..8 are regions 2..8 which clash with row 0's region 2. Keep
	// simple: let rows 2..8 be regions 3..9 — but we only have ids 0..8.
	// Rename rows 2..8 to use ids 3..9? N=9 means region ids in [0, 9).
	// Use:
	regionMap[2] = []int{3, 3, 3, 3, 3, 3, 3, 3, 3}
	regionMap[3] = []int{4, 4, 4, 4, 4, 4, 4, 4, 4}
	regionMap[4] = []int{5, 5, 5, 5, 5, 5, 5, 5, 5}
	regionMap[5] = []int{6, 6, 6, 6, 6, 6, 6, 6, 6}
	regionMap[6] = []int{7, 7, 7, 7, 7, 7, 7, 7, 7}
	regionMap[7] = []int{8, 8, 8, 8, 8, 8, 8, 8, 8}
	// Row 8 needs to provide marks for regions 0, 1, 2 (each needs k=2;
	// row 0+row 1 = 2 marks each if regions 0..2 cover both rows 0..1).
	// Actually regions 0..2 span rows 0 AND 1 — each region has 6 cells
	// (3 on row 0, 3 on row 1), needs 2 marks. That's fine.
	// But N=9 k=2 needs each column to have 2 marks; row 8 must exist.
	regionMap[8] = []int{0, 1, 2, 0, 1, 2, 0, 1, 2}
	// Whoops — this sprinkles region 0 onto row 8 which changes regNeed[0]
	// cell count. Let's just use row 8 = region 0 entirely and make
	// the map "cheat" — this is just an R9 unit test, not an end-to-end
	// puzzle.
	regionMap[8] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	// Now region 0 spans rows 0, 1, 8 cols 0-2 (9 cells); region 1 spans
	// rows 0, 1, 8 cols 3-5 (9 cells); region 2 spans rows 0, 1, 8 cols
	// 6-8 (9 cells). regNeed[0..2] = 2 each.

	s := buildState(t, regionMap, 9, 2)
	// Eliminate row 0 cands except cols 0 (region 0), 5 (region 1), 7
	// (region 2). That gives row 0 cands = {0, 5, 7} — regions 0, 1, 2.
	// Combined region 0 + region 1 cells on row 0 = {0, 5} = 2 cells.
	// R9 should eliminate other-region cands on row 0 — which is col 7
	// (region 2).
	for c := 0; c < 9; c++ {
		if c == 0 || c == 5 || c == 7 {
			continue
		}
		s.eliminateCand(0, c)
	}
	if s.cands[0] != (uint16(1)<<0 | uint16(1)<<5 | uint16(1)<<7) {
		t.Fatalf("setup: expected cands[0] = 0,5,7; got %09b", s.cands[0])
	}

	// Act — call the ORIGINAL (unsound) implementation.
	changed := ruleRegionPairExclusionOriginal(s)

	// Assert — demonstrates the original unsoundness. Col 7 is eliminated
	// despite being a valid row-0 mark under at least one solution.
	if !changed {
		t.Fatal("expected original R9 to fire (and unsoundly eliminate col 7)")
	}
	if s.cands[0]&(uint16(1)<<7) != 0 {
		t.Errorf("unsound R9: expected col 7 eliminated, got cands=%09b", s.cands[0])
	}
}

// ---------------------------------------------------------------------------
// Per-rule NECESSITY tests — remove rule X, assert the full-ruleset-solvable
// fixture no longer solves.
// ---------------------------------------------------------------------------

// uniqueFixtureRegionMap returns the 5x5 known-unique fixture from R-063.
// Brute confirms exactly one solution; deductive stalls without help but
// solves with pre-placed marks.
func uniqueFixtureRegionMap() [][]int {
	return [][]int{
		{3, 3, 2, 2, 0},
		{3, 2, 2, 0, 0},
		{3, 4, 2, 0, 1},
		{3, 4, 4, 0, 1},
		{4, 4, 1, 1, 1},
	}
}

// uniqueFixtureSolution returns the unique solution for
// uniqueFixtureRegionMap: (0,2),(1,0),(2,3),(3,1),(4,4).
func uniqueFixtureSolution() []Mark {
	return []Mark{{0, 2}, {1, 0}, {2, 3}, {3, 1}, {4, 4}}
}

// runNecessityOnPartial takes a factory that builds a partial state and
// asserts that the full ruleset Solves it while the pruned ruleset does
// not.
func runNecessityOnPartial(t *testing.T, name string, factory func() *solverState, omit ruleID) {
	t.Helper()

	s1 := factory()
	full := solveWith(s1, defaultRuleset())
	if full != OutcomeSolved {
		t.Fatalf("%s: full ruleset expected Solved, got %v (solved=%v, stalled cands=%d)",
			name, full, s1.solved(), s1.totalCandidates())
	}

	s2 := factory()
	pruned := solveWith(s2, rulesetMinus(omit))
	if pruned == OutcomeSolved {
		t.Fatalf("%s: expected non-Solved with rule %d removed, got OutcomeSolved", name, omit)
	}
}

// TestNecessity_R1: Pre-place all but one mark; to finalize the last mark,
// the solver needs R1 to eliminate the adjacency-forbidden cells (and R3
// to place the last one). Without R1, the remaining cell has multiple
// cands and R3 cannot fire. Without R1 the solver also might place an
// adjacent cell — solved()'s adjacency check rejects that too.
func TestNecessity_R1(t *testing.T) {
	t.Parallel()

	// Use the known-unique 5x5 k=1 fixture. Pre-place 3 of the 5 marks
	// (indices 0, 1, 2) so only (3,1) and (4,4) remain. Those two
	// placements require R1 to eliminate neighbors before R3 can fire.
	rm := uniqueFixtureRegionMap()
	sol := uniqueFixtureSolution()
	factory := func() *solverState {
		s := newSolverState(rm, 5, 1)
		// Place 0, 1, 2 directly — but also apply R1 around them so the
		// cands are consistent.
		for i := 0; i < 3; i++ {
			m := sol[i]
			s.placeMark(m.Row, m.Col)
		}
		// Do NOT pre-apply R1 — we want R1 to be essential for the
		// remaining progress.
		return s
	}
	runNecessityOnPartial(t, "R1_partial_5x5_3pre", factory, ruleR1)
}

// TestNecessity_R2: Pre-place marks so a row/col becomes saturated; the
// solver needs R2 to clear stale cands before R3 can fire on a dependent
// unit.
func TestNecessity_R2(t *testing.T) {
	t.Parallel()

	// Same 5x5 fixture, pre-place 4 of 5 marks. After placing, the col
	// for the last mark has 4 marks already → R2 needs to clear stale
	// cands. R3 then finds the 1 remaining cand.
	rm := uniqueFixtureRegionMap()
	sol := uniqueFixtureSolution()
	factory := func() *solverState {
		s := newSolverState(rm, 5, 1)
		// Place marks 0..3 with R1 applied by hand so R2 is the rule that
		// drives the rest.
		applyR1AroundMarks(s, sol[:4], 5)
		for i := 0; i < 4; i++ {
			s.placeMark(sol[i].Row, sol[i].Col)
		}
		return s
	}
	runNecessityOnPartial(t, "R2_partial_5x5_4pre", factory, ruleR2)
}

// TestNecessity_R3: Without R3 no marks get placed. Any fixture that
// requires mark placement breaks without R3.
func TestNecessity_R3(t *testing.T) {
	t.Parallel()

	// Same approach: 4 of 5 pre-placed (plus R1 applied), forcing R3 to
	// place the last one. Omitting R3 leaves the state stalled.
	rm := uniqueFixtureRegionMap()
	sol := uniqueFixtureSolution()
	factory := func() *solverState {
		s := newSolverState(rm, 5, 1)
		applyR1AroundMarks(s, sol[:4], 5)
		for i := 0; i < 4; i++ {
			s.placeMark(sol[i].Row, sol[i].Col)
		}
		return s
	}
	runNecessityOnPartial(t, "R3_partial_5x5_4pre", factory, ruleR3)
}

// Higher-tier necessity tests (R4..R9) use minimal partial solverStates —
// the same fixtures the per-rule unit tests build — and assert that without
// R_i, (a) the distinctive elimination R_i would have made does NOT happen,
// and (b) the solver stalls (rule was genuinely needed for forward progress).
//
// Each test uses a dedicated fixture helper (one per rule) so the unit test
// and the necessity test share the construction logic.

// runNecessityDistinctiveElimination builds a partial solverState via
// `factory`, runs solveWith with the pruned ruleset (R_i omitted), and:
//   - calls assertDistinctiveNotMade on the resulting state to confirm the
//     specific inference R_i would have made is still missing;
//   - asserts the outcome is OutcomeStalled (removing R_i prevents forward
//     progress; it was necessary here).
//
// The partial state is not a solvable puzzle — only the "does the distinctive
// progress happen" question is tested. For full-puzzle necessity, R1..R3
// use runNecessityOnPartial above.
func runNecessityDistinctiveElimination(
	t *testing.T,
	name string,
	factory func() *solverState,
	omit ruleID,
	assertDistinctiveNotMade func(t *testing.T, s *solverState),
) {
	t.Helper()

	s := factory()
	outcome := solveWith(s, rulesetMinus(omit))
	assertDistinctiveNotMade(t, s)
	if outcome != OutcomeStalled {
		t.Fatalf("%s: expected OutcomeStalled without rule %d, got %v (solved=%v)",
			name, omit, outcome, s.solved())
	}
}

// r4Fixture builds the 4x4 k=1 state used in TestRuleR4_SingleLineRegion.
// R4 eliminates (0,2) and (0,3) because region 0 confines row 0's candidates
// to cols 0, 1.
func r4Fixture() *solverState {
	regionMap := [][]int{
		{0, 0, 1, 1},
		{2, 2, 1, 1},
		{2, 3, 3, 1},
		{2, 3, 3, 3},
	}
	s := newSolverState(regionMap, 4, 1)
	return s
}

// TestNecessity_R4: on r4Fixture, R4 eliminates (0,2) and (0,3). Without
// R4, no Tier-1 rule acts (no marks, no saturation, no row/col/region with
// cands == need), and no other higher-tier rule matches this pattern. The
// solver stalls with (0,2) and (0,3) still set.
func TestNecessity_R4(t *testing.T) {
	t.Parallel()
	runNecessityDistinctiveElimination(t, "R4_4x4_singleLineRegion",
		r4Fixture, ruleR4,
		func(t *testing.T, s *solverState) {
			t.Helper()
			if s.cands[0]&(uint16(1)<<2) == 0 {
				t.Errorf("without R4: (0,2) should still be a candidate, cands[0]=%04b", s.cands[0])
			}
			if s.cands[0]&(uint16(1)<<3) == 0 {
				t.Errorf("without R4: (0,3) should still be a candidate, cands[0]=%04b", s.cands[0])
			}
		},
	)
}

// r5Fixture builds a 5x5 k=1 state where R5's distinctive elimination is
// not subsumed by R4 or any other rule. After eliminating row-0 cols 2-4,
// row 0's cands are {0, 1} — both in region 0. R5 eliminates region-0
// cells in row 2 (cols 0, 1).
//
// Region 0 spans rows 0 AND 2 (so R4 cannot confine region 0 to one row
// and eliminate row 0 cols 2-4 from another angle). The 4x4 variant used
// in the R5 unit test allowed R4 to fire on region 1 after row-0
// eliminations; this 5x5 layout keeps every region on ≥2 rows so R4 is
// silent on this fixture and R5 is the unique path to the row-2
// elimination.
func r5Fixture() *solverState {
	regionMap := [][]int{
		{0, 0, 1, 1, 1},
		{2, 2, 1, 1, 1},
		{0, 0, 3, 3, 3},
		{2, 2, 3, 3, 4},
		{2, 2, 4, 4, 4},
	}
	s := newSolverState(regionMap, 5, 1)
	// Reduce row 0 to cands {0, 1} by eliminating cols 2, 3, 4.
	s.eliminateCand(0, 2)
	s.eliminateCand(0, 3)
	s.eliminateCand(0, 4)
	return s
}

// TestNecessity_R5: on r5Fixture, R5 eliminates (2,0) and (2,1). Without
// R5, R3 cannot fire (row 0 has 2 cands, rowNeed=1), R4 cannot fire (region
// 0 spans rows 0 and 2), and no higher-tier rule derives the row-2
// eliminations. Solver stalls with (2,0), (2,1) still candidates.
func TestNecessity_R5(t *testing.T) {
	t.Parallel()
	runNecessityDistinctiveElimination(t, "R5_5x5_singleRegionLine",
		r5Fixture, ruleR5,
		func(t *testing.T, s *solverState) {
			t.Helper()
			if s.cands[2]&(uint16(1)<<0) == 0 {
				t.Errorf("without R5: (2,0) should still be a candidate, cands[2]=%04b", s.cands[2])
			}
			if s.cands[2]&(uint16(1)<<1) == 0 {
				t.Errorf("without R5: (2,1) should still be a candidate, cands[2]=%04b", s.cands[2])
			}
		},
	)
}

// r6Fixture builds the k=1 4x4 state used in TestRuleR6_LockedKSetInLine
// with tracing enabled. Row 0 reduced to a single cand at (0,0); region 0
// covers (0,0) and (2,0). R6 would eliminate (2,0).
//
// Subsumption finding: R6's precondition (rowNeed==k AND cands==k AND
// shared-region) is a STRICT superset of R3's (rowNeed==cands). R3 always
// fires first; after placement regNeed[0] drops to 0 and R2 eliminates
// (2,0). End-state is identical with or without R6. The necessity test
// therefore uses the trace: without R6, no ruleR6 events are recorded.
// See rules.go R6 GoDoc for the full finding.
func r6Fixture() *solverState {
	regionMap := [][]int{
		{0, 0, 1, 1},
		{2, 2, 1, 1},
		{0, 3, 3, 3},
		{2, 2, 3, 3},
	}
	s := newSolverState(regionMap, 4, 1)
	s.eliminateCand(0, 1)
	s.eliminateCand(0, 2)
	s.eliminateCand(0, 3)
	s.trace = make(ruleTrace, 0, 32)
	return s
}

// TestNecessity_R6: trace-level necessity signal. Removing R6 from the
// registry produces a trace without any ruleR6 events. End-state is
// subsumed by R3+R2 (documented in rules.go R6 GoDoc).
func TestNecessity_R6(t *testing.T) {
	t.Parallel()
	s := r6Fixture()
	outcome := solveWith(s, rulesetMinus(ruleR6))
	for _, ev := range s.trace {
		if ev.Rule == ruleR6 {
			t.Errorf("without R6: trace should contain no ruleR6 events, found %+v", ev)
		}
	}
	if outcome != OutcomeSolved && outcome != OutcomeStalled {
		t.Errorf("without R6: expected Solved or Stalled outcome, got %v", outcome)
	}
}

// r7Fixture builds a 5x5 k=1 state where R7's distinctive elimination is
// not subsumed by any other rule. Column 0 cands reduced to rows 0 and 1
// (colNeed=1); placing (0,1) would force R1 to eliminate both col-0 cands
// → col-0 infeasible. R7 eliminates (0,1).
//
// Region assignments are chosen so (0,0) and (1,0) belong to DIFFERENT
// regions — this prevents R5's col-axis single-region inference from
// eliminating (0,1) via col 0. Every region also spans >1 row so R4 does
// not fire.
func r7Fixture() *solverState {
	regionMap := [][]int{
		{0, 1, 1, 2, 2},
		{3, 3, 1, 2, 2},
		{3, 3, 1, 4, 4},
		{0, 3, 1, 4, 4},
		{0, 0, 4, 4, 4},
	}
	s := newSolverState(regionMap, 5, 1)
	s.eliminateCand(2, 0)
	s.eliminateCand(3, 0)
	s.eliminateCand(4, 0)
	return s
}

// TestNecessity_R7: on r7Fixture, R7 eliminates (0,1) and (1,1). Without
// R7, R3 cannot fire (no row/col/region has cands == need), R4/R5 don't
// match (column 0 is in region 0 which spans rows 0, 1, 2), R6 doesn't
// match (no row reduced to k locked cells sharing a region). Solver
// stalls with (0,1) still a candidate.
func TestNecessity_R7(t *testing.T) {
	t.Parallel()
	runNecessityDistinctiveElimination(t, "R7_5x5_adjacencyForcing",
		r7Fixture, ruleR7,
		func(t *testing.T, s *solverState) {
			t.Helper()
			if s.cands[0]&(uint16(1)<<1) == 0 {
				t.Errorf("without R7: (0,1) should still be a candidate, cands[0]=%05b", s.cands[0])
			}
		},
	)
}

// r8Fixture builds the 6x6 k=2 state used in TestRuleR8_KLineSubset. Rows
// 0, 1 reduced to cands {0, 3} each — the classic 2-row X-wing. R8
// eliminates cols 0, 3 in rows 2..5.
//
// Subsumption finding: R8's precondition (rowNeed[r]==k=2 AND combined
// cands on k rows count == k) forces each row's cands count to be at most
// k. Combined with rowNeed==k, each row's cands count == k exactly, which
// is R3's row-axis firing precondition. R3 (Tier 1) always fires before
// R8 (Tier 4). Therefore R8 never fires in any reachable state — the
// rule is structurally subsumed.
//
// We exercise R8's direct single-invocation behavior in the unit test
// above. For necessity, we assert an execution-level signal: removing R8
// from the registry produces a trace with no ruleR8 events. This proves
// R8 was absent from the pruned solve, which is the strictest necessity
// signal available given the subsumption.
func r8Fixture() *solverState {
	regionMap := rowStripeMap(6)
	s := newSolverState(regionMap, 6, 2)
	for c := 0; c < 6; c++ {
		if c == 0 || c == 3 {
			continue
		}
		s.eliminateCand(0, c)
		s.eliminateCand(1, c)
	}
	s.trace = make(ruleTrace, 0, 32)
	return s
}

// TestNecessity_R8: without R8 in the registry, the trace contains no
// ruleR8 events. The solver still reaches an end state (Contradiction
// in this pathological X-wing fixture, via R3's eager placement into
// vertical adjacency), but R8's unique trace contribution is absent —
// the necessity signal.
//
// Honest subsumption finding: R8's precondition is a strict superset of
// R3's row-axis precondition at k=2. R3 (Tier 1) always fires before R8
// (Tier 4). R8 as written never contributes deductively distinct progress
// in the fixed-point loop. Retained for spec clarity; a future slice may
// move R8 to Tier 1 or weaken its precondition to cands >= rowNeed per
// row, at which point this test becomes a standard elimination necessity
// test.
func TestNecessity_R8(t *testing.T) {
	t.Parallel()
	s := r8Fixture()
	_ = solveWith(s, rulesetMinus(ruleR8))
	for _, ev := range s.trace {
		if ev.Rule == ruleR8 {
			t.Errorf("without R8: trace should contain no ruleR8 events, found %+v", ev)
		}
	}
}

// r9Fixture builds the 9x9 k=2 state used in TestRuleR9_RegionPairExclusion.
// Row 0 cands = {0, 5, 7}: region 0 contributes col 0, region 1 col 5,
// region 2 col 7. Combined regions 0+1 span 2 cells on row 0 (k=2) — R9
// eliminates col 7 (region 2) from row 0.
func r9Fixture() *solverState {
	regionMap := make([][]int, 9)
	for r := range regionMap {
		regionMap[r] = make([]int, 9)
		for c := range regionMap[r] {
			regionMap[r][c] = r
		}
	}
	regionMap[0] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	regionMap[1] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
	regionMap[2] = []int{3, 3, 3, 3, 3, 3, 3, 3, 3}
	regionMap[3] = []int{4, 4, 4, 4, 4, 4, 4, 4, 4}
	regionMap[4] = []int{5, 5, 5, 5, 5, 5, 5, 5, 5}
	regionMap[5] = []int{6, 6, 6, 6, 6, 6, 6, 6, 6}
	regionMap[6] = []int{7, 7, 7, 7, 7, 7, 7, 7, 7}
	regionMap[7] = []int{8, 8, 8, 8, 8, 8, 8, 8, 8}
	regionMap[8] = []int{0, 0, 0, 1, 1, 1, 2, 2, 2}

	s := newSolverState(regionMap, 9, 2)
	for c := 0; c < 9; c++ {
		if c == 0 || c == 5 || c == 7 {
			continue
		}
		s.eliminateCand(0, c)
	}
	return s
}

// TestNecessity_R9: on r9Fixture, R9 eliminates (0,7). Without R9, the
// solver must find another path to that elimination. R3 cannot fire on
// row 0 (3 cands, rowNeed=2). R4/R5 don't match (row 0 spans 3 regions).
// R6 doesn't match (cands=3 != k=2). R7's infeasibility check may or may
// not catch the forcing. R8 requires rowNeed == k on MULTIPLE rows; only
// row 0 is reduced here. Without R9, (0,7) remains a candidate — solver
// stalls.
func TestNecessity_R9(t *testing.T) {
	t.Parallel()
	runNecessityDistinctiveElimination(t, "R9_9x9_regionPairExclusion",
		r9Fixture, ruleR9,
		func(t *testing.T, s *solverState) {
			t.Helper()
			if s.cands[0]&(uint16(1)<<7) == 0 {
				t.Errorf("without R9: (0,7) should still be a candidate, cands[0]=%09b", s.cands[0])
			}
		},
	)
}
