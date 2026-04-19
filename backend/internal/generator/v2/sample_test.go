package generator

import (
	"fmt"
	"testing"
)

// assertValidSolution checks the sampler's post-conditions: exactly n*k marks,
// exactly k per row, exactly k per column, no two marks 8-neighbor adjacent,
// no duplicate (r, c) coordinates.
func assertValidSolution(t *testing.T, marks []Mark, n, k int) {
	t.Helper()

	if len(marks) != n*k {
		t.Fatalf("expected %d marks, got %d: %+v", n*k, len(marks), marks)
	}

	rowCount := make([]int, n)
	colCount := make([]int, n)
	seen := make(map[[2]int]bool, len(marks))
	for _, m := range marks {
		if m.Row < 0 || m.Row >= n || m.Col < 0 || m.Col >= n {
			t.Fatalf("mark out of range: %+v (n=%d)", m, n)
		}
		key := [2]int{m.Row, m.Col}
		if seen[key] {
			t.Fatalf("duplicate mark at %+v", m)
		}
		seen[key] = true
		rowCount[m.Row]++
		colCount[m.Col]++
	}
	for r := range n {
		if rowCount[r] != k {
			t.Errorf("row %d has %d marks, expected %d", r, rowCount[r], k)
		}
	}
	for c := range n {
		if colCount[c] != k {
			t.Errorf("column %d has %d marks, expected %d", c, colCount[c], k)
		}
	}
	for i := range marks {
		for j := i + 1; j < len(marks); j++ {
			rd := marks[i].Row - marks[j].Row
			if rd < 0 {
				rd = -rd
			}
			cd := marks[i].Col - marks[j].Col
			if cd < 0 {
				cd = -cd
			}
			if rd <= 1 && cd <= 1 {
				t.Errorf("marks %+v and %+v are 8-neighbor adjacent", marks[i], marks[j])
			}
		}
	}
}

// minFeasibleN is the empirical per-k lower bound below which the sampler
// is genuinely unsatisfiable (no valid N*k-mark placement exists under the
// row/col/adjacency constraints). Values verified by TestFeasibility.
//
// The package constant NMin is 5; for k=2 the constraints tighten such that
// N < 8 has no solution. The constraint test uses max(NMin, minFeasibleN[k]).
var minFeasibleN = map[int]int{
	1: NMin,
	2: 8,
}

// TestSampleConstraints asserts that 200 samples per (N, k) satisfy every
// sampler invariant. N ranges over [max(NMin, minFeasibleN[k]), 14]; k over
// {1, 2}.
func TestSampleConstraints(t *testing.T) {
	t.Parallel()

	const samplesPerCase = 200

	for _, k := range []int{1, 2} {
		nFrom := minFeasibleN[k]
		for n := nFrom; n <= 14; n++ {
			n, k := n, k
			name := fmt.Sprintf("N=%d/k=%d", n, k)
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				// Arrange
				g, err := New(n, k, WithSeed(int64(n*100+k)))
				if err != nil {
					t.Fatalf("New(%d, %d) failed: %v", n, k, err)
				}

				// Act / Assert — repeated sampling.
				for i := 0; i < samplesPerCase; i++ {
					marks, ok := g.sampleSolution()
					if !ok {
						t.Fatalf("sample %d: sampleSolution returned false for (N=%d, k=%d)", i, n, k)
					}
					assertValidSolution(t, marks, n, k)
				}
			})
		}
	}
}

// TestSampleDistinct sanity-checks that the sampler produces diverse output —
// not a proof of uniform sampling, just a cheap check that row-order and
// combo shuffling both do something.
func TestSampleDistinct(t *testing.T) {
	t.Parallel()

	// Arrange
	g, err := New(8, 1, WithSeed(42))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Act
	seen := make(map[string]int)
	const samples = 50
	for i := 0; i < samples; i++ {
		marks, ok := g.sampleSolution()
		if !ok {
			t.Fatalf("sample %d: sampleSolution returned false", i)
		}
		key := fingerprint(marks)
		seen[key]++
	}

	// Assert
	if len(seen) < samples/4 {
		t.Errorf("expected diverse samples, got %d distinct out of %d", len(seen), samples)
	}
}

func fingerprint(marks []Mark) string {
	var b [256]byte
	out := b[:0]
	for _, m := range marks {
		out = append(out, byte(m.Row), byte(m.Col), ';')
	}
	return string(out)
}

func BenchmarkSolutionSample(b *testing.B) {
	cases := []struct {
		n int
		k int
	}{
		{n: 14, k: 1},
		{n: 14, k: 2},
	}
	for _, tc := range cases {
		b.Run(fmt.Sprintf("N=%d/k=%d", tc.n, tc.k), func(b *testing.B) {
			g, err := New(tc.n, tc.k, WithSeed(1))
			if err != nil {
				b.Fatalf("New failed: %v", err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				marks, ok := g.sampleSolution()
				if !ok {
					b.Fatal("sampleSolution returned false")
				}
				if len(marks) != tc.n*tc.k {
					b.Fatalf("expected %d marks, got %d", tc.n*tc.k, len(marks))
				}
			}
		})
	}
}
