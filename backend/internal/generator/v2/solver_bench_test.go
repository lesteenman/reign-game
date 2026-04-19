package generator

import (
	"testing"
)

// BenchmarkSolverFixedPoint measures one fixed-point pass of the deductive
// solver at a representative "typical mid-generation state."
//
// Setup: N=14 k=2 row-stripes region map (row r is region r). We pre-place
// N-k marks (one mark short of a full row) in a specific pattern that
// makes the remaining state solvable by deductive rules. The benchmark
// measures one solve() call from this partial state.
//
// Per PG-06: target <50 µs/op, zero allocations after warm-up. On N=14
// the typical state is large (14×14 grid, 14 regions of 14 cells each);
// this benchmark flags pathological regressions.
func BenchmarkSolverFixedPoint(b *testing.B) {
	// Build a mid-generation state: N=14 k=1 row-stripes, all row marks
	// placed except row 0's. Row 0 then needs R3 + R1 + R2 to drive the
	// final placement. The state isn't solvable from empty, but it is
	// solvable given the pre-placed marks — making the benchmark measure
	// a realistic "typical" solver workload.
	const n = 14
	const k = 1
	rm := make([][]int, n)
	for r := range rm {
		rm[r] = make([]int, n)
		for c := range rm[r] {
			rm[r][c] = r
		}
	}

	// Pre-compute the marks that would put this state in a mid-solve
	// configuration. For row-stripes, any valid placement puts 1 mark
	// per row at a column such that no two are 8-adjacent. We use a
	// knight-step pattern: (r, (r*3) mod n) — rough enough that
	// adjacent rows rarely conflict but works for N=14.
	preMarks := make([]Mark, 0, n-1)
	for r := 0; r < n-1; r++ {
		preMarks = append(preMarks, Mark{Row: r, Col: (r * 3) % n})
	}

	// Allocate solverState once; reset before each iteration.
	s := newSolverState(rm, n, k)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s.reset()
		if err := s.initFromRegionMap(rm, n, k); err != nil {
			b.Fatalf("initFromRegionMap: %v", err)
		}
		applyR1AroundMarks(s, preMarks, n)
		for _, m := range preMarks {
			s.placeMark(m.Row, m.Col)
		}
		b.StartTimer()
		_ = solve(s)
	}
}
