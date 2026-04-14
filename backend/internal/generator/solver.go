package generator

// solve collects up to maxSolutions valid marker placements on a region map.
// A valid placement puts exactly one marker per row, per column, and per region,
// with no two markers horizontally, vertically, or diagonally adjacent.
// It uses backtracking, placing markers row by row.
func solve(regionMap [][]int, gridSize int, maxSolutions int) [][][]bool {
	solutions := make([][][]bool, 0, maxSolutions)
	placedCol := make([]bool, gridSize)
	placedRegion := make([]bool, gridSize)
	markerCols := make([]int, gridSize)
	for i := range markerCols {
		markerCols[i] = -1
	}

	var backtrack func(row int)
	backtrack = func(row int) {
		if len(solutions) >= maxSolutions {
			return
		}
		if row == gridSize {
			sol := make([][]bool, gridSize)
			for r := 0; r < gridSize; r++ {
				sol[r] = make([]bool, gridSize)
				if markerCols[r] >= 0 {
					sol[r][markerCols[r]] = true
				}
			}
			solutions = append(solutions, sol)
			return
		}
		for col := 0; col < gridSize; col++ {
			if placedCol[col] {
				continue
			}
			rid := regionMap[row][col]
			if placedRegion[rid] {
				continue
			}
			if !adjacencySafe(markerCols, row, col) {
				continue
			}

			placedCol[col] = true
			placedRegion[rid] = true
			markerCols[row] = col

			backtrack(row + 1)

			placedCol[col] = false
			placedRegion[rid] = false
			markerCols[row] = -1

			if len(solutions) >= maxSolutions {
				return
			}
		}
	}

	backtrack(0)
	return solutions
}

// CountSolutions counts valid marker placements on a region map, stopping after
// finding 2 solutions. It returns 0, 1, or 2.
func CountSolutions(regionMap [][]int, gridSize int) int {
	return len(solve(regionMap, gridSize, 2))
}

// adjacencySafe checks that placing a marker at (row, col) does not violate
// the adjacency constraint with any previously placed marker in earlier rows.
// Two markers are adjacent if they differ by at most 1 in both row and column
// (horizontal, vertical, or diagonal neighbors).
func adjacencySafe(markerCols []int, row, col int) bool {
	for r := 0; r < row; r++ {
		mc := markerCols[r]
		if mc == -1 {
			continue
		}
		rowDiff := row - r
		colDiff := col - mc
		if colDiff < 0 {
			colDiff = -colDiff
		}
		if rowDiff <= 1 && colDiff <= 1 {
			return false
		}
	}
	return true
}

// Solve returns the unique solution for a region map, if one exists.
// It returns the solution grid and true if exactly one solution exists.
// If the puzzle has zero or multiple solutions, it returns nil and false.
func Solve(regionMap [][]int, gridSize int) ([][]bool, bool) {
	solutions := solve(regionMap, gridSize, 2)
	if len(solutions) == 1 {
		return solutions[0], true
	}
	return nil, false
}
