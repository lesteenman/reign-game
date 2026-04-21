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
	g.solver.trace = nil // no recording during probe solves
	if err := g.solver.initFromRegionOf(&g.regionOf, g.n, g.k); err != nil {
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
	budget := g.budgetForCurrent()

	// Two-pass acceptance per step:
	//   (1) strict improvement — any swap that raises solved-cell count
	//   (2) graded plateau — same-score with p=0.1 OR one-cell regression
	//       with p=0.05. The walker stays greedy because plateau-heavy
	//       acceptance diffuses through the basin instead of climbing
	//       toward solved, especially under the R-067b min-size rule
	//       which trims the boundary swaps available per step.
	for step := 0; step < budget; step++ {
		baseline := countSolvedCells(&g.solver)
		accepted := g.tryOneSwap(seeds, baseline, false)
		if !accepted {
			accepted = g.tryOneSwap(seeds, baseline, true)
		}
		if !accepted {
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

	return mutationFailed
}

// budgetForCurrent picks the mutation budget for this Generator's (n, k).
// An explicit WithMaxMutations call wins outright. Otherwise k=2 uses the
// smaller defaultMaxMutationsK2 because each probe-solve at k=2 is slower
// and hard cases otherwise push end-to-end wall time past the 2 s ceiling.
// k=1 uses the bigger defaultMaxMutations so the walker has room to
// oscillate past local optima at N=12 k=1, where the Step 7 gate lives.
func (g *Generator) budgetForCurrent() int {
	if g.cfg.maxMutations > 0 {
		return g.cfg.maxMutations
	}
	if g.k == 2 {
		return defaultMaxMutationsK2
	}
	return defaultMaxMutations
}

// tryOneSwap scans boundary cells (cells on the border between two
// regions) and tries reassigning each to the neighboring region. The
// acceptance rule depends on allowPlateau:
//   - allowPlateau=false (strict): accept iff solved-cell count strictly
//     increases.
//   - allowPlateau=true (weighted plateau): accept a strict improvement
//     immediately, OR accept a same-score swap with p=0.1, OR accept a
//     one-cell regression with p=0.05. The second bucket gives the walker
//     a way off a plateau; the third gives it a way over a low ridge into
//     a neighboring basin. Deeper regressions stay rejected so the walk
//     does not unravel past-earned progress. Under R-067b's min-size rule
//     the boundary-swap surface is tighter, so the acceptance rates are
//     kept low — a heavier plateau bucket diffuses the walker through
//     the basin instead of climbing toward solved.
//
// On accept: updates g.regionOf in place and returns true with g.solver
// holding the post-swap solve. On miss: restores g.solver to the pre-scan
// state (the caller uses g.solver.solved() / .contradicts() after return).
//
// Scan prioritization: start from cells near a "stalled" cell (a cell
// that is neither a confirmed mark nor eliminated), since those are
// where the solver is stuck. Spec: "examine region boundaries within
// Manhattan distance 2" of stalled cells.
func (g *Generator) tryOneSwap(seeds [][]Mark, baseline int, allowPlateau bool) bool {
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
			// Honor the R-067b min-size rule. The grower refuses to leave
			// any region under regionMinSize; the mutator must not undo
			// that by draining cells below the floor.
			if countRegionCells(&g.regionOf, fromID, n) <= regionMinSize {
				continue
			}

			g.regionOf[r][c] = int8(toID)

			g.solver.trace = nil
			if err := g.solver.initFromRegionOf(&g.regionOf, n, g.k); err != nil {
				g.regionOf[r][c] = int8(fromID)
				continue
			}
			_ = solve(&g.solver)
			newCount := countSolvedCells(&g.solver)
			if newCount > baseline {
				return true
			}
			if allowPlateau {
				switch newCount - baseline {
				case 0:
					if g.rng.IntN(10) == 0 {
						return true
					}
				case -1:
					if g.rng.IntN(20) == 0 {
						return true
					}
				}
			}
			g.regionOf[r][c] = int8(fromID)
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
	if err := g.solver.initFromRegionOf(&g.regionOf, n, g.k); err == nil {
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
// from region `fromID`, every remaining cell of that region is still
// reachable via 4-adjacent cells of the same region (and all seed marks
// are among the reachable cells).
//
// A weaker version of this check that only verified seed reachability was
// the source of R-067-era disconnected regions: at k=1 a region has a
// single seed, so the seed trivially remains its own connected component
// while non-seed tail cells get orphaned. We now count cells too.
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

	// Count cells in fromID before the removal so we can compare against
	// the BFS-reachable count after skipping (r, c).
	totalCells := 0
	for rr := range n {
		for cc := range n {
			if int(regionOf[rr][cc]) == fromID {
				totalCells++
			}
		}
	}

	var visited [nMax][nMax]bool
	reached := bfsRegionVisit(regionOf, fromID, start.Row, start.Col, r, c, n, &visited)

	// The post-removal region must have (totalCells - 1) reachable cells.
	// If BFS reaches fewer, some non-seed cells are now orphaned.
	if reached != totalCells-1 {
		return false
	}

	// Also confirm every seed is visited — defensive; the cell-count
	// check implies it, but keep for symmetry with the previous contract.
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
