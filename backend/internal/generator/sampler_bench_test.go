package generator

import "testing"

// BenchmarkSolutionSample measures raw throughput of sampleSolution across
// the supported (N, k) grid. Per PG-03 and input-spec §8 the floor is
// N=Nmin=6 at k=1 and N=9 at k=2 (content-dead below that, see
// bench/n-feasibility.md).
func BenchmarkSolutionSample(b *testing.B) {
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
			g, err := New(c.n, c.k, WithSeed(int64(100*c.n+c.k)))
			if err != nil {
				b.Fatalf("New: %v", err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, ok := g.sampleSolution(); !ok {
					b.Fatalf("sampler failed at i=%d", i)
				}
			}
		})
	}
}
