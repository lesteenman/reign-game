package generator

import (
	"reflect"
	"testing"
)

func TestPairSeedsK1Identity(t *testing.T) {
	t.Parallel()

	// Arrange
	marks := []Mark{
		{Row: 0, Col: 0},
		{Row: 1, Col: 2},
		{Row: 2, Col: 4},
		{Row: 3, Col: 1},
		{Row: 4, Col: 3},
	}

	// Act
	got := pairSeeds(marks, 5, 1)

	// Assert
	if len(got) != 5 {
		t.Fatalf("expected 5 seed groups, got %d", len(got))
	}
	for i, group := range got {
		if len(group) != 1 {
			t.Errorf("group %d: expected len 1, got %d", i, len(group))
			continue
		}
		if group[0] != marks[i] {
			t.Errorf("group %d: expected %v, got %v", i, marks[i], group[0])
		}
	}
}

func TestPairSeedsK2GreedyNearestNeighbor(t *testing.T) {
	t.Parallel()

	// Arrange — six marks arranged in three obvious near-neighbor pairs:
	// (0,0)-(0,1), (4,4)-(4,5), (8,8)-(9,8). Manhattan distance 1 each.
	// Any other pairing is worse; greedy must find the three closest pairs.
	marks := []Mark{
		{Row: 0, Col: 0},
		{Row: 8, Col: 8},
		{Row: 0, Col: 1},
		{Row: 4, Col: 4},
		{Row: 9, Col: 8},
		{Row: 4, Col: 5},
	}

	// Act
	got := pairSeeds(marks, 3, 2)

	// Assert
	if len(got) != 3 {
		t.Fatalf("expected 3 seed groups, got %d", len(got))
	}
	for i, group := range got {
		if len(group) != 2 {
			t.Errorf("group %d: expected len 2, got %d", i, len(group))
		}
		d := manhattan(group[0], group[1])
		if d != 1 {
			t.Errorf("group %d: expected distance 1 (greedy nearest), got %d — %v",
				i, d, group)
		}
	}

	// Every mark appears exactly once across all groups.
	seen := map[Mark]int{}
	for _, g := range got {
		for _, m := range g {
			seen[m]++
		}
	}
	for _, m := range marks {
		if seen[m] != 1 {
			t.Errorf("mark %v appears %d times, expected 1", m, seen[m])
		}
	}
}

func TestPairSeedsK2CountMatches(t *testing.T) {
	t.Parallel()

	// Arrange — sampler guarantees 2N marks with no two on the same row or col.
	// Use a hand-built N=9 solution approximated as a diagonal pattern.
	n := 9
	marks := make([]Mark, 0, 2*n)
	for i := 0; i < n; i++ {
		marks = append(marks,
			Mark{Row: i, Col: i},
			Mark{Row: (i + 3) % n, Col: (i + 6) % n},
		)
	}

	// Act
	got := pairSeeds(marks, n, 2)

	// Assert
	if len(got) != n {
		t.Fatalf("expected %d seed groups, got %d", n, len(got))
	}
	covered := 0
	for _, g := range got {
		if len(g) != 2 {
			t.Errorf("group size != 2: %v", g)
		}
		covered += len(g)
	}
	if covered != len(marks) {
		t.Errorf("covered %d marks, expected %d", covered, len(marks))
	}
}

func TestManhattan(t *testing.T) {
	t.Parallel()

	// Arrange
	cases := []struct {
		a, b Mark
		want int
	}{
		{Mark{0, 0}, Mark{0, 0}, 0},
		{Mark{0, 0}, Mark{3, 4}, 7},
		{Mark{5, 2}, Mark{1, 8}, 10},
		{Mark{7, 3}, Mark{7, 9}, 6},
	}

	for _, tc := range cases {
		// Act
		got := manhattan(tc.a, tc.b)
		// Assert
		if got != tc.want {
			t.Errorf("manhattan(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestPairSeedsK1PreservesInput(t *testing.T) {
	t.Parallel()

	// Arrange
	input := []Mark{{0, 0}, {1, 1}, {2, 2}}
	inputCopy := make([]Mark, len(input))
	copy(inputCopy, input)

	// Act
	_ = pairSeeds(input, 3, 1)

	// Assert — pair must not mutate the input slice.
	if !reflect.DeepEqual(input, inputCopy) {
		t.Errorf("pairSeeds mutated input: %v vs %v", input, inputCopy)
	}
}
