package generator

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNewValidatesN(t *testing.T) {
	t.Parallel()

	// Arrange
	cases := []struct {
		name    string
		n       int
		wantErr bool
	}{
		{name: "zero rejected", n: 0, wantErr: true},
		{name: "negative rejected", n: -1, wantErr: true},
		{name: "seventeen rejected", n: 17, wantErr: true},
		{name: "five accepted (N_min interim)", n: 5, wantErr: false},
		{name: "fourteen accepted", n: 14, wantErr: false},
		{name: "sixteen accepted (upper bound)", n: 16, wantErr: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			g, err := New(tc.n, 1)

			// Assert
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for n=%d, got nil (generator=%v)", tc.n, g)
				}
				if !errors.Is(err, ErrNOutOfRange) {
					t.Errorf("expected ErrNOutOfRange for n=%d, got %v", tc.n, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for n=%d: %v", tc.n, err)
			}
			if g == nil {
				t.Fatal("expected non-nil generator")
			}
		})
	}
}

func TestNewValidatesMarksPerUnit(t *testing.T) {
	t.Parallel()

	// Arrange
	cases := []struct {
		name         string
		marksPerUnit int
		wantErr      bool
	}{
		{name: "zero rejected", marksPerUnit: 0, wantErr: true},
		{name: "three rejected", marksPerUnit: 3, wantErr: true},
		{name: "negative rejected", marksPerUnit: -1, wantErr: true},
		{name: "one accepted (standard)", marksPerUnit: 1, wantErr: false},
		{name: "two accepted (double)", marksPerUnit: 2, wantErr: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			g, err := New(8, tc.marksPerUnit)

			// Assert
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for marksPerUnit=%d, got nil (generator=%v)", tc.marksPerUnit, g)
				}
				if !errors.Is(err, ErrKUnsupported) {
					t.Errorf("expected ErrKUnsupported for marksPerUnit=%d, got %v", tc.marksPerUnit, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for marksPerUnit=%d: %v", tc.marksPerUnit, err)
			}
			if g == nil {
				t.Fatal("expected non-nil generator")
			}
		})
	}
}

func TestPuzzleJSONRoundTrip(t *testing.T) {
	t.Parallel()

	// Arrange
	original := Puzzle{
		N:            5,
		MarksPerUnit: 1,
		Regions: [][]int{
			{0, 0, 1, 1, 1},
			{0, 0, 1, 2, 2},
			{3, 0, 1, 2, 2},
			{3, 3, 4, 2, 2},
			{3, 3, 4, 4, 4},
		},
		Solution: []Mark{
			{Row: 0, Col: 0},
			{Row: 1, Col: 2},
			{Row: 2, Col: 4},
			{Row: 3, Col: 1},
			{Row: 4, Col: 3},
		},
		Difficulty: Hard,
		Metrics: Metrics{
			MaxTier:    3,
			TierCounts: []int{0, 5, 3, 2, 0},
			TraceLen:   10,
		},
	}

	// Act
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Puzzle
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Assert
	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round-trip mismatch:\n original = %+v\n decoded  = %+v", original, decoded)
	}

	emitted := string(data)
	requiredTags := []string{
		`"n"`,
		`"marks_per_unit"`,
		`"regions"`,
		`"solution"`,
		`"difficulty"`,
		`"metrics"`,
		`"max_tier"`,
		`"tier_counts"`,
		`"trace_len"`,
		`"r"`,
		`"c"`,
	}
	for _, tag := range requiredTags {
		if !strings.Contains(emitted, tag) {
			t.Errorf("emitted JSON missing tag %s: %s", tag, emitted)
		}
	}
}

func TestGenerateReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	// Arrange
	g, err := New(8, 1)
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	// Act
	_, genErr := g.Generate(context.Background())

	// Assert
	if genErr == nil {
		t.Fatal("expected Generate to return an error in this scaffold slice")
	}
	if !strings.Contains(genErr.Error(), "not implemented") {
		t.Errorf("expected error message to contain \"not implemented\", got %q", genErr.Error())
	}
}

func TestNMinConstant(t *testing.T) {
	t.Parallel()

	// Arrange / Act / Assert
	if NMin != 5 {
		t.Errorf("expected NMin == 5, got %d", NMin)
	}
}

func TestOptionsSetConfigFields(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		seed         int64 = 424242
		maxAttempts  int   = 7
		maxMutations int   = 99
	)

	// Act
	g, err := New(8, 2,
		WithSeed(seed),
		WithMaxAttempts(maxAttempts),
		WithMaxMutations(maxMutations),
		WithDifficulty(Expert),
	)
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	// Assert
	if g.cfg.seed != seed {
		t.Errorf("seed: expected %d, got %d", seed, g.cfg.seed)
	}
	if !g.cfg.seedSet {
		t.Error("seedSet: expected true after WithSeed, got false")
	}
	if g.cfg.maxAttempts != maxAttempts {
		t.Errorf("maxAttempts: expected %d, got %d", maxAttempts, g.cfg.maxAttempts)
	}
	if g.cfg.maxMutations != maxMutations {
		t.Errorf("maxMutations: expected %d, got %d", maxMutations, g.cfg.maxMutations)
	}
	if g.cfg.difficulty != Expert {
		t.Errorf("difficulty: expected %v, got %v", Expert, g.cfg.difficulty)
	}
}
