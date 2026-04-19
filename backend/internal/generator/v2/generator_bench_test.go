package generator

import (
	"context"
	"errors"
	"testing"
)

// BenchmarkGenerateOne measures end-to-end Generate latency at the
// committed (N, k) points. The gate from input-spec §6.1 / tasks.md is
// <5s/op at N=12 k=1 (initial; tightened in R-068).
//
// Note: ErrMaxAttemptsExhausted counts as a "slow" op (we still spent the
// time). We report b.N worth of attempts; the caller reads op/s from there.
func BenchmarkGenerateOne(b *testing.B) {
	cases := []struct {
		name string
		n, k int
	}{
		{name: "N=8/k=1", n: 8, k: 1},
		{name: "N=9/k=1", n: 9, k: 1},
		{name: "N=12/k=1", n: 12, k: 1},
		{name: "N=9/k=2", n: 9, k: 2},
		{name: "N=12/k=2", n: 12, k: 2},
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
