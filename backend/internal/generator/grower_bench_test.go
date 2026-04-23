package generator

import "testing"

// preSampleSeeds draws `count` seed sets from `g` ahead of the timed
// loop so BenchmarkRegionGrow* measures only the grower, not the
// sampler. Sampler failures are skipped and retried until the buffer
// is full.
func preSampleSeeds(b *testing.B, g *Generator, n, k, count int) [][][]Mark {
	b.Helper()
	seeds := make([][][]Mark, 0, count)
	for len(seeds) < count {
		sol, ok := g.sampleSolution()
		if !ok {
			continue
		}
		seeds = append(seeds, pairSeeds(sol, n, k))
	}
	return seeds
}

// BenchmarkRegionGrow measures the cheap grower (growRegions) alone, with
// sampler+pair-seed work moved outside the timed loop. Seeds are pre-
// sampled per b.N so the inner loop times only growRegions. Per PG-08 and
// input-spec §8.
func BenchmarkRegionGrow(b *testing.B) {
	cases := []struct {
		name string
		n, k int
	}{
		{name: "N=6/k=1", n: 6, k: 1},
		{name: "N=8/k=1", n: 8, k: 1},
		{name: "N=10/k=1", n: 10, k: 1},
		{name: "N=12/k=1", n: 12, k: 1},
		{name: "N=14/k=1", n: 14, k: 1},
		{name: "N=9/k=2", n: 9, k: 2},
		{name: "N=10/k=2", n: 10, k: 2},
		{name: "N=12/k=2", n: 12, k: 2},
		{name: "N=14/k=2", n: 14, k: 2},
	}

	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) {
			g, err := New(c.n, c.k, WithSeed(int64(200*c.n+c.k)))
			if err != nil {
				b.Fatalf("New: %v", err)
			}
			// Pre-sample seed sets so the timed loop measures only the
			// grower. Sampler cost is covered by BenchmarkSolutionSample.
			seeds := preSampleSeeds(b, g, c.n, c.k, b.N)

			var dst [nMax][nMax]int8
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = g.growRegions(seeds[i], &dst)
			}
		})
	}
}

// BenchmarkRegionGrowSolverGuided measures the solver-guided grower
// (growRegionsSolverGuided) alone. This variant clones solver state per
// probe, so its per-op cost is dominated by the probe count — per-N cost
// grows faster than the cheap variant.
func BenchmarkRegionGrowSolverGuided(b *testing.B) {
	cases := []struct {
		name string
		n, k int
	}{
		{name: "N=6/k=1", n: 6, k: 1},
		{name: "N=9/k=1", n: 9, k: 1},
		{name: "N=12/k=1", n: 12, k: 1},
		{name: "N=9/k=2", n: 9, k: 2},
		{name: "N=12/k=2", n: 12, k: 2},
	}

	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) {
			g, err := New(c.n, c.k, WithSeed(int64(300*c.n+c.k)))
			if err != nil {
				b.Fatalf("New: %v", err)
			}
			seeds := make([][][]Mark, 0, b.N)
			for len(seeds) < b.N {
				sol, ok := g.sampleSolution()
				if !ok {
					continue
				}
				seeds = append(seeds, pairSeeds(sol, c.n, c.k))
			}

			var dst [nMax][nMax]int8
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = g.growRegionsSolverGuided(seeds[i], &dst)
			}
		})
	}
}
