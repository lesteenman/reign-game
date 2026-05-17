package generator

import (
	"context"
	"errors"
	"testing"
)

// propertyCorpusKnownDead lists ruleIDs the corpus is currently
// unable to exercise. Tracked: R-06D (dead-rule investigation,
// ROADMAP.md Phase 6b — deferred to the audit-loop phase).
//
// An entry here is NOT a blanket relaxation — it degrades a zero-
// firing rule from an error to a warning so a real regression
// elsewhere still surfaces. Each entry's resolution is either a hand-
// crafted fixture demonstrating the rule fires, or deletion of the
// rule.
//
//   - R6 (Locked k-set in line, Tier 3): 500-puzzle corpus yields zero
//     firings. R7 appears to dominate the Tier 3 space. Possibly
//     redundant; possibly a classifier subtlety not captured by the
//     current rule ordering.
//   - R8 (K-line subset / X-wing, Tier 4): Tier 4 only fires on
//     Expert puzzles, and the corpus' (N=12, k=2) bucket is too thin
//     to guarantee one. Re-check after R-068d quantifies Expert yield.
//   - R9 (Region pair exclusion, Tier 4): same story as R8.
var propertyCorpusKnownDead = map[ruleID]string{
	ruleR6: "R7 appears to dominate Tier 3 — confirm or retire",
	ruleR8: "Tier 4; Expert yield too low in current corpus to exercise",
	ruleR9: "Tier 4; Expert yield too low in current corpus to exercise",
}

// TestPropertyCorpus verifies two properties on a ~500-puzzle corpus
// drawn from the supported (N, k) grid:
//
//  1. Every generated puzzle's deductive solution equals its brute
//     solution (cross-check, repeated under load).
//  2. Every rule in R1..R9 fires at least once, unless it is listed
//     in propertyCorpusKnownDead.
//
// Runs in the default suite; skips under -short (~5 min wall-clock).
func TestPropertyCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property corpus under -short")
	}
	t.Parallel()

	type bucket struct {
		n, k, samples int
	}
	buckets := []bucket{
		{n: 9, k: 1, samples: 120},
		{n: 11, k: 1, samples: 120},
		{n: 12, k: 1, samples: 60},
		{n: 9, k: 2, samples: 120},
		{n: 11, k: 2, samples: 60},
		{n: 12, k: 2, samples: 20},
	}

	var ruleFired [10]int // index by ruleID (1..9)

	// Hoisted solver state — initFromRegionMap zeroes every field, so
	// reuse across iterations is safe and avoids ~N_samples allocations.
	var s solverState
	trace := make(ruleTrace, 0, 256)

	for _, b := range buckets {
		g, err := New(b.n, b.k, WithSeed(int64(800*b.n+b.k)), WithMaxAttempts(20))
		if err != nil {
			t.Fatalf("New %d/%d: %v", b.n, b.k, err)
		}
		ok := 0
		for i := 0; i < b.samples; i++ {
			p, err := g.Generate(context.Background())
			if err != nil {
				if !errors.Is(err, ErrMaxAttemptsExhausted) {
					t.Fatalf("N=%d k=%d sample %d: %v", b.n, b.k, i, err)
				}
				continue
			}
			label := fmtSample(b.n, b.k, i)

			// Property 1: deductive == brute.
			assertDeductiveBruteAgree(t, label, &p)

			// Property 2: collect rule firings. Re-solve with trace
			// enabled (Generate records the trace internally but does
			// not export it; we re-run on a trace-enabled state here).
			if err := s.initFromRegionMap(p.Regions, p.N, p.MarksPerUnit); err != nil {
				t.Fatalf("%s: initFromRegionMap: %v", label, err)
			}
			s.trace = trace[:0]
			if outcome := solve(&s); outcome != OutcomeSolved {
				t.Fatalf("%s: solver outcome=%v on generated puzzle",
					label, outcome)
			}
			for _, ev := range s.trace {
				if ev.Rule >= 1 && ev.Rule <= 9 {
					ruleFired[ev.Rule]++
				}
			}
			trace = s.trace // cache back the possibly-grown backing slice
			ok++
		}
		t.Logf("N=%d k=%d: %d/%d ok", b.n, b.k, ok, b.samples)
	}

	for r := ruleID(1); r <= 9; r++ {
		switch {
		case ruleFired[r] > 0:
			t.Logf("R%d fired %d times", int(r), ruleFired[r])
		case propertyCorpusKnownDead[r] != "":
			t.Logf("R%d never fired (known-dead: %s)", int(r), propertyCorpusKnownDead[r])
		default:
			t.Errorf("rule R%d never fired across the corpus — redundant or buggy",
				int(r))
		}
	}
}
