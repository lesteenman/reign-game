//go:build diag

package generator

import (
	"math/bits"
	"testing"
)

// TestDiagPipelineStages breaks down where attempts fail at a given (N, k).
// Categorizes each attempt as: sample failure, grow failure, mutator failure,
// brute-non-unique, or full success.
//
// Build-tag gated so it never runs in default CI (200+ attempts × 3 cells
// × two passes is measurable wall-clock). Invoke explicitly when tuning:
//
//	go test -tags=diag -run TestDiagPipelineStages -v ./internal/generator/...
func TestDiagPipelineStages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping diag under -short")
	}
	t.Parallel()

	cases := []struct {
		n, k int
	}{
		{n: 9, k: 1},
		{n: 12, k: 1},
		{n: 9, k: 2},
	}

	const attempts = 200

	for _, c := range cases {
		c := c
		t.Run(namef(c.n, c.k), func(t *testing.T) {
			t.Parallel()

			g, err := New(c.n, c.k, WithSeed(int64(5000*c.n+c.k)), WithMaxAttempts(attempts))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			var sampleFail, growFail, mutateFail, bruteAmbiguous, success int

			for i := 0; i < attempts; i++ {
				marks, ok := g.sampleSolution()
				if !ok {
					sampleFail++
					continue
				}
				seeds := pairSeeds(marks, c.n, c.k)
				if !g.growRegions(seeds, &g.regionOf) {
					growFail++
					continue
				}
				outcome := g.solveAndMutate(seeds)
				if outcome != mutationSolved {
					mutateFail++
					continue
				}
				rm := convertRegionsToSlices(&g.regionOf, c.n)
				sols, err := bruteSolveAll(rm, c.n, c.k, 2)
				if err != nil {
					mutateFail++
					continue
				}
				if len(sols) != 1 {
					bruteAmbiguous++
					continue
				}
				success++
			}

			t.Logf("N=%d k=%d: sampleFail=%d growFail=%d mutateFail=%d bruteAmbig=%d success=%d",
				c.n, c.k, sampleFail, growFail, mutateFail, bruteAmbiguous, success)

			// Re-run, tracking how close-to-solved the mutator gets on
			// failed attempts. Useful to understand if we're stuck at
			// 90% solved or 10% solved.
			g2, _ := New(c.n, c.k, WithSeed(int64(5000*c.n+c.k+9)), WithMaxAttempts(attempts))
			var solvedDistSum, solvedDistCount int
			for i := 0; i < 50; i++ {
				marks, ok := g2.sampleSolution()
				if !ok {
					continue
				}
				seeds := pairSeeds(marks, c.n, c.k)
				if !g2.growRegions(seeds, &g2.regionOf) {
					continue
				}
				rm := convertRegionsToSlices(&g2.regionOf, c.n)
				g2.solver.trace = nil
				_ = g2.solver.initFromRegionMap(rm, c.n, c.k)
				_ = solve(&g2.solver)
				initScore := countSolvedCells(&g2.solver)
				_ = g2.solveAndMutate(seeds)
				finalScore := countSolvedCells(&g2.solver)
				solvedDistSum += finalScore - initScore
				solvedDistCount++
				totalMarks := 0
				for r := range g2.n {
					totalMarks += bits.OnesCount16(g2.solver.marks[r])
				}
				_ = totalMarks
			}
			if solvedDistCount > 0 {
				t.Logf("  average score improvement from mutation: %.2f / %d cells",
					float64(solvedDistSum)/float64(solvedDistCount), c.n*c.n)
			}
		})
	}
}
