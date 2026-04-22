package generator

import (
	"context"
	"errors"
	"testing"
)

// TestGenerateRespectsMinSize is the end-to-end version of
// TestGrowRegionsMinSize. The grower-only test passed throughout R-067b
// and R-067c, but a production puzzle surfaced a 1-cell region in a
// served 9x9 Standard puzzle, meaning the rule leaks somewhere between
// the grower's output and Generate's return. This test drives
// Generate() directly and asserts the returned Puzzle.Regions has no
// region below regionMinSize.
//
// Covers the committed Step 7 combos plus a couple of smaller grids
// so a leak on any combo is caught. Each bucket seeds the generator
// deterministically so a failure reproduces.
func TestGenerateRespectsMinSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping min-size gate under -short")
	}
	t.Parallel()

	cases := []struct {
		n, k, samples int
	}{
		{n: 6, k: 1, samples: 100},
		{n: 9, k: 1, samples: 200},
		{n: 12, k: 1, samples: 100},
		{n: 9, k: 2, samples: 200},
		{n: 12, k: 2, samples: 50},
	}

	for _, c := range cases {
		c := c
		t.Run(namef(c.n, c.k), func(t *testing.T) {
			t.Parallel()

			g, err := New(c.n, c.k, WithSeed(int64(4000*c.n+c.k)), WithMaxAttempts(20))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			for i := 0; i < c.samples; i++ {
				p, err := g.Generate(context.Background())
				if err != nil {
					if errors.Is(err, ErrMaxAttemptsExhausted) {
						continue
					}
					t.Fatalf("N=%d k=%d sample %d: %v", c.n, c.k, i, err)
				}
				sizes := make([]int, c.n)
				for r := range p.Regions {
					for _, gid := range p.Regions[r] {
						sizes[gid]++
					}
				}
				for gid, sz := range sizes {
					if sz < regionMinSize {
						t.Fatalf("N=%d k=%d sample %d: region %d has %d cells (< min %d)\nregions=%v",
							c.n, c.k, i, gid, sz, regionMinSize, p.Regions)
					}
				}
			}
		})
	}
}
