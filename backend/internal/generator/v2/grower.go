package generator

import "math/bits"

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
	n := g.n
	if len(seeds) != n {
		return false
	}

	// regionSize[g] = current cell count for region g.
	var regionSize [nMax]int

	// regionFrontier[g] is a bitmask matrix: bit (r, c) set means cell
	// (r, c) is an unclaimed 4-neighbor of region g. The bit is cleared
	// when the cell is claimed (by ANY region).
	//
	// Storage: regionFrontierRow[g][r] is a uint16 column bitmask for row r.
	var regionFrontierRow [nMax][nMax]uint16

	// claimedRow[r] has a bit set for each claimed column in row r.
	var claimedRow [nMax]uint16

	// Initialize dst: every in-range cell to -1 (unclaimed). Out-of-range
	// cells (>= n) are set to 0 so that later conversion does not read
	// uninitialized sentinel values.
	for r := range nMax {
		for c := range nMax {
			dst[r][c] = 0
		}
	}
	for r := range n {
		for c := range n {
			dst[r][c] = -1
		}
	}

	// Seat seed marks. Each seed's region id is its index in `seeds`.
	for gid, group := range seeds {
		if len(group) != g.k {
			return false
		}
		for _, m := range group {
			if m.Row < 0 || m.Row >= n || m.Col < 0 || m.Col >= n {
				return false
			}
			if dst[m.Row][m.Col] != -1 {
				// Two seed marks landed on the same cell — sampler
				// guarantee violation.
				return false
			}
			dst[m.Row][m.Col] = int8(gid)
			claimedRow[m.Row] |= 1 << uint(m.Col)
			regionSize[gid]++
		}
	}

	// For k >= 2, seeds within a group are not (generally) 4-adjacent, so
	// the region would start disconnected. Bridge each pair of seeds with
	// a Manhattan L-path: move along rows first, then along cols. Skip
	// cells already claimed by another region — bridging silently through
	// another region's cells would violate that region's seed invariant.
	//
	// If a bridge cell IS already claimed and the seeds therefore end up
	// in different 4-components, fail fast and let the orchestrator sample
	// again with a different RNG draw; mid-grow repair is harder than
	// retrying from the sampler. Empirically these collisions are rare
	// because the greedy nearest-neighbor pairer picks the two closest
	// marks and those rarely need a cell another pair has grabbed.
	if g.k >= 2 {
		for gid, group := range seeds {
			for i := 1; i < len(group); i++ {
				bridgeCells(dst, &claimedRow, &regionSize, group[i-1], group[i], gid)
			}
		}
		for gid, group := range seeds {
			if !seedComponentsConnected(dst, group, gid, n) {
				return false
			}
		}
	}

	// Seed each region's frontier with the 4-neighbors of every cell the
	// region currently owns (seed marks + any bridge cells). Unclaimed
	// 4-neighbors become frontier.
	for r := range n {
		for c := range n {
			rid := dst[r][c]
			if rid < 0 {
				continue
			}
			for _, d := range fourNeighborOffsets {
				nr, nc := r+d[0], c+d[1]
				if nr < 0 || nr >= n || nc < 0 || nc >= n {
					continue
				}
				if dst[nr][nc] != -1 {
					continue
				}
				regionFrontierRow[rid][nr] |= 1 << uint(nc)
			}
		}
	}

	// Loop until every cell is claimed. remaining = total in-bounds cells
	// minus the cells already claimed by seeds + bridges.
	remaining := n * n
	for gid := range n {
		remaining -= regionSize[gid]
	}

	// Scratch: list of (region, weight) entries for weighted pick.
	type weighted struct {
		gid    int
		weight int
	}
	var regionBuf [nMax]weighted

	// Scratch: list of frontier cells (linearized r*n+c). Reused each
	// iteration; oversized to the worst case (n*n cells).
	frontierBuf := make([]int, 0, n*n)

	for remaining > 0 {
		// Gather all unclaimed cells that appear in ANY region's frontier.
		// (Using a union mask per row is cheaper than scanning cell by cell.)
		frontierBuf = frontierBuf[:0]
		for r := range n {
			var union uint16
			for gid := range n {
				union |= regionFrontierRow[gid][r]
			}
			// Safety: a frontier bit must never be a claimed bit.
			union &^= claimedRow[r]
			m := union
			for m != 0 {
				c := bits.TrailingZeros16(m)
				m &^= 1 << c
				frontierBuf = append(frontierBuf, r*n+c)
			}
		}
		if len(frontierBuf) == 0 {
			// No unclaimed frontier — means unclaimed cells are
			// disconnected from all regions. Cheap variant can't recover.
			return false
		}

		// Pick a random frontier cell.
		pickIdx := g.rng.IntN(len(frontierBuf))
		cell := frontierBuf[pickIdx]
		cr, cc := cell/n, cell%n
		cellBit := uint16(1) << uint(cc)

		// Collect regions whose frontier contains this cell; compute
		// inverse-size weights.
		totalWeight := 0
		count := 0
		for gid := range n {
			if regionFrontierRow[gid][cr]&cellBit == 0 {
				continue
			}
			// Weight = n - regionSize[gid]. Clamped to 1 to avoid zero/
			// negative when a region is already at target size.
			w := n - regionSize[gid]
			if w < 1 {
				w = 1
			}
			regionBuf[count] = weighted{gid: gid, weight: w}
			totalWeight += w
			count++
		}
		if count == 0 {
			// Should be impossible: if the cell is in frontierBuf, at
			// least one region claims it.
			return false
		}

		// Weighted pick.
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

		// Assign the cell to `chosen`.
		dst[cr][cc] = int8(chosen)
		claimedRow[cr] |= cellBit
		regionSize[chosen]++
		remaining--

		// Strip the now-claimed cell from every region's frontier.
		for gid := range n {
			regionFrontierRow[gid][cr] &^= cellBit
		}

		// Add `chosen`'s new frontier cells: unclaimed 4-neighbors of
		// (cr, cc).
		for _, d := range fourNeighborOffsets {
			nr, nc := cr+d[0], cc+d[1]
			if nr < 0 || nr >= n || nc < 0 || nc >= n {
				continue
			}
			if dst[nr][nc] != -1 {
				continue
			}
			regionFrontierRow[chosen][nr] |= 1 << uint(nc)
		}
	}

	return true
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
