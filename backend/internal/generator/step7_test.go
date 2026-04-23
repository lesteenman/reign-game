package generator

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestStep7Gate measures the end-to-end Generate success rate across the
// supported (N, k) grid. PG-11's committed gate points are (N=12, k=1)
// and (N=9, k=2) at >=80%, both LIVE. Other combos are reported for
// visibility.
//
// This test is NOT run with -short.
func TestStep7Gate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Step 7 gate under -short")
	}
	t.Parallel()

	gatedCombos := []struct {
		n, k    int
		enforce bool
	}{
		{n: 9, k: 2, enforce: true},
		{n: 12, k: 1, enforce: true},
	}

	// These are reported for transparency.
	reportedCombos := []struct {
		n, k int
	}{
		{n: 6, k: 1},
		{n: 7, k: 1},
		{n: 8, k: 1},
		{n: 9, k: 1},
		{n: 10, k: 1},
		{n: 11, k: 1},
		{n: 10, k: 2},
		{n: 11, k: 2},
		{n: 12, k: 2},
	}

	const attempts = 100
	const gateRate = 0.80

	report := func(n, k, succeeded int) {
		t.Logf("N=%d k=%d: %d/%d succeeded (%.1f%%)",
			n, k, succeeded, attempts, 100*float64(succeeded)/float64(attempts))
	}

	for _, c := range gatedCombos {
		succ := runGenerateAttempts(t, c.n, c.k, attempts)
		report(c.n, c.k, succ)
		rate := float64(succ) / float64(attempts)
		if rate < gateRate {
			if c.enforce {
				t.Errorf("Step 7 gate FAILED at N=%d k=%d: rate %.1f%% < %.1f%%",
					c.n, c.k, 100*rate, 100*gateRate)
			} else {
				t.Logf("Step 7 gate BELOW THRESHOLD (non-enforcing) at N=%d k=%d: rate %.1f%% < %.1f%% — see R-066 comment for mutator follow-up",
					c.n, c.k, 100*rate, 100*gateRate)
			}
		}
	}

	for _, c := range reportedCombos {
		succ := runGenerateAttempts(t, c.n, c.k, attempts)
		report(c.n, c.k, succ)
	}
}

// runGenerateAttempts constructs a fresh generator and records how many
// attempts return a valid puzzle (not ErrMaxAttemptsExhausted and not an
// error). Seeded deterministically per (n, k) so re-runs are reproducible.
func runGenerateAttempts(t *testing.T, n, k, attempts int) int {
	t.Helper()

	// Use a generous per-call attempt cap; the gate is measuring
	// grow+mutate quality, not retry behavior.
	g, err := New(n, k, WithSeed(int64(1000*n+10*k+1)), WithMaxAttempts(10))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	succeeded := 0
	for i := 0; i < attempts; i++ {
		_, err := g.Generate(context.Background())
		if err == nil {
			succeeded++
			continue
		}
		if !errors.Is(err, ErrMaxAttemptsExhausted) {
			t.Fatalf("unexpected Generate error at N=%d k=%d attempt %d: %v",
				n, k, i, err)
		}
	}
	return succeeded
}

// TestGenerateDeterministic: same seed -> same puzzle (deep-equal on
// Regions, Solution, Difficulty).
func TestGenerateDeterministic(t *testing.T) {
	t.Parallel()

	// Arrange
	const n, k = 8, 1
	const seed int64 = 12345
	g1, _ := New(n, k, WithSeed(seed), WithMaxAttempts(20))
	g2, _ := New(n, k, WithSeed(seed), WithMaxAttempts(20))

	// Act
	p1, err1 := g1.Generate(context.Background())
	p2, err2 := g2.Generate(context.Background())

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatalf("Generate errors: %v / %v", err1, err2)
	}
	if !reflect.DeepEqual(p1.Regions, p2.Regions) {
		t.Errorf("Regions differ\n p1=%v\n p2=%v", p1.Regions, p2.Regions)
	}
	if !reflect.DeepEqual(p1.Solution, p2.Solution) {
		t.Errorf("Solution differs\n p1=%v\n p2=%v", p1.Solution, p2.Solution)
	}
	if p1.Difficulty != p2.Difficulty {
		t.Errorf("Difficulty differs: %v vs %v", p1.Difficulty, p2.Difficulty)
	}
}

// TestGenerateHonorsContextCancellation: a pre-cancelled ctx returns
// context.Canceled promptly.
func TestGenerateHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	// Arrange
	g, _ := New(12, 1, WithMaxAttempts(1000))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	// Act
	_, err := g.Generate(ctx)

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestGenerateMaxAttemptsExhausted: if the pipeline can never produce a
// valid puzzle within maxAttempts, Generate returns ErrMaxAttemptsExhausted.
// We force this by demanding Expert difficulty at N=6 (where Expert is
// essentially unreachable).
func TestGenerateMaxAttemptsExhausted(t *testing.T) {
	t.Parallel()

	// Arrange — Expert at the smallest supported N is vanishingly rare
	// because the tier-4 rules (R8, R9) are degenerate at k=1 and R6/R8
	// are subsumed by R3 (see design.md §4.2).
	g, err := New(6, 1,
		WithSeed(1),
		WithMaxAttempts(3),
		WithDifficulty(Expert),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Act
	_, genErr := g.Generate(context.Background())

	// Assert
	if !errors.Is(genErr, ErrMaxAttemptsExhausted) {
		t.Errorf("expected ErrMaxAttemptsExhausted, got %v", genErr)
	}
}

// TestGenerateDifficultyFilterMatches: when WithDifficulty(Easy) is set
// and Generate succeeds, the returned puzzle's Difficulty is Easy.
func TestGenerateDifficultyFilterMatches(t *testing.T) {
	t.Parallel()

	// Arrange
	g, err := New(9, 1,
		WithSeed(42),
		WithMaxAttempts(50),
		WithDifficulty(Easy),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Act
	p, genErr := g.Generate(context.Background())

	// Assert
	if errors.Is(genErr, ErrMaxAttemptsExhausted) {
		t.Skip("no Easy puzzle produced in 50 attempts — test stochastic, skipping")
	}
	if genErr != nil {
		t.Fatalf("Generate: %v", genErr)
	}
	if p.Difficulty != Easy {
		t.Errorf("expected Difficulty=Easy, got %v", p.Difficulty)
	}
}

// TestGenerateOutputShape: returned Regions is [N][N] with values in
// [0, N) and exactly N distinct region ids.
func TestGenerateOutputShape(t *testing.T) {
	t.Parallel()

	// Arrange
	const n, k = 8, 1
	g, err := New(n, k, WithSeed(77), WithMaxAttempts(20))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Act
	p, err := g.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Assert
	if len(p.Regions) != n {
		t.Fatalf("len(Regions) = %d, want %d", len(p.Regions), n)
	}
	seen := make(map[int]bool)
	for r := range n {
		if len(p.Regions[r]) != n {
			t.Errorf("len(Regions[%d]) = %d, want %d", r, len(p.Regions[r]), n)
		}
		for c := range n {
			v := p.Regions[r][c]
			if v < 0 || v >= n {
				t.Errorf("Regions[%d][%d] = %d out of [0, %d)", r, c, v, n)
			}
			seen[v] = true
		}
	}
	if len(seen) != n {
		t.Errorf("distinct region ids = %d, want %d", len(seen), n)
	}
}

// TestGenerateBruteSolutionMatches: the puzzle returned by Generate has a
// region map that brute-solves to exactly one solution, and that solution
// matches p.Solution.
func TestGenerateBruteSolutionMatches(t *testing.T) {
	t.Parallel()

	// Arrange
	const n, k = 8, 1
	g, err := New(n, k, WithSeed(88), WithMaxAttempts(20))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p, err := g.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Act
	sols, err := bruteSolveAll(p.Regions, n, k, 2)
	if err != nil {
		t.Fatalf("bruteSolveAll: %v", err)
	}

	// Assert
	if len(sols) != 1 {
		t.Fatalf("brute-solve returned %d solutions, want 1", len(sols))
	}
	if !marksEqualUnordered(sols[0], p.Solution) {
		t.Errorf("brute solution does not match Generate's Solution\n  brute: %v\n  gen:   %v",
			sols[0], p.Solution)
	}
}

func marksEqualUnordered(a, b []Mark) bool {
	if len(a) != len(b) {
		return false
	}
	ma := map[Mark]int{}
	for _, m := range a {
		ma[m]++
	}
	for _, m := range b {
		ma[m]--
		if ma[m] < 0 {
			return false
		}
	}
	return true
}
