package generator

import (
	"testing"
)

func TestComputeTargetSizes(t *testing.T) {
	tests := []struct {
		name     string
		gridSize int
		variance float64
		minSize  int
	}{
		{
			name:     "5x5 zero variance produces uniform sizes",
			gridSize: 5,
			variance: 0.0,
			minSize:  3,
		},
		{
			name:     "7x7 zero variance produces uniform sizes",
			gridSize: 7,
			variance: 0.0,
			minSize:  3,
		},
		{
			name:     "9x9 zero variance produces uniform sizes",
			gridSize: 9,
			variance: 0.0,
			minSize:  3,
		},
		{
			name:     "5x5 full variance respects constraints",
			gridSize: 5,
			variance: 1.0,
			minSize:  3,
		},
		{
			name:     "7x7 full variance respects constraints",
			gridSize: 7,
			variance: 1.0,
			minSize:  3,
		},
		{
			name:     "9x9 full variance respects constraints",
			gridSize: 9,
			variance: 1.0,
			minSize:  3,
		},
		{
			name:     "5x5 half variance respects constraints",
			gridSize: 5,
			variance: 0.5,
			minSize:  3,
		},
		{
			name:     "9x9 double queens minSize 4",
			gridSize: 9,
			variance: 0.5,
			minSize:  4,
		},
		{
			name:     "5x5 full variance with minSize 4",
			gridSize: 5,
			variance: 1.0,
			minSize:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			expectedTotal := tt.gridSize * tt.gridSize
			numRegions := tt.gridSize

			// Act
			sizes := computeTargetSizes(tt.gridSize, tt.variance, tt.minSize)

			// Assert
			if len(sizes) != numRegions {
				t.Fatalf("got %d sizes, want %d", len(sizes), numRegions)
			}

			total := 0
			for i, s := range sizes {
				if s < tt.minSize {
					t.Errorf("size[%d] = %d, below minSize %d", i, s, tt.minSize)
				}
				total += s
			}
			if total != expectedTotal {
				t.Errorf("total = %d, want %d", total, expectedTotal)
			}

			// At zero variance, all sizes must equal gridSize.
			if tt.variance == 0.0 {
				for i, s := range sizes {
					if s != tt.gridSize {
						t.Errorf("at variance=0.0: size[%d] = %d, want %d", i, s, tt.gridSize)
					}
				}
			}

			// At non-zero variance, check there is some variation — but only when
			// there is meaningful room (spare cells > gridSize). With minSize=4 on
			// a 5x5 grid there are only 5 spare cells, so uniform is possible.
			spareCells := tt.gridSize*tt.gridSize - tt.gridSize*tt.minSize
			if tt.variance >= 1.0 && spareCells > tt.gridSize {
				allSame := true
				for _, s := range sizes {
					if s != sizes[0] {
						allSame = false
						break
					}
				}
				if allSame {
					t.Errorf("at variance=%.1f with gridSize=%d: expected some variation, all sizes are %d",
						tt.variance, tt.gridSize, sizes[0])
				}
			}
		})
	}
}

func TestComputeTargetSizes_Deterministic(t *testing.T) {
	// Verify stability: run multiple times, all must satisfy invariants.
	for i := 0; i < 20; i++ {
		// Arrange
		gridSize := 7
		variance := 0.8
		minSz := 3

		// Act
		sizes := computeTargetSizes(gridSize, variance, minSz)

		// Assert
		total := 0
		for _, s := range sizes {
			if s < minSz {
				t.Fatalf("iteration %d: size %d below min %d", i, s, minSz)
			}
			total += s
		}
		if total != gridSize*gridSize {
			t.Fatalf("iteration %d: total %d != %d", i, total, gridSize*gridSize)
		}
	}
}

func TestBFSRegionGenerator_WithVariance(t *testing.T) {
	// Known 5x5 solution: markers at (0,2),(1,0),(2,3),(3,1),(4,4).
	solution := [][]bool{
		{false, false, true, false, false},
		{true, false, false, false, false},
		{false, false, false, true, false},
		{false, true, false, false, false},
		{false, false, false, false, true},
	}

	tests := []struct {
		name         string
		gridSize     int
		opts         RegionOpts
		wantUniform  bool
	}{
		{
			name:     "zero variance produces uniform regions",
			gridSize: 5,
			opts: RegionOpts{
				Variance: 0.0,
				MinSize:  3,
			},
			wantUniform: true,
		},
		{
			name:     "half variance produces valid variable regions",
			gridSize: 5,
			opts: RegionOpts{
				Variance: 0.5,
				MinSize:  3,
			},
			wantUniform: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			gen := NewBFSRegionGenerator()

			// Act — retry since BFS can fail stochastically.
			var regionMap [][]int
			var err error
			for attempt := 0; attempt < 100; attempt++ {
				regionMap, err = gen.GenerateRegions(solution, tt.gridSize, tt.opts)
				if err == nil {
					break
				}
			}

			// Assert
			if err != nil {
				t.Fatalf("GenerateRegions failed after 100 attempts: %v", err)
			}
			if valErr := ValidateRegionMap(regionMap, tt.gridSize); valErr != nil {
				t.Fatalf("ValidateRegionMap failed: %v", valErr)
			}

			// Each region must contain exactly one solution marker.
			regionMarkers := make(map[int]int)
			for r, row := range solution {
				for c, cell := range row {
					if cell {
						regionMarkers[regionMap[r][c]]++
					}
				}
			}
			for rid := 0; rid < tt.gridSize; rid++ {
				if regionMarkers[rid] != 1 {
					t.Errorf("region %d has %d markers, want 1", rid, regionMarkers[rid])
				}
			}

			// Check sizes for zero-variance case: all regions should be
			// close to gridSize (the BFS fallback can cause minor deviation).
			if tt.wantUniform {
				regionSizes := make(map[int]int)
				for _, row := range regionMap {
					for _, id := range row {
						regionSizes[id]++
					}
				}
				for rid := 0; rid < tt.gridSize; rid++ {
					diff := regionSizes[rid] - tt.gridSize
					if diff < 0 {
						diff = -diff
					}
					if diff > 2 {
						t.Errorf("at variance=0: region %d has %d cells, want ~%d (diff %d > 2)",
							rid, regionSizes[rid], tt.gridSize, diff)
					}
				}
			}
		})
	}
}

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
			name:      "valid 5x5 region map uniform sizes",
			regionMap: validRegionMap5x5,
			gridSize:  5,
			wantErr:   false,
		},
		{
			name: "valid 5x5 region map variable sizes",
			regionMap: [][]int{
				{0, 0, 0, 1, 1},
				{0, 0, 0, 1, 1},
				{2, 2, 0, 1, 1},
				{3, 2, 4, 4, 4},
				{3, 3, 3, 4, 4},
			},
			gridSize: 5,
			wantErr:  false,
		},
		{
			name: "valid 3x3 region map",
			regionMap: [][]int{
				{0, 0, 1},
				{0, 1, 1},
				{2, 2, 2},
			},
			gridSize: 3,
			wantErr:  false,
		},
		{
			// Region 4 has only 2 cells (below minimum of 3).
			// Counts: 0=8, 1=5, 2=5, 3=5, 4=2 → total=25
			name: "region below minimum size",
			regionMap: [][]int{
				{0, 0, 0, 1, 1},
				{0, 0, 0, 1, 1},
				{0, 0, 2, 2, 1},
				{3, 3, 2, 2, 4},
				{3, 3, 3, 2, 4},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "below minimum",
		},
		{
			name: "total cell count mismatch via wrong number of regions",
			regionMap: [][]int{
				{0, 0, 1, 1, 1},
				{0, 0, 1, 2, 2},
				{3, 3, 1, 2, 2},
				{3, 3, 3, 2, 2},
				{3, 3, 3, 2, 2},
			},
			gridSize: 5,
			wantErr:  true,
			errMsg:   "region count",
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
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestGenerateRegionMap(t *testing.T) {
	// Known 5x5 solution: markers at (0,2),(1,0),(2,3),(3,1),(4,4).
	gridSize := 5
	solution := [][]bool{
		{false, false, true, false, false},
		{true, false, false, false, false},
		{false, false, false, true, false},
		{false, true, false, false, false},
		{false, false, false, false, true},
	}

	// Run multiple times. GenerateRegionMap uses a randomized greedy algorithm
	// that can fail on some attempts; retry up to 20 times per iteration.
	for i := 0; i < 5; i++ {
		t.Run("iteration", func(t *testing.T) {
			var regionMap [][]int
			var err error
			for attempt := 0; attempt < 100; attempt++ {
				regionMap, err = GenerateRegionMap(solution, gridSize)
				if err == nil {
					break
				}
			}
			if err != nil {
				t.Fatalf("GenerateRegionMap failed after 100 attempts: %v", err)
			}

			// Must pass validation.
			if err := ValidateRegionMap(regionMap, gridSize); err != nil {
				t.Fatalf("ValidateRegionMap failed: %v", err)
			}

			// Each region must contain exactly one solution marker.
			regionMarkers := make(map[int]int)
			for r, row := range solution {
				for c, cell := range row {
					if cell {
						regionMarkers[regionMap[r][c]]++
					}
				}
			}
			for rid := 0; rid < gridSize; rid++ {
				if regionMarkers[rid] != 1 {
					t.Errorf("region %d has %d markers, want 1", rid, regionMarkers[rid])
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
