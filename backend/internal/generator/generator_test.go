package generator

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	// Run multiple times to catch randomness issues.
	for i := 0; i < 5; i++ {
		t.Run("iteration", func(t *testing.T) {
			gridSize := 5
			puzzle, err := Generate(gridSize, 30*time.Second)
			if err != nil {
				t.Fatalf("Generate returned error: %v", err)
			}

			// GridSize matches.
			if puzzle.GridSize != gridSize {
				t.Errorf("GridSize = %d, want %d", puzzle.GridSize, gridSize)
			}

			// Mode is "standard".
			if puzzle.Mode != "standard" {
				t.Errorf("Mode = %q, want %q", puzzle.Mode, "standard")
			}

			// ID is empty (handler sets it).
			if puzzle.ID != "" {
				t.Errorf("ID = %q, want empty string", puzzle.ID)
			}

			// RegionMap passes validation.
			if err := ValidateRegionMap(puzzle.RegionMap, gridSize); err != nil {
				t.Fatalf("ValidateRegionMap failed: %v", err)
			}

			// Solution has exactly gridSize markers.
			markerCount := 0
			for _, row := range puzzle.Solution {
				for _, cell := range row {
					if cell {
						markerCount++
					}
				}
			}
			if markerCount != gridSize {
				t.Errorf("solution has %d markers, want %d", markerCount, gridSize)
			}

			// Exactly one solution (unique).
			solCount := CountSolutions(puzzle.RegionMap, gridSize)
			if solCount != 1 {
				t.Errorf("CountSolutions = %d, want 1", solCount)
			}

			// Each region contains exactly one solution marker.
			regionMarkers := make(map[int]int)
			for r, row := range puzzle.Solution {
				for c, cell := range row {
					if cell {
						rid := puzzle.RegionMap[r][c]
						regionMarkers[rid]++
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

func TestGenerateTimeout(t *testing.T) {
	_, err := Generate(5, 1*time.Nanosecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
