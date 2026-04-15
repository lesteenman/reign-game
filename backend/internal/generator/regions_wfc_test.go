package generator

import (
	"testing"
)

// test5x5Solution is a known valid 5x5 solution: markers at (0,2),(1,0),(2,3),(3,1),(4,4).
var test5x5Solution = [][]bool{
	{false, false, true, false, false},
	{true, false, false, false, false},
	{false, false, false, true, false},
	{false, true, false, false, false},
	{false, false, false, false, true},
}

func TestWFCRegionGenerator_Correctness(t *testing.T) {
	tests := []struct {
		name     string
		gridSize int
		solution [][]bool
		opts     RegionOpts
	}{
		{
			name:     "5x5 uniform regions",
			gridSize: 5,
			solution: test5x5Solution,
			opts:     RegionOpts{Variance: 0.0, MinSize: 3},
		},
		{
			name:     "5x5 variable regions with half variance",
			gridSize: 5,
			solution: test5x5Solution,
			opts:     RegionOpts{Variance: 0.5, MinSize: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			gen := NewWFCRegionGenerator()

			// Act
			regionMap, err := gen.GenerateRegions(tt.solution, tt.gridSize, tt.opts)

			// Assert
			if err != nil {
				t.Fatalf("GenerateRegions failed: %v", err)
			}
			if valErr := ValidateRegionMap(regionMap, tt.gridSize); valErr != nil {
				t.Fatalf("ValidateRegionMap failed: %v", valErr)
			}
		})
	}
}

func TestWFCRegionGenerator_EachRegionContainsMarker(t *testing.T) {
	// Arrange
	gen := NewWFCRegionGenerator()
	gridSize := 5
	opts := RegionOpts{Variance: 0.0, MinSize: 3}

	// Act
	regionMap, err := gen.GenerateRegions(test5x5Solution, gridSize, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateRegions failed: %v", err)
	}

	regionMarkers := make(map[int]int)
	for r, row := range test5x5Solution {
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
}

func TestWFCRegionGenerator_VariableSizes(t *testing.T) {
	// Arrange
	gen := NewWFCRegionGenerator()
	gridSize := 5
	opts := RegionOpts{Variance: 0.5, MinSize: 3}

	// Act
	regionMap, err := gen.GenerateRegions(test5x5Solution, gridSize, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateRegions failed: %v", err)
	}
	if valErr := ValidateRegionMap(regionMap, gridSize); valErr != nil {
		t.Fatalf("ValidateRegionMap failed: %v", valErr)
	}

	// Count region sizes — all must be >= minSize.
	regionSizes := make(map[int]int)
	for _, row := range regionMap {
		for _, id := range row {
			regionSizes[id]++
		}
	}
	for rid := 0; rid < gridSize; rid++ {
		if regionSizes[rid] < opts.MinSize {
			t.Errorf("region %d has %d cells, below minSize %d", rid, regionSizes[rid], opts.MinSize)
		}
	}
}

func TestWFCRegionGenerator_MultipleIterations(t *testing.T) {
	// Arrange
	gen := NewWFCRegionGenerator()
	gridSize := 5
	opts := RegionOpts{Variance: 0.0, MinSize: 3}

	// Act & Assert — run 10 iterations, all must produce valid maps.
	for i := 0; i < 10; i++ {
		regionMap, err := gen.GenerateRegions(test5x5Solution, gridSize, opts)
		if err != nil {
			t.Fatalf("iteration %d: GenerateRegions failed: %v", i, err)
		}
		if valErr := ValidateRegionMap(regionMap, gridSize); valErr != nil {
			t.Fatalf("iteration %d: ValidateRegionMap failed: %v", i, valErr)
		}
	}
}

func TestWFCRegionGenerator_7x7(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 7x7 WFC test in short mode")
	}

	// Arrange — build a valid 7x7 solution with one marker per row, no shared column,
	// no diagonal adjacency. Markers at: (0,1),(1,3),(2,5),(3,0),(4,2),(5,4),(6,6).
	gridSize := 7
	solution := make([][]bool, gridSize)
	markerCols := []int{1, 3, 5, 0, 2, 4, 6}
	for r := 0; r < gridSize; r++ {
		solution[r] = make([]bool, gridSize)
		solution[r][markerCols[r]] = true
	}
	gen := NewWFCRegionGenerator()
	opts := RegionOpts{Variance: 0.0, MinSize: 3}

	// Act
	regionMap, err := gen.GenerateRegions(solution, gridSize, opts)

	// Assert
	if err != nil {
		t.Fatalf("GenerateRegions failed: %v", err)
	}
	if valErr := ValidateRegionMap(regionMap, gridSize); valErr != nil {
		t.Fatalf("ValidateRegionMap failed: %v", valErr)
	}
}

func BenchmarkWFCRegionGenerator_5x5(b *testing.B) {
	gen := NewWFCRegionGenerator()
	opts := RegionOpts{Variance: 0.0, MinSize: 3}
	for b.Loop() {
		_, _ = gen.GenerateRegions(test5x5Solution, 5, opts)
	}
}

func BenchmarkBFSRegionGenerator_5x5(b *testing.B) {
	gen := NewBFSRegionGenerator()
	opts := RegionOpts{Variance: 0.0, MinSize: 3}
	for b.Loop() {
		_, _ = gen.GenerateRegions(test5x5Solution, 5, opts)
	}
}

func BenchmarkWFCRegionGenerator_7x7(b *testing.B) {
	gridSize := 7
	solution := make([][]bool, gridSize)
	markerCols := []int{1, 3, 5, 0, 2, 4, 6}
	for r := 0; r < gridSize; r++ {
		solution[r] = make([]bool, gridSize)
		solution[r][markerCols[r]] = true
	}
	gen := NewWFCRegionGenerator()
	opts := RegionOpts{Variance: 0.0, MinSize: 3}
	for b.Loop() {
		_, _ = gen.GenerateRegions(solution, gridSize, opts)
	}
}

func BenchmarkBFSRegionGenerator_7x7(b *testing.B) {
	gridSize := 7
	solution := make([][]bool, gridSize)
	markerCols := []int{1, 3, 5, 0, 2, 4, 6}
	for r := 0; r < gridSize; r++ {
		solution[r] = make([]bool, gridSize)
		solution[r][markerCols[r]] = true
	}
	gen := NewBFSRegionGenerator()
	opts := RegionOpts{Variance: 0.0, MinSize: 3}
	for b.Loop() {
		_, _ = gen.GenerateRegions(solution, gridSize, opts)
	}
}
