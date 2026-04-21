//go:build soak

package generator

import (
	"context"
	"errors"
	"testing"
)

// TestSoak runs 10,000+ Generate() calls across the committed (N, k)
// grid and asserts every invariant the property + grower tests cover:
// partition structure, region min-size (R-067b), solution shape,
// brute-uniqueness, deductive-brute agreement, and deductive solver
// still reaches OutcomeSolved.
//
// Build-tagged because 10k samples at N=12 is ~30 min wall-clock. Runs
// via .github/workflows/generator-check.yml (non-blocking) and
// locally via `go test -tags=soak -run TestSoak -v ./internal/generator/`.
func TestSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak under -short")
	}
	t.Parallel()

	cases := []struct {
		n, k, samples int
	}{
		{n: 9, k: 1, samples: 3000},
		{n: 12, k: 1, samples: 2000},
		{n: 9, k: 2, samples: 3000},
		{n: 12, k: 2, samples: 2000},
	}

	for _, c := range cases {
		c := c
		t.Run(namef(c.n, c.k), func(t *testing.T) {
			t.Parallel()

			g, err := New(c.n, c.k, WithSeed(int64(600*c.n+c.k)), WithMaxAttempts(20))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			succeeded := 0
			for i := 0; i < c.samples; i++ {
				p, err := g.Generate(context.Background())
				if err != nil {
					if !errors.Is(err, ErrMaxAttemptsExhausted) {
						t.Fatalf("sample %d: %v", i, err)
					}
					continue
				}
				label := fmtSample(c.n, c.k, i)
				assertRegionPartition(t, label, p.Regions, p.N)
				if want := p.N * p.MarksPerUnit; len(p.Solution) != want {
					t.Fatalf("%s: %d solution marks, want %d",
						label, len(p.Solution), want)
				}
				assertDeductiveBruteAgree(t, label, &p)
				succeeded++
			}
			t.Logf("N=%d k=%d: %d/%d ok", c.n, c.k, succeeded, c.samples)
		})
	}
}
