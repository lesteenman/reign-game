package generator

// growState is the mutable per-grow-attempt state shared between the cheap
// and solver-guided grower variants. It's big enough (~1 KiB) that snapshot
// via *dst = *src is cheap — which is how the solver-guided variant probes
// alternative assignments without re-doing setup.
type growState struct {
	// regionOf mirrors the caller's dst. -1 means unclaimed during growth.
	regionOf [nMax][nMax]int8
	// regionSize[g] = current cell count for region g.
	regionSize [nMax]int
	// regionFrontierRow[g][r] is a uint16 column bitmask: bit c set means
	// cell (r, c) is an unclaimed 4-neighbor of region g.
	regionFrontierRow [nMax][nMax]uint16
	// claimedRow[r] has a bit set for each claimed column in row r.
	claimedRow [nMax]uint16
	// remaining is the count of still-unclaimed cells in [0..n)x[0..n).
	remaining int
}

// growRegions tiles the grid with N 4-connected regions grown out of the
// given seed groups. Implements the cheap variant from input-spec.md §4.3
// Step B: random-weighted frontier growth with inverse-size weighting so
// regions balance their final sizes around N (on average).
//
// Contract:
//
//   - len(seeds) must equal g.n; each inner slice has length g.k.
//   - On return with true, every cell in dst[0..n-1][0..n-1] holds a
//     region ID in [0, n). Cells outside the n×n subgrid are cleared to 0
//     (caller treats the array as [n][n] regardless).
//   - Each region is 4-connected and contains its k seed marks.
//
// Algorithm (cell-level, k-agnostic):
//  1. Write seeds into dst; initialize per-region sizes from their seed
//     counts; mark every seed's 4-neighbor unclaimed cells as frontier
//     candidates.
//  2. Loop: pick an unclaimed cell from the frontier uniformly at random.
//     Among regions whose frontier contains it, pick one weighted by
//     (N - currentSize) (favors smaller regions).
//  3. Assign the cell, extend that region's frontier with the cell's
//     unclaimed 4-neighbors, update the shared unclaimed frontier.
//  4. Terminate when every cell is claimed.
//
// Returns false if the algorithm can't make progress (shouldn't happen for a
// connected grid + valid seeds; returning false rather than panicking lets
// the orchestrator discard the attempt cleanly).
//
// The function uses g.rng for all randomness — fixed seeds produce
// deterministic grows.
func (g *Generator) growRegions(seeds [][]Mark, dst *[nMax][nMax]int8) bool {
	var gs growState
	if !g.initGrowState(seeds, &gs) {
		return false
	}
	if !g.growCheapLoopOn(&gs) {
		return false
	}
	copyRegionOf(dst, &gs.regionOf)
	return true
}

// initGrowState seeds the grow-state struct: seats the seed marks,
// bridges k>=2 pairs with L-paths, seeds initial frontiers, and computes
// `remaining`. Returns false on any structural violation (bad seeds,
// seed collision, bridging leaving a pair disconnected).
func (g *Generator) initGrowState(seeds [][]Mark, gs *growState) bool {
	n := g.n
	if len(seeds) != n {
		return false
	}

	for r := range nMax {
		for c := range nMax {
			gs.regionOf[r][c] = 0
		}
		gs.claimedRow[r] = 0
		gs.regionSize[r] = 0
		for gid := range nMax {
			gs.regionFrontierRow[gid][r] = 0
		}
	}
	for r := range n {
		for c := range n {
			gs.regionOf[r][c] = -1
		}
	}

	for gid, group := range seeds {
		if len(group) != g.k {
			return false
		}
		for _, m := range group {
			if m.Row < 0 || m.Row >= n || m.Col < 0 || m.Col >= n {
				return false
			}
			if gs.regionOf[m.Row][m.Col] != -1 {
				return false
			}
			gs.regionOf[m.Row][m.Col] = int8(gid)
			gs.claimedRow[m.Row] |= 1 << uint(m.Col)
			gs.regionSize[gid]++
		}
	}

	if g.k >= 2 {
		for gid, group := range seeds {
			for i := 1; i < len(group); i++ {
				bridgeCells(&gs.regionOf, &gs.claimedRow, &gs.regionSize, group[i-1], group[i], gid)
			}
		}
		for gid, group := range seeds {
			if !seedComponentsConnected(&gs.regionOf, group, gid, n) {
				return false
			}
		}
	}

	for r := range n {
		for c := range n {
			rid := gs.regionOf[r][c]
			if rid < 0 {
				continue
			}
			for _, d := range fourNeighborOffsets {
				nr, nc := r+d[0], c+d[1]
				if nr < 0 || nr >= n || nc < 0 || nc >= n {
					continue
				}
				if gs.regionOf[nr][nc] != -1 {
					continue
				}
				gs.regionFrontierRow[rid][nr] |= 1 << uint(nc)
			}
		}
	}

	gs.remaining = n * n
	for gid := range n {
		gs.remaining -= gs.regionSize[gid]
	}
	return true
}

// commitCell assigns (cr, cc) to region `chosen`, updates frontiers and
// counters. Factored out so cheap and solver-guided variants share the
// same bookkeeping.
func commitCell(gs *growState, n, cr, cc, chosen int) {
	cellBit := uint16(1) << uint(cc)
	gs.regionOf[cr][cc] = int8(chosen)
	gs.claimedRow[cr] |= cellBit
	gs.regionSize[chosen]++
	gs.remaining--

	for gid := range n {
		gs.regionFrontierRow[gid][cr] &^= cellBit
	}
	for _, d := range fourNeighborOffsets {
		nr, nc := cr+d[0], cc+d[1]
		if nr < 0 || nr >= n || nc < 0 || nc >= n {
			continue
		}
		if gs.regionOf[nr][nc] != -1 {
			continue
		}
		gs.regionFrontierRow[chosen][nr] |= 1 << uint(nc)
	}
}

// copyRegionOf copies the full [nMax][nMax] fixed-size regionOf snapshot
// into dst via a single value assignment. Out-of-bounds cells (>= n) are
// assumed already zeroed by initGrowState — this helper does not re-zero.
func copyRegionOf(dst, src *[nMax][nMax]int8) {
	*dst = *src
}

// seedComponentsConnected returns true iff every seed mark in `group` is
// reachable from the first seed via 4-adjacent cells that all belong to
// region gid. Uses the shared bfsRegionVisit helper (neighbors.go).
func seedComponentsConnected(dst *[nMax][nMax]int8, group []Mark, gid, n int) bool {
	if len(group) <= 1 {
		return true
	}
	var visited [nMax][nMax]bool
	bfsRegionVisit(dst, gid, group[0].Row, group[0].Col, -1, -1, n, &visited)
	for _, m := range group {
		if !visited[m.Row][m.Col] {
			return false
		}
	}
	return true
}

// bridgeCells claims an L-shaped Manhattan path of cells between a and b
// for region gid, so the region starts 4-connected even when its seed marks
// aren't directly adjacent. Traverses rows first, then columns. Skips any
// cell that is already claimed — the mutator can fix residual
// disconnection later, and bridging a seed through another region's cell
// silently would violate that region's seed invariant.
func bridgeCells(
	dst *[nMax][nMax]int8,
	claimedRow *[nMax]uint16,
	regionSize *[nMax]int,
	a, b Mark,
	gid int,
) {
	// Walk row coordinate from a.Row towards b.Row, keeping col = a.Col.
	// At each step (excluding the start and end, both already seeds), try
	// to claim the cell for gid.
	stepR := 1
	if a.Row > b.Row {
		stepR = -1
	}
	curR := a.Row
	for curR != b.Row {
		curR += stepR
		c := a.Col
		if curR == b.Row && c == b.Col {
			break
		}
		if dst[curR][c] != -1 {
			// Already claimed (likely by this region's other seed). Stop
			// bridging here and let the column walk pick up from b.
			continue
		}
		dst[curR][c] = int8(gid)
		claimedRow[curR] |= 1 << uint(c)
		regionSize[gid]++
	}
	// Now walk column coordinate from a.Col towards b.Col, keeping row = b.Row.
	stepC := 1
	if a.Col > b.Col {
		stepC = -1
	}
	curC := a.Col
	for curC != b.Col {
		curC += stepC
		r := b.Row
		if r == b.Row && curC == b.Col {
			break
		}
		if dst[r][curC] != -1 {
			continue
		}
		dst[r][curC] = int8(gid)
		claimedRow[r] |= 1 << uint(curC)
		regionSize[gid]++
	}
}
