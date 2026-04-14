package generator

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// Generate creates a random puzzle with the given grid size that has exactly
// one solution. It uses the provided solver and region strategies for pluggable
// generation algorithms. It retries until a unique puzzle is found or the
// timeout expires.
func Generate(gridSize int, markersPerUnit int, solver SolverStrategy, regions RegionStrategy, opts GenerateOpts) (*model.Puzzle, error) {
	deadline := time.Now().Add(opts.Timeout)

	mode := opts.Mode
	if mode == "" {
		mode = "standard"
	}

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle: timeout after %v", opts.Timeout)
		}

		solution := solver.GenerateSolution(gridSize, markersPerUnit)
		if solution == nil {
			continue
		}

		regionMap, err := regions.GenerateRegions(solution, gridSize, opts.RegionOpts)
		if err != nil {
			continue
		}

		if solver.CountSolutions(regionMap, gridSize, markersPerUnit, 2) != 1 {
			continue
		}

		return &model.Puzzle{
			ID:        "",
			GridSize:  gridSize,
			Mode:      mode,
			RegionMap: regionMap,
			Solution:  solution,
		}, nil
	}
}

// generateSolution places gridSize*markersPerUnit markers on a gridSize×gridSize
// grid such that each row and column has exactly markersPerUnit markers, and no
// two markers are adjacent (horizontally, vertically, or diagonally).
// Returns nil if no valid placement is found.
func generateSolution(gridSize, markersPerUnit int) [][]bool {
	// markerCols[row] holds the columns where markers are placed in that row.
	markerCols := make([][]int, gridSize)
	for i := range markerCols {
		markerCols[i] = make([]int, 0, markersPerUnit)
	}
	usedCol := make([]int, gridSize)

	if !placeSolutionRow(markerCols, usedCol, 0, gridSize, markersPerUnit) {
		return nil
	}

	grid := make([][]bool, gridSize)
	for r := 0; r < gridSize; r++ {
		grid[r] = make([]bool, gridSize)
		for _, c := range markerCols[r] {
			grid[r][c] = true
		}
	}
	return grid
}

// placeSolutionRow recursively places markersPerUnit markers per row in a
// random order. It tries random column combinations for each row.
func placeSolutionRow(markerCols [][]int, usedCol []int, row, gridSize, markersPerUnit int) bool {
	if row == gridSize {
		return true
	}

	// Generate all valid column combinations for this row via recursive helper.
	return placeSolutionCols(markerCols, usedCol, row, 0, 0, gridSize, markersPerUnit)
}

// placeSolutionCols recursively selects markersPerUnit columns for a single row
// during solution generation. Columns are tried in random order within each
// recursion level. startCol is the minimum column index to try (ensuring
// combinations, not permutations). placed counts markers placed so far in this row.
func placeSolutionCols(markerCols [][]int, usedCol []int, row, startCol, placed, gridSize, markersPerUnit int) bool {
	if placed == markersPerUnit {
		return placeSolutionRow(markerCols, usedCol, row+1, gridSize, markersPerUnit)
	}

	remaining := markersPerUnit - placed
	// Try columns in random order, but only those >= startCol.
	candidates := make([]int, 0, gridSize-startCol)
	for c := startCol; c <= gridSize-remaining; c++ {
		candidates = append(candidates, c)
	}
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for _, col := range candidates {
		if usedCol[col] >= markersPerUnit {
			continue
		}
		if !adjacencySafe(markerCols, row, col) {
			continue
		}

		markerCols[row] = append(markerCols[row], col)
		usedCol[col]++

		// For the next column in this row, we need col+1 as minimum to
		// maintain combination ordering. But since we shuffled, we need to
		// pass col+1 to ensure no duplicates while allowing any order.
		if placeSolutionCols(markerCols, usedCol, row, col+1, placed+1, gridSize, markersPerUnit) {
			return true
		}

		markerCols[row] = markerCols[row][:len(markerCols[row])-1]
		usedCol[col]--
	}
	return false
}
