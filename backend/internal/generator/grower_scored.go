package generator

import "math/bits"

// regionMinSize is the per-region floor for generated puzzles (R-067b).
// Every region in a grown tile must have at least this many cells at both
// k=1 and k=2. The ceiling is deliberately unbounded — curation rejects
// overly-large regions out-of-band (see GAME_DESIGN.md "Planned Work").
const regionMinSize = 3

// buildFrontier fills g.growFrontierBuf with cells eligible for assignment
// this step and reports whether the min-size rule is currently active.
//
// When any region has fewer than regionMinSize cells, the frontier is
// restricted to cells adjacent to at least one such region — the caller's
// pick is then forced to grow an under-size region. Once every region has
// reached regionMinSize the restriction drops and every unclaimed
// frontier cell is eligible.
//
// Returns ok=false if the resulting buffer is empty. In the constrained
// case that means bridging trapped every under-size region (rare) and the
// caller should fail the attempt so the orchestrator can resample.
//
// The `constrained` flag is computed in the same region-size scan that
// drives the frontier restriction, so callers do not need a separate
// anyUndersized pass.
func (g *Generator) buildFrontier(gs *growState) (ok, constrained bool) {
	n := g.n
	for gid := range n {
		if gs.regionSize[gid] < regionMinSize {
			constrained = true
			break
		}
	}

	g.growFrontierBuf = g.growFrontierBuf[:0]
	for r := range n {
		var union uint16
		for gid := range n {
			if constrained && gs.regionSize[gid] >= regionMinSize {
				continue
			}
			union |= gs.regionFrontierRow[gid][r]
		}
		union &^= gs.claimedRow[r]
		m := union
		for m != 0 {
			c := bits.TrailingZeros16(m)
			m &^= 1 << c
			g.growFrontierBuf = append(g.growFrontierBuf, r*n+c)
		}
	}
	return len(g.growFrontierBuf) > 0, constrained
}

// pickSmallestUndersized returns the under-size region with the fewest
// cells that has (cr, cc) in its frontier. Size ties are reservoir-
// sampled via g.rng so no gid is systematically favored.
//
// Returns -1 only as a defense against a precondition violation —
// buildFrontier, in the constrained branch, only emits cells that at
// least one under-size region can take. The caller keeps the -1 check
// so a future refactor cannot silently break the invariant.
func (g *Generator) pickSmallestUndersized(gs *growState, cr, cc int) int {
	cellBit := uint16(1) << uint(cc)
	chosen := -1
	smallest := g.n + 1
	tieCount := 0
	for gid := range g.n {
		if gs.regionSize[gid] >= regionMinSize {
			continue
		}
		if gs.regionFrontierRow[gid][cr]&cellBit == 0 {
			continue
		}
		sz := gs.regionSize[gid]
		switch {
		case sz < smallest:
			smallest = sz
			chosen = gid
			tieCount = 1
		case sz == smallest:
			tieCount++
			if g.rng.IntN(tieCount) == 0 {
				chosen = gid
			}
		}
	}
	return chosen
}

// growRegionsSolverGuided is the R-066 expensive variant of the region
// grower (input-spec.md §4.3 Step B / PG-12). It differs from the cheap
// variant only in how a frontier cell's candidate region is chosen: the
// cheap variant picks by inverse-size weight; the solver-guided variant
// evaluates each candidate by tentatively assigning the cell, completing
// the tiling with the cheap loop, running the deductive solver, and
// scoring by solved-cell count.
//
// Setup (seeds, bridges, initial frontiers) is identical to the cheap
// variant and reuses initGrowState. The frontier walk also reuses the
// cheap loop's bitmask union trick.
//
// Efficiency contract (critical; called out in PG-12 and review-local
// R-065 finding MAJOR #1):
//   - Solver state is cloned via *dst = *src — no initFromRegionMap on
//     each probe.
//   - grow-state scratch is a fixed-size struct copied the same way.
//   - Trace recording is OFF during scoring (NF3: zero alloc in the hot
//     loop). The orchestrator re-enables it for the final classification
//     pass on g.solver.
//
// When a frontier cell has only one candidate region, the probe is
// skipped entirely — there is nothing to choose. This short-circuit is
// the main reason the per-attempt penalty stays bounded.
//
// Returns true on successful tiling, false on any structural failure
// (same contract as growRegions).
func (g *Generator) growRegionsSolverGuided(seeds [][]Mark, dst *[nMax][nMax]int8) bool {
	n := g.n

	if !g.initGrowState(seeds, &g.scoringGrow) {
		return false
	}
	// Authoritative (mutable) growth state for this call. Kept as a local
	// value struct so we can snapshot it with value copy into scoringGrow
	// during probes. Start FROM the scratch so setup is not duplicated.
	var gs = g.scoringGrow

	// Reuse the pre-allocated frontier buffer on the Generator. growCheapLoopOn
	// uses the same buffer when invoked from probeAssignment — that's safe
	// because probeAssignment calls growCheapLoopOn serially, not nested.
	g.growFrontierBuf = g.growFrontierBuf[:0]
	// candidates holds the region ids whose frontier contains the current
	// cell. Sized for worst case (every region has the cell as frontier).
	var candidates [nMax]int

	for gs.remaining > 0 {
		ok, constrained := g.buildFrontier(&gs)
		if !ok {
			return false
		}

		// Pick a frontier cell. We use the RNG here (matching the cheap
		// variant's stream) so that WithSeed stays meaningful.
		pickIdx := g.rng.IntN(len(g.growFrontierBuf))
		cell := g.growFrontierBuf[pickIdx]
		cr, cc := cell/n, cell%n
		cellBit := uint16(1) << uint(cc)

		// Under the min-size rule the candidate list is restricted to
		// under-size regions. buildFrontier's invariant guarantees at
		// least one such region is present, so count == 0 below is a
		// true failure, not a "rule filtered everything out".
		count := 0
		for gid := range n {
			if gs.regionFrontierRow[gid][cr]&cellBit == 0 {
				continue
			}
			if constrained && gs.regionSize[gid] >= regionMinSize {
				continue
			}
			candidates[count] = gid
			count++
		}
		if count == 0 {
			return false
		}

		chosen := candidates[0]
		if count == 1 {
			// Single candidate — skip probe entirely.
			commitCell(&gs, n, cr, cc, chosen)
			continue
		}

		// Multi-candidate: score each via tentative-completion + solve.
		bestScore := -1
		tieCount := 0
		// Probe each candidate in a fresh snapshot of gs. The snapshot
		// itself is ~1 KiB so *dst = *src is fine.
		//
		// Note: the cheap completion inside the probe consumes g.rng.
		// We do not bother restoring the RNG between probes — the
		// scoring differences are what matter, not the specific
		// completions. The outer grow determinism is preserved by the
		// outer frontier pickIdx above; fixed seed still produces the
		// same result on repeated runs because all g.rng calls happen
		// in a deterministic order.
		for i := 0; i < count; i++ {
			score := g.probeAssignment(&gs, cr, cc, candidates[i])
			if score > bestScore {
				bestScore = score
				chosen = candidates[i]
				tieCount = 1
			} else if score == bestScore {
				// Reservoir-sample tie-breaking so the winner is
				// uniform over equally-scored candidates.
				tieCount++
				if g.rng.IntN(tieCount) == 0 {
					chosen = candidates[i]
				}
			}
		}

		commitCell(&gs, n, cr, cc, chosen)
	}

	copyRegionOf(dst, &gs.regionOf)
	return true
}

// probeAssignment scores a tentative assignment of (cr, cc) to region
// gid. Returns the deductive solver's solved-cell count after a cheap
// completion of the remaining grid.
//
// Heavy-handed but correct: the only state clones are the grow-state
// snapshot and the solverState value copy (both fixed-size, <10 KiB
// total). `initFromRegionMap` is called once per probe — on a cheap
// completion we built in-place in scoringGrow.
func (g *Generator) probeAssignment(gs *growState, cr, cc, gid int) int {
	// Clone the current grow state into scoringGrow scratch.
	g.scoringGrow = *gs

	// Commit the tentative cell.
	commitCell(&g.scoringGrow, g.n, cr, cc, gid)

	// Finish the tiling cheaply on the scratch state. Failure here means
	// the scratch grow got stuck — the candidate is unworkable, score 0.
	if g.scoringGrow.remaining > 0 {
		if !g.growCheapLoopOn(&g.scoringGrow) {
			return 0
		}
	}

	// Initialize the scoring solver directly from the scratch regionOf
	// array. initFromRegionOf is semantically identical to
	// initFromRegionMap but skips the per-probe [][]int allocation that
	// convertRegionsToSlices would produce. At N=12 k=1 with 120-180
	// probes per attempt this removes ~20 KB of GC pressure per attempt.
	g.scoringSolver.trace = nil
	if err := g.scoringSolver.initFromRegionOf(&g.scoringGrow.regionOf, g.n, g.k); err != nil {
		return 0
	}

	// Solve deductively; score by solved-cell count.
	_ = solve(&g.scoringSolver)
	return countSolvedCells(&g.scoringSolver)
}

// growCheapLoopOn is the cheap-loop body operating on an arbitrary
// grow state (not the Generator's default state). The only difference
// from growCheapLoop is the target argument — extracted so the
// solver-guided probe can complete its scratch state without touching
// the authoritative one.
func (g *Generator) growCheapLoopOn(gs *growState) bool {
	n := g.n
	type weighted struct {
		gid    int
		weight int
	}
	var regionBuf [nMax]weighted

	// Reuse the Generator's pre-allocated frontier buffer. Safe under nested
	// use (growRegionsSolverGuided -> probeAssignment -> growCheapLoopOn)
	// because the outer caller resets [:0] at each iteration top and does
	// not re-read the buffer after calling into the inner scope.
	for gs.remaining > 0 {
		ok, constrained := g.buildFrontier(gs)
		if !ok {
			return false
		}

		pickIdx := g.rng.IntN(len(g.growFrontierBuf))
		cell := g.growFrontierBuf[pickIdx]
		cr, cc := cell/n, cell%n
		cellBit := uint16(1) << uint(cc)

		if constrained {
			chosen := g.pickSmallestUndersized(gs, cr, cc)
			if chosen < 0 {
				return false
			}
			commitCell(gs, n, cr, cc, chosen)
			continue
		}

		totalWeight := 0
		count := 0
		for gid := range n {
			if gs.regionFrontierRow[gid][cr]&cellBit == 0 {
				continue
			}
			w := n - gs.regionSize[gid]
			if w < 1 {
				w = 1
			}
			regionBuf[count] = weighted{gid: gid, weight: w}
			totalWeight += w
			count++
		}
		if count == 0 {
			return false
		}

		pick := g.rng.IntN(totalWeight)
		chosen := regionBuf[count-1].gid
		acc := 0
		for i := 0; i < count; i++ {
			acc += regionBuf[i].weight
			if pick < acc {
				chosen = regionBuf[i].gid
				break
			}
		}

		commitCell(gs, n, cr, cc, chosen)
	}
	return true
}
