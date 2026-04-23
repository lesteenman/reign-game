package generator

// pairSeeds groups the N*k solution marks into N seed groups of k marks each,
// one group per target region. The grouping rule depends on k:
//
//   - k=1: identity pairing. Each mark is its own singleton seed group.
//     Result shape: [N][1]Mark.
//   - k=2: greedy nearest-neighbor pairing by Manhattan distance. Repeatedly
//     pick the two unpaired marks whose Manhattan distance is smallest; tie-
//     break by the pair-creation order encountered during the scan (stable
//     for a given input order). Result shape: [N][2]Mark.
//
// The caller guarantees len(solution) == n*k. Returns exactly N groups.
//
// Pure function. No state on the Generator is touched. Small allocations
// are acceptable (pair() is called once per attempt, not per mutation).
func pairSeeds(solution []Mark, n, k int) [][]Mark {
	if k == 1 {
		out := make([][]Mark, n)
		for i := range solution {
			out[i] = []Mark{solution[i]}
		}
		return out
	}

	// k == 2.
	total := n * k
	paired := make([]bool, total)
	out := make([][]Mark, 0, n)

	for len(out) < n {
		bestI, bestJ, bestDist := -1, -1, -1
		for i := 0; i < total; i++ {
			if paired[i] {
				continue
			}
			for j := i + 1; j < total; j++ {
				if paired[j] {
					continue
				}
				d := manhattan(solution[i], solution[j])
				if bestDist == -1 || d < bestDist {
					bestI, bestJ, bestDist = i, j, d
				}
			}
		}
		if bestI == -1 {
			// Shouldn't happen: an even number of unpaired marks exists
			// as long as total = N*k = 2N with k=2.
			break
		}
		paired[bestI] = true
		paired[bestJ] = true
		out = append(out, []Mark{solution[bestI], solution[bestJ]})
	}
	return out
}

// manhattan returns the L1 distance between two marks.
func manhattan(a, b Mark) int {
	dr := a.Row - b.Row
	if dr < 0 {
		dr = -dr
	}
	dc := a.Col - b.Col
	if dc < 0 {
		dc = -dc
	}
	return dr + dc
}
