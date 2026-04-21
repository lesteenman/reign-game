//go:build soak

package generator

import (
	"context"
	"errors"
	"math"
	"os"
	"strconv"
	"testing"
)

// TestSoak runs 10 000+ Generate() calls across the committed (N, k)
// grid and asserts every invariant the property + grower tests cover:
// partition structure, region min-size (R-067b), solution shape,
// brute-uniqueness, deductive-brute agreement, and deductive solver
// still reaches OutcomeSolved.
//
// REIGN_SOAK_SCALE (float, default 1.0) multiplies the per-bucket
// sample count. CI runs set REIGN_SOAK_SCALE=0.1 for a ~20 min PR
// check; full-depth runs on a dev box take the default 1.0 and push
// through the full 10k. Per input-spec §7.1, the 10k depth is for
// deep checks and not required for normal CI.
//
// Build-tagged. Invoke directly:
//
//	go test -tags=soak -run TestSoak -v -timeout=45m ./internal/generator/
func TestSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak under -short")
	}
	t.Parallel()

	scale := 1.0
	if v := os.Getenv("REIGN_SOAK_SCALE"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatalf("REIGN_SOAK_SCALE=%q: %v", v, err)
		}
		if parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			t.Fatalf("REIGN_SOAK_SCALE=%q: must be a positive finite float", v)
		}
		scale = parsed
	}

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
		scaled := int(float64(c.samples) * scale)
		if scaled < 1 {
			scaled = 1
		}
		t.Run(namef(c.n, c.k), func(t *testing.T) {
			t.Parallel()

			g, err := New(c.n, c.k, WithSeed(int64(600*c.n+c.k)), WithMaxAttempts(20))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			succeeded := 0
			for i := 0; i < scaled; i++ {
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
			t.Logf("N=%d k=%d: %d/%d ok (scale=%.2f)", c.n, c.k, succeeded, scaled, scale)
		})
	}
}
