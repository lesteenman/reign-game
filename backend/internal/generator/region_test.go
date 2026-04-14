package generator

import (
	"testing"
)

// validRegionMap5x5 is the reference 5x5 region map used across tests.
// Each region has exactly 5 contiguous cells. Unique solution: (0,2),(1,0),(2,3),(3,1),(4,4).
var validRegionMap5x5 = [][]int{
	{3, 3, 2, 2, 0},
	{3, 2, 2, 0, 0},
	{3, 4, 2, 0, 1},
	{3, 4, 4, 0, 1},
	{4, 4, 1, 1, 1},
}

func TestValidateRegionMap(t *testing.T) {
	tests := []struct {
		name      string
		regionMap [][]int
		gridSize  int
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid 5x5 region map",
			regionMap: validRegionMap5x5,
			gridSize:  5,
			wantErr:   false,
		},
		{
			name: "region with wrong cell count",
			regionMap: [][]int{
				{0, 0, 1, 1, 1},
				{0, 0, 1, 2, 2},
				{3, 3, 1, 2, 2},
				{3, 4, 4, 4, 2},
				{3, 3, 4, 4, 4},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "cell count",
		},
		{
			name: "non-contiguous region",
			regionMap: [][]int{
				{0, 1, 1, 1, 1},
				{2, 2, 1, 3, 3},
				{2, 2, 0, 3, 3},
				{4, 4, 4, 4, 3},
				{4, 0, 0, 0, 4},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "not contiguous",
		},
		{
			name: "out-of-range region ID",
			regionMap: [][]int{
				{3, 3, 2, 2, 0},
				{3, 2, 2, 0, 0},
				{3, 4, 2, 0, 5},
				{3, 4, 4, 0, 1},
				{4, 4, 1, 1, 1},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "out of range",
		},
		{
			name: "wrong grid dimensions - too few rows",
			regionMap: [][]int{
				{3, 3, 2, 2, 0},
				{3, 2, 2, 0, 0},
				{3, 4, 2, 0, 1},
				{3, 4, 4, 0, 1},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "row count",
		},
		{
			name: "wrong grid dimensions - wrong column count",
			regionMap: [][]int{
				{3, 3, 2, 2},
				{3, 2, 2, 0},
				{3, 4, 2, 0},
				{3, 4, 4, 0},
				{4, 4, 1, 1},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "column count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegionMap(tt.regionMap, tt.gridSize)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
