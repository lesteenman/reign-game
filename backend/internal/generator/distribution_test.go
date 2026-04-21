//go:build distribution

package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestDifficultyDistribution runs Generate() for a per-(N, k) wall-
// clock budget and records the difficulty histogram + Expert yield.
// Writes bench/difficulty-distribution.md when REIGN_BENCH_WRITE=1.
//
// Build-tagged. Default budget is 1 hour per bucket — 4 hours total
// wall-clock — so a full refresh needs `-timeout=8h`. Shrink via
// REIGN_DIST_BUDGET_SEC for a faster pilot.
//
// Invocation:
//
//	REIGN_BENCH_WRITE=1 REIGN_DIST_BUDGET_SEC=1800 \
//	  go test -tags=distribution -run TestDifficultyDistribution -v \
//	  -timeout=8h ./internal/generator/
//
// Buckets cover the committed gate and the stretch ceiling. N=14 is
// informational — wall time per call is >10s so sample counts are
// intentionally small.
func TestDifficultyDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping distribution under -short")
	}

	budgetSec := 3600
	if v := os.Getenv("REIGN_DIST_BUDGET_SEC"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("REIGN_DIST_BUDGET_SEC=%q: %v", v, err)
		}
		budgetSec = parsed
	}
	budget := time.Duration(budgetSec) * time.Second

	// cases is already in (n, k) order; the write-to-md loop preserves
	// that order, so no post-run sort is needed.
	cases := []struct{ n, k int }{
		{n: 12, k: 1},
		{n: 12, k: 2},
		{n: 14, k: 1},
		{n: 14, k: 2},
	}

	type result struct {
		n, k      int
		elapsed   time.Duration
		attempts  int
		successes int
		byDiff    map[Difficulty]int
		totalDur  time.Duration
	}
	results := make([]result, 0, len(cases))

	for _, c := range cases {
		g, err := New(c.n, c.k, WithSeed(int64(1200*c.n+c.k)), WithMaxAttempts(20))
		if err != nil {
			t.Fatalf("New %d/%d: %v", c.n, c.k, err)
		}

		r := result{
			n:      c.n,
			k:      c.k,
			byDiff: make(map[Difficulty]int),
		}
		deadline := time.Now().Add(budget)
		ctx := context.Background()
		for time.Now().Before(deadline) {
			r.attempts++
			t0 := time.Now()
			p, err := g.Generate(ctx)
			r.totalDur += time.Since(t0)
			if err != nil {
				if errors.Is(err, ErrMaxAttemptsExhausted) {
					continue
				}
				t.Fatalf("N=%d k=%d: %v", c.n, c.k, err)
			}
			r.successes++
			r.byDiff[p.Difficulty]++
		}
		r.elapsed = budget - time.Until(deadline)
		results = append(results, r)
		t.Logf("N=%d k=%d: %d/%d ok over %s (%s/puzzle)",
			c.n, c.k, r.successes, r.attempts, r.elapsed,
			r.totalDur/time.Duration(max(r.successes, 1)))
	}

	out := "# Generate() difficulty distribution (R-068d)\n\n"
	out += fmt.Sprintf("Each bucket ran for %s wall-clock. Override with REIGN_DIST_BUDGET_SEC. Captured by `TestDifficultyDistribution` under `-tags=distribution`.\n\n",
		budget)
	out += "| (N, k) | attempts | ok | ok/min | easy | medium | hard | expert | expert yield |\n"
	out += "|---|---|---|---|---|---|---|---|---|\n"
	for _, r := range results {
		okPerMin := float64(r.successes) * 60 / r.elapsed.Seconds()
		expertYield := 0.0
		if r.successes > 0 {
			expertYield = float64(r.byDiff[Expert]) / float64(r.successes)
		}
		out += fmt.Sprintf("| %d, %d | %d | %d | %.1f | %d | %d | %d | %d | %.1f%% |\n",
			r.n, r.k, r.attempts, r.successes, okPerMin,
			r.byDiff[Easy], r.byDiff[Medium], r.byDiff[Hard], r.byDiff[Expert],
			100*expertYield,
		)
	}
	out += "\n## Interpretation\n\n"
	out += "- **Expert yield** is `expert_count / total_successes`. Per input-spec §10 the `WithDifficulty(Expert)` filter is a retry-until-match loop. If yield is far below the consumer's throughput target the retry budget dominates wall time and v2 difficulty-targeting (biased generation) becomes necessary.\n"
	out += "- **Throughput (ok/min)** at N=12 drives Lambda concurrency sizing. Combine with `BenchmarkGenerateParallel` (baseline.txt) to project aggregate pool refill rate.\n"
	out += "- Row-by-row quirks (a bucket starving in one tier) are usually a classifier issue rather than a generator issue; cross-check against the rule trace distribution from the property-corpus logs.\n"

	if os.Getenv("REIGN_BENCH_WRITE") != "1" {
		t.Log("REIGN_BENCH_WRITE not set; skipping bench/difficulty-distribution.md write")
		return
	}
	path := "bench/difficulty-distribution.md"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
