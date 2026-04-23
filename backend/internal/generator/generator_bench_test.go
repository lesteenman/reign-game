package generator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// BenchmarkGenerateOne measures end-to-end Generate latency across the
// committed (N, k) grid. Per input-spec §8 the range is N=Nmin..14 at
// k=1 and N=9..14 at k=2 (k=2 is content-dead below N=9, see
// bench/n-feasibility.md).
//
// ErrMaxAttemptsExhausted counts as a "slow" op — we still spent the
// time. Consumers read op/s from b.N.
func BenchmarkGenerateOne(b *testing.B) {
	cases := []struct {
		name string
		n, k int
	}{
		{name: "N=6/k=1", n: 6, k: 1},
		{name: "N=7/k=1", n: 7, k: 1},
		{name: "N=8/k=1", n: 8, k: 1},
		{name: "N=9/k=1", n: 9, k: 1},
		{name: "N=10/k=1", n: 10, k: 1},
		{name: "N=11/k=1", n: 11, k: 1},
		{name: "N=12/k=1", n: 12, k: 1},
		{name: "N=13/k=1", n: 13, k: 1},
		{name: "N=14/k=1", n: 14, k: 1},
		{name: "N=9/k=2", n: 9, k: 2},
		{name: "N=10/k=2", n: 10, k: 2},
		{name: "N=11/k=2", n: 11, k: 2},
		{name: "N=12/k=2", n: 12, k: 2},
		{name: "N=13/k=2", n: 13, k: 2},
		{name: "N=14/k=2", n: 14, k: 2},
	}

	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) {
			g, err := New(c.n, c.k, WithMaxAttempts(50))
			if err != nil {
				b.Fatalf("New: %v", err)
			}
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := g.Generate(ctx)
				if err != nil && !errors.Is(err, ErrMaxAttemptsExhausted) {
					b.Fatalf("Generate: %v", err)
				}
			}
		})
	}
}

// BenchmarkGenerateParallel measures aggregate Generate throughput with
// one Generator per goroutine (Generators are not safe for concurrent
// use — see §9 of input-spec). Reported as ns/op against b.N *total*
// calls across all goroutines, so op/s = goroutines × per-goroutine
// throughput. This is the number the consumer uses to pick between a
// Lambda-backed or local-batch deployment (§8).
//
// The PG-12 / R-066 solver-guided grower clones solver state per probe,
// so parallel throughput does not scale quite as well as the single-
// threaded benchmark would suggest — running this is the only honest
// way to size the Lambda concurrency.
func BenchmarkGenerateParallel(b *testing.B) {
	cases := []struct {
		name string
		n, k int
	}{
		{name: "N=12/k=1", n: 12, k: 1},
		{name: "N=12/k=2", n: 12, k: 2},
	}

	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) {
			var seed atomic.Int64
			seed.Store(int64(400*c.n + c.k))
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				// Each goroutine holds its own Generator seeded uniquely
				// so parallel runs do not share RNG state.
				g, err := New(c.n, c.k,
					WithSeed(seed.Add(1)),
					WithMaxAttempts(50),
				)
				if err != nil {
					b.Fatalf("New: %v", err)
				}
				ctx := context.Background()
				for pb.Next() {
					_, err := g.Generate(ctx)
					if err != nil && !errors.Is(err, ErrMaxAttemptsExhausted) {
						b.Fatalf("Generate: %v", err)
					}
				}
			})
		})
	}
}
