package generator

import (
	"testing"
)

// TestGrowRegionsInvariants: for 200 random sampled solutions at each
// supported (N, k), verify the grower produces a valid partition:
//   - Every cell in [0..n)x[0..n) holds a region id in [0, n).
//   - Every region is 4-connected.
//   - Every region contains all k of its seed marks.
//   - Exactly n distinct region ids are used, each non-empty.
func TestGrowRegionsInvariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		n, k    int
		samples int
	}{
		{n: 6, k: 1, samples: 200},
		{n: 9, k: 1, samples: 200},
		{n: 12, k: 1, samples: 200},
		{n: 9, k: 2, samples: 200},
		{n: 12, k: 2, samples: 200},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(namef(tc.n, tc.k), func(t *testing.T) {
			t.Parallel()

			// Arrange
			g, err := New(tc.n, tc.k, WithSeed(int64(1000*tc.n+tc.k)))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			grown := 0
			attempts := 0
			// Give the sampler/grower a generous retry budget — the
			// grower's bridging step can fail on k=2 when two pairs
			// stake overlapping L-paths. Those failures are legitimate
			// (orchestrator restarts the attempt) but we need `samples`
			// successful grows for invariant checking.
			const maxAttemptsFactor = 4
			for grown < tc.samples && attempts < tc.samples*maxAttemptsFactor {
				attempts++
				sol, ok := g.sampleSolution()
				if !ok {
					t.Fatalf("attempt %d: sampler failed", attempts)
				}
				seeds := pairSeeds(sol, tc.n, tc.k)

				// Act
				var rm [nMax][nMax]int8
				if !g.growRegions(seeds, &rm) {
					continue
				}

				// Assert
				assertPartition(t, grown, &rm, tc.n)
				assertSeedsInRegion(t, grown, &rm, seeds)
				assertRegionsConnected(t, grown, &rm, tc.n)
				grown++
			}
			if grown < tc.samples {
				t.Fatalf("only %d/%d grows succeeded in %d attempts (growth success rate %.1f%%)",
					grown, tc.samples, attempts, 100*float64(grown)/float64(attempts))
			}
			t.Logf("grew %d/%d (success %.1f%%)", grown, attempts, 100*float64(grown)/float64(attempts))
		})
	}
}

func namef(n, k int) string {
	switch {
	case n == 6 && k == 1:
		return "N=6/k=1"
	case n == 9 && k == 1:
		return "N=9/k=1"
	case n == 12 && k == 1:
		return "N=12/k=1"
	case n == 9 && k == 2:
		return "N=9/k=2"
	case n == 12 && k == 2:
		return "N=12/k=2"
	}
	return "unknown"
}

func assertPartition(t *testing.T, sample int, rm *[nMax][nMax]int8, n int) {
	t.Helper()
	used := make([]int, n)
	for r := range n {
		for c := range n {
			rid := rm[r][c]
			if rid < 0 || int(rid) >= n {
				t.Fatalf("sample %d: rm[%d][%d]=%d out of [0,%d)",
					sample, r, c, rid, n)
			}
			used[rid]++
		}
	}
	for i, count := range used {
		if count == 0 {
			t.Fatalf("sample %d: region %d is empty", sample, i)
		}
	}
}

func assertSeedsInRegion(t *testing.T, sample int, rm *[nMax][nMax]int8, seeds [][]Mark) {
	t.Helper()
	for gid, group := range seeds {
		for _, m := range group {
			if rm[m.Row][m.Col] != int8(gid) {
				t.Fatalf("sample %d: seed %v for region %d ended up in region %d",
					sample, m, gid, rm[m.Row][m.Col])
			}
		}
	}
}

func assertRegionsConnected(t *testing.T, sample int, rm *[nMax][nMax]int8, n int) {
	t.Helper()
	// BFS from any cell of each region; count cells reached.
	visited := make([][]bool, n)
	for r := range visited {
		visited[r] = make([]bool, n)
	}

	regionSize := make([]int, n)
	regionStart := make([]Mark, n)
	for r := range n {
		for c := range n {
			rid := int(rm[r][c])
			regionSize[rid]++
			if regionSize[rid] == 1 {
				regionStart[rid] = Mark{Row: r, Col: c}
			}
		}
	}

	for rid := range n {
		start := regionStart[rid]
		// BFS
		queue := []Mark{start}
		visited[start.Row][start.Col] = true
		reached := 0
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			reached++
			for _, d := range fourNeighborOffsets {
				nr, nc := cur.Row+d[0], cur.Col+d[1]
				if nr < 0 || nr >= n || nc < 0 || nc >= n {
					continue
				}
				if visited[nr][nc] {
					continue
				}
				if int(rm[nr][nc]) != rid {
					continue
				}
				visited[nr][nc] = true
				queue = append(queue, Mark{Row: nr, Col: nc})
			}
		}
		if reached != regionSize[rid] {
			t.Fatalf("sample %d: region %d not 4-connected (reached %d / %d cells)",
				sample, rid, reached, regionSize[rid])
		}
	}
}

// TestGrowRegionsSolverGuidedInvariants: solver-guided variant (R-066)
// must produce identical structural invariants as the cheap variant —
// valid 4-connected partition, seeds in their own regions, exactly N
// regions used. The success rate is exercised separately in Step 7.
func TestGrowRegionsSolverGuidedInvariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		n, k    int
		samples int
	}{
		{n: 6, k: 1, samples: 40},
		{n: 9, k: 1, samples: 40},
		{n: 12, k: 1, samples: 40},
		{n: 9, k: 2, samples: 40},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(namef(tc.n, tc.k), func(t *testing.T) {
			t.Parallel()

			// Arrange
			g, err := New(tc.n, tc.k, WithSeed(int64(2000*tc.n+tc.k)))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			grown := 0
			attempts := 0
			const maxAttemptsFactor = 6
			for grown < tc.samples && attempts < tc.samples*maxAttemptsFactor {
				attempts++
				sol, ok := g.sampleSolution()
				if !ok {
					t.Fatalf("attempt %d: sampler failed", attempts)
				}
				seeds := pairSeeds(sol, tc.n, tc.k)

				// Act
				var rm [nMax][nMax]int8
				if !g.growRegionsSolverGuided(seeds, &rm) {
					continue
				}

				// Assert
				assertPartition(t, grown, &rm, tc.n)
				assertSeedsInRegion(t, grown, &rm, seeds)
				assertRegionsConnected(t, grown, &rm, tc.n)
				grown++
			}
			if grown < tc.samples {
				t.Fatalf("only %d/%d solver-guided grows succeeded in %d attempts (%.1f%%)",
					grown, tc.samples, attempts, 100*float64(grown)/float64(attempts))
			}
		})
	}
}

// TestGrowRegionsSolverGuidedDeterministic: same seed -> same region map
// for the solver-guided variant. Determinism proof for the R-066 variant.
func TestGrowRegionsSolverGuidedDeterministic(t *testing.T) {
	t.Parallel()

	// Arrange
	const n, k = 9, 1
	g1, _ := New(n, k, WithSeed(4242))
	g2, _ := New(n, k, WithSeed(4242))
	sol, ok := g1.sampleSolution()
	if !ok {
		t.Fatal("sampler failed")
	}
	_, _ = g2.sampleSolution() // keep RNG streams aligned
	seeds := pairSeeds(sol, n, k)

	// Act
	var rm1, rm2 [nMax][nMax]int8
	if !g1.growRegionsSolverGuided(seeds, &rm1) {
		t.Fatal("solver-guided grow 1 failed")
	}
	if !g2.growRegionsSolverGuided(seeds, &rm2) {
		t.Fatal("solver-guided grow 2 failed")
	}

	// Assert
	for r := range n {
		for c := range n {
			if rm1[r][c] != rm2[r][c] {
				t.Fatalf("determinism: rm1[%d][%d]=%d, rm2=%d",
					r, c, rm1[r][c], rm2[r][c])
			}
		}
	}
}

// TestGrowRegionsDeterministicWithSeed: two generators with the same seed
// must produce the same region map from the same seeds input.
func TestGrowRegionsDeterministicWithSeed(t *testing.T) {
	t.Parallel()

	// Arrange
	const n, k = 9, 1
	g1, _ := New(n, k, WithSeed(42))
	g2, _ := New(n, k, WithSeed(42))
	sol, ok := g1.sampleSolution()
	if !ok {
		t.Fatal("sampler failed")
	}
	_, _ = g2.sampleSolution() // sync the RNG stream
	seeds := pairSeeds(sol, n, k)

	// Act
	var rm1, rm2 [nMax][nMax]int8
	if !g1.growRegions(seeds, &rm1) {
		t.Fatal("grow 1 failed")
	}
	if !g2.growRegions(seeds, &rm2) {
		t.Fatal("grow 2 failed")
	}

	// Assert
	for r := range n {
		for c := range n {
			if rm1[r][c] != rm2[r][c] {
				t.Fatalf("determinism: rm1[%d][%d]=%d, rm2=%d",
					r, c, rm1[r][c], rm2[r][c])
			}
		}
	}
}
