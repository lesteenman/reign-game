//go:build latency

package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// TestLatencyDistribution records per-call Generate() durations at each
// committed (N, k) and writes median / P90 / P99 / max to
// bench/latency-distribution.md.
//
// Build-tagged because it runs 200 samples per combo at N=12/14, which
// pushes wall-clock past the default CI budget. Run explicitly:
//
//	go test -tags=latency -run TestLatencyDistribution -v \
//	  -timeout=60m ./internal/generator/
//
// Output lives under bench/ so a review PR can include the committed
// distribution alongside the bench baseline.
func TestLatencyDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency distribution under -short")
	}

	// Sample counts are chosen so each bucket fits in a ~2 min local run.
	// Stretch N=14 lives in R-068d's 1-hour distribution suite, not here.
	cases := []struct {
		n, k    int
		samples int
	}{
		{n: 6, k: 1, samples: 500},
		{n: 9, k: 1, samples: 300},
		{n: 12, k: 1, samples: 200},
		{n: 9, k: 2, samples: 300},
		{n: 12, k: 2, samples: 100},
	}

	type row struct {
		n, k                                 int
		samples                              int
		successes                            int
		medianNs, p90Ns, p99Ns, maxNs, minNs int64
		p99OverMedian                        float64
	}
	rows := make([]row, 0, len(cases))

	for _, c := range cases {
		g, err := New(c.n, c.k, WithSeed(int64(500*c.n+c.k)), WithMaxAttempts(20))
		if err != nil {
			t.Fatalf("New %d/%d: %v", c.n, c.k, err)
		}
		ctx := context.Background()
		durs := make([]int64, 0, c.samples)
		successes := 0
		for i := 0; i < c.samples; i++ {
			start := time.Now()
			_, err := g.Generate(ctx)
			d := time.Since(start).Nanoseconds()
			durs = append(durs, d)
			if err == nil {
				successes++
			} else if !errors.Is(err, ErrMaxAttemptsExhausted) {
				t.Fatalf("N=%d k=%d sample %d: %v", c.n, c.k, i, err)
			}
		}
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
		median := durs[len(durs)/2]
		p90 := durs[min(len(durs)*9/10, len(durs)-1)]
		p99 := durs[min(len(durs)*99/100, len(durs)-1)]
		rows = append(rows, row{
			n: c.n, k: c.k,
			samples:       c.samples,
			successes:     successes,
			minNs:         durs[0],
			medianNs:      median,
			p90Ns:         p90,
			p99Ns:         p99,
			maxNs:         durs[len(durs)-1],
			p99OverMedian: float64(p99) / float64(median),
		})
		t.Logf("N=%d k=%d: %d/%d ok, median=%s p99=%s max=%s ratio=%.2fx",
			c.n, c.k, successes, c.samples,
			time.Duration(median), time.Duration(p99), time.Duration(durs[len(durs)-1]),
			float64(p99)/float64(median))
	}

	out := "# Generate() latency distribution (R-068a)\n\n"
	out += "Per-call `Generate()` timings at each committed (N, k). Captured by `TestLatencyDistribution` under `-tags=latency`. Samples vary by N because wall-clock at N=14 is high; sample counts are chosen so every bucket fits in a reasonable local run.\n\n"
	out += "| (N, k) | samples | ok | median | p90 | p99 | max | p99/median |\n"
	out += "|---|---|---|---|---|---|---|---|\n"
	for _, r := range rows {
		out += fmt.Sprintf("| %d, %d | %d | %d | %s | %s | %s | %s | %.2fx |\n",
			r.n, r.k, r.samples, r.successes,
			time.Duration(r.medianNs), time.Duration(r.p90Ns),
			time.Duration(r.p99Ns), time.Duration(r.maxNs),
			r.p99OverMedian,
		)
	}
	out += "\n## Interpretation\n\nPer input-spec §11, p99/median > 3× at any committed (N, k) is the trigger for recommending `WithRacing` in Step 11's handoff. A row with ratio <= 3× is within the single-stream budget; >= 3× means the slow-tail attempts are blocking a P99-sensitive consumer (Lambda response, user-facing generate-on-demand). The handoff document (bench/step11-handoff.md) reads this table and makes the racing call.\n"

	path := "bench/latency-distribution.md"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
