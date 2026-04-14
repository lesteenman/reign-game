package generator

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// Generate creates a random puzzle with the given grid size that has exactly
// one solution. It retries until a unique puzzle is found or the timeout expires.
func Generate(gridSize int, timeout time.Duration) (*model.Puzzle, error) {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle: timeout after %v", timeout)
		}

		solution := generateSolution(gridSize)
		if solution == nil {
			continue
		}

		regionMap, err := GenerateRegionMap(solution, gridSize)
		if err != nil {
			continue
		}

		if CountSolutions(regionMap, gridSize) != 1 {
			continue
		}

		return &model.Puzzle{
			ID:        "",
			GridSize:  gridSize,
			Mode:      "standard",
			RegionMap: regionMap,
			Solution:  solution,
		}, nil
	}
}

// generateSolution places gridSize markers on a gridSize×gridSize grid such that
// each row and column has exactly one marker, and no two markers are adjacent
// (horizontally, vertically, or diagonally). Returns nil if no valid placement is found.
func generateSolution(gridSize int) [][]bool {
	// markerCols[row] = column of the marker in that row, -1 means unplaced.
	markerCols := make([]int, gridSize)
	for i := range markerCols {
		markerCols[i] = -1
	}
	usedCol := make([]bool, gridSize)

	if !placeSolutionRow(markerCols, usedCol, 0, gridSize) {
		return nil
	}

	grid := make([][]bool, gridSize)
	for r := 0; r < gridSize; r++ {
		grid[r] = make([]bool, gridSize)
		grid[r][markerCols[r]] = true
	}
	return grid
}

// placeSolutionRow recursively places markers row by row in a random order.
func placeSolutionRow(markerCols []int, usedCol []bool, row, gridSize int) bool {
	if row == gridSize {
		return true
	}

	// Try columns in random order.
	perm := rand.Perm(gridSize)
	for _, col := range perm {
		if usedCol[col] {
			continue
		}
		if !adjacencySafe(markerCols, row, col) {
			continue
		}

		markerCols[row] = col
		usedCol[col] = true

		if placeSolutionRow(markerCols, usedCol, row+1, gridSize) {
			return true
		}

		markerCols[row] = -1
		usedCol[col] = false
	}
	return false
}
