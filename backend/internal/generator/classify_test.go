package generator

import (
	"reflect"
	"testing"
)

func TestClassifyBuckets(t *testing.T) {
	t.Parallel()

	// Arrange
	cases := []struct {
		name  string
		trace ruleTrace
		want  Difficulty
	}{
		{
			name:  "empty trace -> Easy (trivially solved)",
			trace: ruleTrace{},
			want:  Easy,
		},
		{
			name: "only tier 1 rules -> Easy",
			trace: ruleTrace{
				{Rule: ruleR1},
				{Rule: ruleR2},
				{Rule: ruleR3},
			},
			want: Easy,
		},
		{
			name: "tier 1 + tier 2 -> Medium",
			trace: ruleTrace{
				{Rule: ruleR1},
				{Rule: ruleR4},
			},
			want: Medium,
		},
		{
			name: "tier 1 + tier 3 -> Hard",
			trace: ruleTrace{
				{Rule: ruleR1},
				{Rule: ruleR7},
			},
			want: Hard,
		},
		{
			name: "tier 4 -> Expert",
			trace: ruleTrace{
				{Rule: ruleR1},
				{Rule: ruleR9},
			},
			want: Expert,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			d, _ := classify(tc.trace)

			// Assert
			if d != tc.want {
				t.Errorf("got %v, want %v", d, tc.want)
			}
		})
	}
}

func TestClassifyMetrics(t *testing.T) {
	t.Parallel()

	// Arrange
	trace := ruleTrace{
		{Rule: ruleR1}, // tier 1
		{Rule: ruleR1}, // tier 1
		{Rule: ruleR4}, // tier 2
		{Rule: ruleR7}, // tier 3
		{Rule: ruleR9}, // tier 4
		{Rule: ruleR9}, // tier 4
	}

	// Act
	_, m := classify(trace)

	// Assert
	if m.MaxTier != 4 {
		t.Errorf("MaxTier = %d, want 4", m.MaxTier)
	}
	if m.TraceLen != 6 {
		t.Errorf("TraceLen = %d, want 6", m.TraceLen)
	}
	wantCounts := []int{0, 2, 1, 1, 2}
	if !reflect.DeepEqual(m.TierCounts, wantCounts) {
		t.Errorf("TierCounts = %v, want %v", m.TierCounts, wantCounts)
	}
}

func TestClassifyStable(t *testing.T) {
	t.Parallel()

	// Arrange — same trace classified twice must be identical.
	trace := ruleTrace{
		{Rule: ruleR1},
		{Rule: ruleR3},
		{Rule: ruleR5},
	}

	// Act
	d1, m1 := classify(trace)
	d2, m2 := classify(trace)

	// Assert
	if d1 != d2 {
		t.Errorf("Difficulty not stable: %v vs %v", d1, d2)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("Metrics not stable: %v vs %v", m1, m2)
	}
}
