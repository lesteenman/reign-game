package generator

// fourNeighborOffsets lists the 4 orthogonal (N/S/E/W) offsets used for
// region growing, region connectivity checks, and any other 4-adjacency
// reasoning. Package-scope so grower / mutator / future callers share one
// definition rather than inlining the literal.
//
// The 8-neighbor (king's-move) equivalent lives on the test side as
// eightNeighborOffsets in rules_test.go — it exists only to simulate R1
// in tests; production R1 uses bitmask ops (adjacentColumnsMask in
// sample.go).
var fourNeighborOffsets = [4][2]int{
	{-1, 0},
	{0, -1},
	{0, 1},
	{1, 0},
}

// bfsRegionVisit runs a 4-connected BFS from startRow/startCol, visiting
// only cells where regionOf[r][c] == gid (matching the region being
// measured). Optional skipRow/skipCol excludes a single cell from the
// walk (used by the mutator to answer "would the region stay connected
// if this cell were removed?"); pass skipRow = -1 to disable the skip.
//
// Returns the count of cells visited; `visited` is populated so the
// caller can check specific cells (e.g. "was every seed reached?").
//
// Callers pass a caller-owned `visited` array — the function only writes
// `true`, so zero-initialization is the caller's responsibility. Allowing
// the caller to own the buffer keeps bfsRegionVisit allocation-free and
// lets hot-path callers pre-zero only the cells they expect to touch.
func bfsRegionVisit(
	regionOf *[nMax][nMax]int8,
	gid, startRow, startCol, skipRow, skipCol, n int,
	visited *[nMax][nMax]bool,
) int {
	if startRow < 0 || startRow >= n || startCol < 0 || startCol >= n {
		return 0
	}
	if skipRow == startRow && skipCol == startCol {
		return 0
	}
	if int(regionOf[startRow][startCol]) != gid {
		return 0
	}

	visited[startRow][startCol] = true
	count := 1

	// A fixed-size ring would be more efficient for a hot path; this slice
	// is allocation-free on the typical n<=16 case (cap of nMax*nMax) but
	// still has slice-shift churn. Callers on a tight loop (R-066's mutator
	// probes) will want to pass a caller-owned queue buffer. Deferred.
	queue := make([]Mark, 0, nMax*nMax)
	queue = append(queue, Mark{Row: startRow, Col: startCol})

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range fourNeighborOffsets {
			nr, nc := cur.Row+d[0], cur.Col+d[1]
			if nr < 0 || nr >= n || nc < 0 || nc >= n {
				continue
			}
			if visited[nr][nc] {
				continue
			}
			if nr == skipRow && nc == skipCol {
				continue
			}
			if int(regionOf[nr][nc]) != gid {
				continue
			}
			visited[nr][nc] = true
			count++
			queue = append(queue, Mark{Row: nr, Col: nc})
		}
	}
	return count
}
