package generator

import "math/bits"

// reset zeroes the used portion of the state for re-use across Generate
// calls. The state retains its backing arrays; only the effectively-used
// slices ([:s.n]) need wiping.
func (s *solverState) reset() {
	for r := range nMax {
		s.cands[r] = 0
		s.marks[r] = 0
		s.rowNeed[r] = 0
		s.colNeed[r] = 0
		s.regNeed[r] = 0
		for g := range nMax {
			s.regCellsByRow[g][r] = 0
			s.regOf[r][g] = 0
		}
	}
	s.n = 0
	s.k = 0
	if s.trace != nil {
		s.trace = s.trace[:0]
	}
}

// solutionMarks is a convenience wrapper that allocates a fresh []Mark.
// Prefer appendSolutionMarks in any hot path; this is for tests + one-off
// debug callers where the extra allocation does not matter.
func (s *solverState) solutionMarks() []Mark {
	total := 0
	for r := range s.n {
		total += bits.OnesCount16(s.marks[r])
	}
	return s.appendSolutionMarks(make([]Mark, 0, total))
}
