package generator

import (
	"context"
	"errors"
	"testing"
)

// propertyCorpusLiveRules lists the ruleIDs registered in defaultRuleset.
// Property 2 below asserts each one fires at least once across the corpus.
var propertyCorpusLiveRules = []ruleID{ruleR1, ruleR2, ruleR3, ruleR4, ruleR5, ruleR7}

// TestPropertyCorpus verifies two properties on a ~500-puzzle corpus
// drawn from the supported (N, k) grid:
//
//  1. Every generated puzzle's deductive solution equals its brute
//     solution (cross-check, repeated under load).
//  2. Every live rule (R1..R5, R7) fires at least once.
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

	for _, r := range propertyCorpusLiveRules {
		if ruleFired[r] > 0 {
			t.Logf("R%d fired %d times", int(r), ruleFired[r])
			continue
		}
		t.Errorf("rule R%d never fired across the corpus — redundant or buggy", int(r))
	}
}
