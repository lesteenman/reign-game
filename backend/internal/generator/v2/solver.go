package generator

// Outcome is the result of one deductive solve.
type Outcome int

const (
	// OutcomeStalled means the fixed-point loop ran to completion with no
	// rule firing in the last full pass, but the puzzle is not yet solved.
	// The mutator (R-065) reacts to Stalled by trying boundary swaps.
	OutcomeStalled Outcome = iota
	// OutcomeSolved means every row, column, and region has zero need; the
	// marks field holds the full solution.
	OutcomeSolved
	// OutcomeContradiction means some unit's need exceeds its remaining
	// candidates, proving the current partial state is infeasible. The
	// orchestrator restarts from the sampler.
	OutcomeContradiction
)

// solve runs the deductive rules to fixed point with the full R1..R9
// registry. Returns the outcome. The rule trace on s.trace (if recording) is
// left at its final state.
func solve(s *solverState) Outcome {
	return solveWith(s, defaultRuleset())
}

// solveWith runs the deductive rules to fixed point with the given ruleset.
// Used by rules_test.go's necessity fixtures — they pass a pruned ruleset
// (all rules minus rule X) and assert the solver now stalls or reaches
// contradiction.
//
// Algorithm: apply rules in tier order. On ANY change in ANY tier, restart
// from Tier 1. Fixed point = one full pass over tiers 1..4 with no changes.
// After each change check for contradiction; on contradiction, return
// immediately. After the fixed point, check for solved; else stalled.
//
// `rs` is passed by pointer to avoid a 96-byte value copy on every call
// (ruleset contains four []ruleFunc slices — the value copy duplicates
// the three-word slice headers for each).
func solveWith(s *solverState, rs *ruleset) Outcome {
	if s.contradicts() {
		return OutcomeContradiction
	}

	for {
		changed := false

		// Tier 1.
		for _, rule := range rs.tier1 {
			if rule(s) {
				changed = true
				if s.contradicts() {
					return OutcomeContradiction
				}
			}
		}
		if changed {
			continue
		}

		// Tier 2.
		for _, rule := range rs.tier2 {
			if rule(s) {
				changed = true
				if s.contradicts() {
					return OutcomeContradiction
				}
			}
		}
		if changed {
			continue
		}

		// Tier 3.
		for _, rule := range rs.tier3 {
			if rule(s) {
				changed = true
				if s.contradicts() {
					return OutcomeContradiction
				}
			}
		}
		if changed {
			continue
		}

		// Tier 4.
		for _, rule := range rs.tier4 {
			if rule(s) {
				changed = true
				if s.contradicts() {
					return OutcomeContradiction
				}
			}
		}
		if !changed {
			break
		}
	}

	if s.solved() {
		return OutcomeSolved
	}
	return OutcomeStalled
}
