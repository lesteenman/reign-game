package generator

// classify returns the Difficulty bucket and Metrics summary for a solve
// trace. Thresholds come from design.md §8:
//
//   - MaxTier == 0 (no rule fired)           -> Easy (trivial by construction)
//   - MaxTier <= 1                           -> Easy
//   - MaxTier == 2                           -> Medium
//   - MaxTier == 3                           -> Hard
//   - MaxTier == 4                           -> Expert
//
// TierCounts is length-5 (indices 0..4; index 0 is unused by convention to
// keep tier numbers aligned with their slice index). TraceLen is the total
// number of rule events.
//
// Stable across runs for the same input trace: uses only counting and
// max — no RNG, no side effects.
func classify(trace ruleTrace) (Difficulty, Metrics) {
	tierCounts := make([]int, 5)
	maxTier := 0
	for _, ev := range trace {
		t := ruleTier[ev.Rule]
		if t < 1 || t > 4 {
			continue
		}
		tierCounts[t]++
		if t > maxTier {
			maxTier = t
		}
	}

	metrics := Metrics{
		MaxTier:    maxTier,
		TierCounts: tierCounts,
		TraceLen:   len(trace),
	}

	var d Difficulty
	switch {
	case maxTier <= 1:
		d = Easy
	case maxTier == 2:
		d = Medium
	case maxTier == 3:
		d = Hard
	default:
		d = Expert
	}
	return d, metrics
}
