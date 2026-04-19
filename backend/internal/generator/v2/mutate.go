package generator

import "math/bits"

// mutationOutcome is the three-way result of a single grow+solve+mutate run.
// The orchestrator uses it to decide whether to accept the attempt, retry
// with a fresh sample, or escalate.
type mutationOutcome int

const (
	// mutationSolved means the deductive solver reached a full solution
	// after the (possibly zero) swaps the mutator applied. The caller
	// should then run the brute-uniqueness check.
	mutationSolved mutationOutcome = iota
	// mutationFailed means either the solver hit a contradiction, or the
	// mutator ran out of its budget (K swaps) without reaching Solved.
	// Orchestrator restarts from the sampler.
	mutationFailed
)

// solveAndMutate runs the deductive solver on the current region map.
// If the solver stalls, it tries boundary-swap mutations — each swap is
// accepted iff, after re-running the solver, the deductive-solved cell
// count strictly increases. Loops up to g.cfg.maxMutations.
//
// Invariants preserved on every accepted swap:
//   - Both affected regions remain 4-connected.
//   - Both regions keep all their seed marks.
//   - No region drops to zero cells.
//
// On success (mutationSolved), g.solver holds the fully solved state —
// caller can read its solution marks via appendSolutionMarks.
//
// Trace recording is DISABLED during probe solves (mutator is the hot
// path). The caller runs one final trace-enabled solve for classification.
func (g *Generator) solveAndMutate(seeds [][]Mark) mutationOutcome {
	// Prepare region map in the solver-friendly shape: solver needs a
	// [][]int slice. Build once, mutate in place across swaps.
	rm := convertRegionsToSlices(&g.regionOf, g.n)

	g.solver.trace = nil // no recording during probe solves
	if err := g.solver.initFromRegionMap(rm, g.n, g.k); err != nil {
		return mutationFailed
	}

	outcome := solve(&g.solver)
	switch outcome {
	case OutcomeSolved:
		return mutationSolved
	case OutcomeContradiction:
		return mutationFailed
	}

	// OutcomeStalled — attempt boundary swaps.
	budget := g.cfg.maxMutations
	if budget <= 0 {
		budget = defaultMaxMutations
	}

	for step := 0; step < budget; step++ {
		baseline := countSolvedCells(&g.solver)
		accepted := g.tryOneSwap(seeds, rm, baseline)
		if !accepted {
			// No swap improves — give up.
			return mutationFailed
		}
		// tryOneSwap installs g.solver with the re-solved state already.
		switch {
		case g.solver.solved():
			return mutationSolved
		case g.solver.contradicts():
			return mutationFailed
		}
	}

	// Budget exhausted without reaching Solved.
	return mutationFailed
}

// tryOneSwap scans boundary cells (cells on the border between two
// regions) and tries reassigning each to the neighboring region. Accepts
// the first swap that (a) preserves all region invariants and (b) strictly
// increases the solver-solved cell count. On accept: updates g.regionOf
// and rm in place and returns true with g.solver holding the post-swap
// solve. On miss: restores g.solver to the pre-scan state (the caller
// uses g.solver.solved() / .contradicts() after return).
//
// Scan prioritization: start from cells near a "stalled" cell (a cell
// that is neither a confirmed mark nor eliminated), since those are
// where the solver is stuck. Spec: "examine region boundaries within
// Manhattan distance 2" of stalled cells.
func (g *Generator) tryOneSwap(seeds [][]Mark, rm [][]int, baseline int) bool {
	n := g.n

	// Collect stalled cells (candidates that are NOT yet marked — i.e.
	// cells the solver has neither confirmed nor eliminated).
	var stalledCells [nMax * nMax]int // row*n + col
	stalledCount := 0
	for r := range n {
		cands := g.solver.cands[r]
		m := cands
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			stalledCells[stalledCount] = r*n + c
			stalledCount++
		}
	}

	// For each stalled cell, try swaps on cells within Manhattan 2. A
	// "visited" mask avoids re-trying the same (r, c).
	var tried [nMax][nMax]bool

	trySwap := func(r, c int) bool {
		if r < 0 || r >= n || c < 0 || c >= n {
			return false
		}
		if tried[r][c] {
			return false
		}
		tried[r][c] = true
		fromID := int(g.regionOf[r][c])
		if isSeedCell(seeds, fromID, Mark{Row: r, Col: c}) {
			return false
		}
		for _, d := range fourNeighborOffsets {
			nr, nc := r+d[0], c+d[1]
			if nr < 0 || nr >= n || nc < 0 || nc >= n {
				continue
			}
			toID := int(g.regionOf[nr][nc])
			if toID == fromID {
				continue
			}
			if !fromRegionConnectedWithoutCell(&g.regionOf, fromID, r, c, seeds, n) {
				continue
			}
			if countRegionCells(&g.regionOf, fromID, n) <= 1 {
				continue
			}

			g.regionOf[r][c] = int8(toID)
			rm[r][c] = toID

			g.solver.trace = nil
			if err := g.solver.initFromRegionMap(rm, n, g.k); err != nil {
				g.regionOf[r][c] = int8(fromID)
				rm[r][c] = fromID
				continue
			}
			_ = solve(&g.solver)
			newCount := countSolvedCells(&g.solver)
			if newCount > baseline {
				return true
			}
			g.regionOf[r][c] = int8(fromID)
			rm[r][c] = fromID
		}
		return false
	}

	// Phase 1: stalled-cell-guided scan (Manhattan <= 2).
	if stalledCount > 0 {
		for i := 0; i < stalledCount; i++ {
			sr, sc := stalledCells[i]/n, stalledCells[i]%n
			for dr := -2; dr <= 2; dr++ {
				for dc := -2; dc <= 2; dc++ {
					if abs(dr)+abs(dc) > 2 {
						continue
					}
					if trySwap(sr+dr, sc+dc) {
						return true
					}
				}
			}
		}
	}

	// Phase 2: global scan fallback (Phase 1 didn't find anything, and
	// the stalled-cell list may have been empty or too narrow).
	for r := range n {
		for c := range n {
			if trySwap(r, c) {
				return true
			}
		}
	}

	// No improving swap found; restore solver to the pre-scan state so
	// the caller's g.solver reads are trustworthy.
	g.solver.trace = nil
	if err := g.solver.initFromRegionMap(rm, n, g.k); err == nil {
		_ = solve(&g.solver)
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// fromRegionConnectedWithoutCell returns true iff, with cell (r, c) removed
// from region `fromID`, the rest of that region is still 4-connected AND
// still contains all its seed marks.
//
// Tried before the caller commits a swap. Uses BFS confined to the
// modified region.
func fromRegionConnectedWithoutCell(
	regionOf *[nMax][nMax]int8,
	fromID, r, c int,
	seeds [][]Mark,
	n int,
) bool {
	// Pick a start cell: the first seed of this region that is not the
	// cell being removed. Also reject if (r, c) is itself a seed — that's
	// the caller's responsibility to filter, but be defensive.
	var start Mark
	found := false
	for _, m := range seeds[fromID] {
		if m.Row == r && m.Col == c {
			return false
		}
		if !found {
			start = m
			found = true
		}
	}
	if !found {
		return false
	}

	var visited [nMax][nMax]bool
	bfsRegionVisit(regionOf, fromID, start.Row, start.Col, r, c, n, &visited)

	// Every seed must be visited (the removed cell (r, c) is already
	// excluded by the skip parameter passed to bfsRegionVisit).
	for _, m := range seeds[fromID] {
		if m.Row == r && m.Col == c {
			continue
		}
		if !visited[m.Row][m.Col] {
			return false
		}
	}
	return true
}

// isSeedCell reports whether (mark) is a seed cell for region gid.
func isSeedCell(seeds [][]Mark, gid int, mark Mark) bool {
	if gid < 0 || gid >= len(seeds) {
		return false
	}
	for _, m := range seeds[gid] {
		if m == mark {
			return true
		}
	}
	return false
}

// countRegionCells counts cells in the given region.
func countRegionCells(regionOf *[nMax][nMax]int8, gid, n int) int {
	count := 0
	for r := range n {
		for c := range n {
			if int(regionOf[r][c]) == gid {
				count++
			}
		}
	}
	return count
}

// countSolvedCells returns the number of cells the deductive solver has
// resolved (i.e., cells with a definite mark OR cells whose row/col/region
// have all their k marks placed). A simpler proxy: count (marks placed).
// Since eliminated candidates don't inherently mean "solved", and a cell
// being a mark is the clearest signal, we use the mark count.
//
// As a secondary signal we also count rows that have fully resolved
// (rowNeed == 0) — these contribute via their mark count already. This
// matches the spec's "strictly increases solved-cell count" phrasing,
// where "solved cell" = a cell whose state is definitively determined.
func countSolvedCells(s *solverState) int {
	// "Solved cells" = cells whose final state (mark vs. non-mark) the
	// deductive solver has determined. That's:
	//   - cells with marks (bits in marks[r]), AND
	//   - cells with no remaining candidates (bits in ~cands[r]) that are
	//     also not marks.
	// Historically we counted both. Mutator-acceptance rule "strictly
	// improves solved count" reads cleanest against this richer measure
	// because it credits eliminations (which the solver will eventually
	// chain into more placements).
	count := 0
	full := uint16(1)<<uint(s.n) - 1
	for r := range s.n {
		count += bits.OnesCount16(s.marks[r])
		solvedNonMark := full &^ s.cands[r] &^ s.marks[r]
		count += bits.OnesCount16(solvedNonMark)
	}
	return count
}
