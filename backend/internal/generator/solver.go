package generator

// solve collects up to maxSolutions valid marker placements on a region map.
// A valid placement puts exactly markersPerUnit markers per row, per column, and
// per region, with no two markers horizontally, vertically, or diagonally adjacent.
// It uses backtracking, placing markers row by row. For each row it selects
// markersPerUnit columns (combinations, not permutations).
func solve(regionMap [][]int, gridSize, markersPerUnit, maxSolutions int) [][][]bool {
	solutions := make([][][]bool, 0, maxSolutions)
	placedCol := make([]int, gridSize)
	placedRegion := make([]int, gridSize)
	// markerCols[row] holds the list of columns where markers are placed in that row.
	markerCols := make([][]int, gridSize)
	for i := range markerCols {
		markerCols[i] = make([]int, 0, markersPerUnit)
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
				for _, c := range markerCols[r] {
					sol[r][c] = true
				}
			}
			solutions = append(solutions, sol)
			return
		}
		// Place markersPerUnit markers in this row by choosing columns in
		// increasing order (combinations).
		placeInRow(regionMap, gridSize, markersPerUnit, maxSolutions, row, 0, 0,
			placedCol, placedRegion, markerCols, &solutions, backtrack)
	}

	backtrack(0)
	return solutions
}

// placeInRow recursively picks markersPerUnit columns for a single row.
// startCol ensures combinations are generated in increasing column order.
// placed tracks how many markers have been placed in this row so far.
func placeInRow(
	regionMap [][]int,
	gridSize, markersPerUnit, maxSolutions, row, startCol, placed int,
	placedCol, placedRegion []int,
	markerCols [][]int,
	solutions *[][][]bool,
	advanceRow func(int),
) {
	if len(*solutions) >= maxSolutions {
		return
	}
	if placed == markersPerUnit {
		advanceRow(row + 1)
		return
	}

	remaining := markersPerUnit - placed
	for col := startCol; col <= gridSize-remaining; col++ {
		if placedCol[col] >= markersPerUnit {
			continue
		}
		rid := regionMap[row][col]
		if placedRegion[rid] >= markersPerUnit {
			continue
		}
		if !adjacencySafe(markerCols, row, col) {
			continue
		}

		placedCol[col]++
		placedRegion[rid]++
		markerCols[row] = append(markerCols[row], col)

		placeInRow(regionMap, gridSize, markersPerUnit, maxSolutions, row, col+1, placed+1,
			placedCol, placedRegion, markerCols, solutions, advanceRow)

		markerCols[row] = markerCols[row][:len(markerCols[row])-1]
		placedCol[col]--
		placedRegion[rid]--

		if len(*solutions) >= maxSolutions {
			return
		}
	}
}

// CountSolutions counts valid marker placements on a region map, stopping after
// finding 2 solutions. It returns 0, 1, or 2. The markersPerUnit parameter
// specifies how many markers each row, column, and region must contain.
func CountSolutions(regionMap [][]int, gridSize, markersPerUnit int) int {
	return len(solve(regionMap, gridSize, markersPerUnit, 2))
}

// adjacencySafe checks that placing a marker at (row, col) does not violate
// the adjacency constraint with any previously placed marker in earlier rows.
// Two markers are adjacent if they differ by at most 1 in both row and column
// (horizontal, vertical, or diagonal neighbors). It also checks against markers
// already placed in the same row (for markersPerUnit > 1).
func adjacencySafe(markerCols [][]int, row, col int) bool {
	// Check all previous rows.
	for r := 0; r < row; r++ {
		for _, mc := range markerCols[r] {
			rowDiff := row - r
			colDiff := col - mc
			if colDiff < 0 {
				colDiff = -colDiff
			}
			if rowDiff <= 1 && colDiff <= 1 {
				return false
			}
		}
	}
	// Check markers already placed in the current row.
	for _, mc := range markerCols[row] {
		colDiff := col - mc
		if colDiff < 0 {
			colDiff = -colDiff
		}
		if colDiff <= 1 {
			return false
		}
	}
	return true
}

// Solve returns the unique solution for a region map, if one exists.
// It returns the solution grid and true if exactly one solution exists.
// If the puzzle has zero or multiple solutions, it returns nil and false.
// The markersPerUnit parameter specifies how many markers each row, column,
// and region must contain.
func Solve(regionMap [][]int, gridSize, markersPerUnit int) ([][]bool, bool) {
	solutions := solve(regionMap, gridSize, markersPerUnit, 2)
	if len(solutions) == 1 {
		return solutions[0], true
	}
	return nil, false
}
