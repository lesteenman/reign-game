//go:build soak

package generator

import (
	"context"
	"errors"
	"testing"
)

// TestSoak runs 10,000+ Generate() calls across the supported (N, k)
// grid, asserting structural invariants on every successful puzzle:
//   - Region map is an [N][N] partition with exactly N region ids.
//   - Every region has >= regionMinSize cells.
//   - Solution has exactly N*k marks and brute-solves to exactly one
//     solution.
//   - Deductive solver's solution matches the brute solution.
//
// Build-tagged because 10k samples at N=12 is ~30 min wall-clock. Runs
// via `.github/workflows/generator-check.yml` (non-blocking) and
// locally via `task bench:generator:soak`.
//
// Invocation:
//
//	go test -tags=soak -run TestSoak -v -timeout=45m ./internal/generator/
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
				assertSoakInvariants(t, i, &p)
				succeeded++
			}
			t.Logf("N=%d k=%d: %d/%d ok", c.n, c.k, succeeded, c.samples)
		})
	}
}

func assertSoakInvariants(t *testing.T, sample int, p *Puzzle) {
	t.Helper()

	n := p.N
	if len(p.Regions) != n {
		t.Fatalf("sample %d: regions rows=%d, want %d", sample, len(p.Regions), n)
	}
	sizes := make([]int, n)
	for r := range p.Regions {
		if len(p.Regions[r]) != n {
			t.Fatalf("sample %d: row %d has %d cols, want %d",
				sample, r, len(p.Regions[r]), n)
		}
		for _, gid := range p.Regions[r] {
			if gid < 0 || gid >= n {
				t.Fatalf("sample %d: region id %d out of [0, %d)",
					sample, gid, n)
			}
			sizes[gid]++
		}
	}
	for gid, sz := range sizes {
		if sz < regionMinSize {
			t.Fatalf("sample %d: region %d has %d cells (< min %d)",
				sample, gid, sz, regionMinSize)
		}
	}
	if want := n * p.MarksPerUnit; len(p.Solution) != want {
		t.Fatalf("sample %d: %d solution marks, want %d",
			sample, len(p.Solution), want)
	}
	sols, err := bruteSolveAll(p.Regions, n, p.MarksPerUnit, 2)
	if err != nil {
		t.Fatalf("sample %d: brute: %v", sample, err)
	}
	if len(sols) != 1 {
		t.Fatalf("sample %d: brute returned %d solutions, want 1", sample, len(sols))
	}
	if !marksEqualUnordered(sols[0], p.Solution) {
		t.Fatalf("sample %d: deductive solution != brute solution\n  brute: %v\n  deductive: %v",
			sample, sols[0], p.Solution)
	}
}
