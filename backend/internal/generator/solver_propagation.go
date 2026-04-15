package generator

import "math/rand/v2"

// PropagationSolver implements SolverStrategy using backtracking enhanced with
// constraint propagation. After each marker placement it propagates the
// consequences — eliminating cells in the same row, column, and adjacent
// positions — and detects forced moves and contradictions early. This
// dramatically prunes the search tree compared to brute-force backtracking,
// enabling efficient solving of 7x7, 9x9, and larger grids.
type PropagationSolver struct{}

// NewPropagationSolver returns a new PropagationSolver.
func NewPropagationSolver() *PropagationSolver {
	return &PropagationSolver{}
}

// propagationState holds the mutable state for constraint propagation search.
// When regionMap is non-nil, region constraints are also tracked (used by
// CountSolutions). When regionMap is nil, only row/column/adjacency constraints
// are enforced (used by GenerateSolution).
type propagationState struct {
	gridSize       int
	markersPerUnit int

	// available[r][c] is true if cell (r,c) can still receive a marker.
	available [][]bool

	// rowAvail[r] counts how many available cells remain in row r.
	rowAvail []int
	// colAvail[c] counts how many available cells remain in column c.
	colAvail []int

	// rowPlaced[r] counts how many markers have been placed in row r.
	rowPlaced []int
	// colPlaced[c] counts how many markers have been placed in column c.
	colPlaced []int

	// markerCols[r] holds columns where markers are placed in row r.
	markerCols [][]int

	// eliminated is a pre-allocated scratch buffer for propagatePlace to avoid
	// per-call allocation on the hot path.
	eliminated [][2]int

	// --- Optional region tracking (nil when not used) ---

	// regionMap[r][c] is the region ID of cell (r,c). Nil when regions are not tracked.
	regionMap [][]int
	// regionPlaced[rid] counts how many markers have been placed in region rid.
	regionPlaced []int
	// regionAvail[rid] counts how many available cells remain in region rid.
	regionAvail []int
	// regionCells[rid] lists (row, col) pairs for each region, enabling O(region size)
	// iteration instead of O(gridSize*gridSize) scans.
	regionCells [][][2]int
	// numRegions is the number of distinct regions.
	numRegions int
}

func newPropagationState(gridSize, markersPerUnit int) *propagationState {
	s := &propagationState{
		gridSize:       gridSize,
		markersPerUnit: markersPerUnit,
		available:      make([][]bool, gridSize),
		rowAvail:       make([]int, gridSize),
		colAvail:       make([]int, gridSize),
		rowPlaced:      make([]int, gridSize),
		colPlaced:      make([]int, gridSize),
		markerCols:     make([][]int, gridSize),
		eliminated:     make([][2]int, 0, 4*gridSize),
	}
	for r := 0; r < gridSize; r++ {
		s.available[r] = make([]bool, gridSize)
		for c := 0; c < gridSize; c++ {
			s.available[r][c] = true
		}
		s.rowAvail[r] = gridSize
		s.markerCols[r] = make([]int, 0, markersPerUnit)
	}
	for c := 0; c < gridSize; c++ {
		s.colAvail[c] = gridSize
	}
	return s
}

// newPropagationStateWithRegions creates a propagation state that also tracks
// region constraints. Used by CountSolutions.
func newPropagationStateWithRegions(regionMap [][]int, gridSize, markersPerUnit int) *propagationState {
	s := newPropagationState(gridSize, markersPerUnit)

	// Determine number of regions.
	numRegions := 0
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			rid := regionMap[r][c]
			if rid >= numRegions {
				numRegions = rid + 1
			}
		}
	}

	regionAvail := make([]int, numRegions)
	regionCells := make([][][2]int, numRegions)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			rid := regionMap[r][c]
			regionAvail[rid]++
			regionCells[rid] = append(regionCells[rid], [2]int{r, c})
		}
	}

	s.regionMap = regionMap
	s.regionPlaced = make([]int, numRegions)
	s.regionAvail = regionAvail
	s.regionCells = regionCells
	s.numRegions = numRegions
	return s
}

// eliminateCell marks a cell as unavailable and updates counts, including
// region counts when region tracking is active.
func (s *propagationState) eliminateCell(r, c int) bool {
	if !s.available[r][c] {
		return false
	}
	s.available[r][c] = false
	s.rowAvail[r]--
	s.colAvail[c]--
	if s.regionMap != nil {
		s.regionAvail[s.regionMap[r][c]]--
	}
	return true
}

// restoreCell marks a cell as available again and updates counts.
func (s *propagationState) restoreCell(r, c int) {
	s.available[r][c] = true
	s.rowAvail[r]++
	s.colAvail[c]++
	if s.regionMap != nil {
		s.regionAvail[s.regionMap[r][c]]++
	}
}

// propagatePlace eliminates cells that conflict with placing a marker at (r, c).
// Returns the list of eliminated cells for backtracking, or nil if a
// contradiction is detected. Uses a pre-allocated scratch buffer to avoid
// per-call allocation.
func (s *propagationState) propagatePlace(r, c int) (eliminated [][2]int, ok bool) {
	s.eliminated = s.eliminated[:0]

	// Eliminate the placed cell itself from available.
	if s.eliminateCell(r, c) {
		s.eliminated = append(s.eliminated, [2]int{r, c})
	}

	// Eliminate all 8 adjacent cells.
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			nr, nc := r+dr, c+dc
			if nr >= 0 && nr < s.gridSize && nc >= 0 && nc < s.gridSize {
				if s.eliminateCell(nr, nc) {
					s.eliminated = append(s.eliminated, [2]int{nr, nc})
				}
			}
		}
	}

	// If this row now has markersPerUnit markers, eliminate all remaining
	// available cells in this row.
	if s.rowPlaced[r] >= s.markersPerUnit {
		for cc := 0; cc < s.gridSize; cc++ {
			if s.eliminateCell(r, cc) {
				s.eliminated = append(s.eliminated, [2]int{r, cc})
			}
		}
	}

	// If this column now has markersPerUnit markers, eliminate all remaining
	// available cells in this column.
	if s.colPlaced[c] >= s.markersPerUnit {
		for rr := 0; rr < s.gridSize; rr++ {
			if s.eliminateCell(rr, c) {
				s.eliminated = append(s.eliminated, [2]int{rr, c})
			}
		}
	}

	// If region tracking is active and this region is full, eliminate all
	// remaining available cells in the region using the per-region cell index.
	if s.regionMap != nil {
		rid := s.regionMap[r][c]
		if s.regionPlaced[rid] >= s.markersPerUnit {
			for _, cell := range s.regionCells[rid] {
				if s.eliminateCell(cell[0], cell[1]) {
					s.eliminated = append(s.eliminated, [2]int{cell[0], cell[1]})
				}
			}
		}
	}

	// Check for contradictions: any row or column that still needs markers
	// but doesn't have enough available cells.
	for i := 0; i < s.gridSize; i++ {
		rowNeeded := s.markersPerUnit - s.rowPlaced[i]
		if rowNeeded > 0 && s.rowAvail[i] < rowNeeded {
			return s.eliminated, false
		}
		colNeeded := s.markersPerUnit - s.colPlaced[i]
		if colNeeded > 0 && s.colAvail[i] < colNeeded {
			return s.eliminated, false
		}
	}

	// Check region contradictions when tracking regions.
	if s.regionMap != nil {
		for rid := 0; rid < s.numRegions; rid++ {
			regNeeded := s.markersPerUnit - s.regionPlaced[rid]
			if regNeeded > 0 && s.regionAvail[rid] < regNeeded {
				return s.eliminated, false
			}
		}
	}

	return s.eliminated, true
}

// undoEliminations restores previously eliminated cells.
func (s *propagationState) undoEliminations(eliminated [][2]int) {
	for i := len(eliminated) - 1; i >= 0; i-- {
		s.restoreCell(eliminated[i][0], eliminated[i][1])
	}
}

// GenerateSolution produces a random valid marker placement for the given grid
// using backtracking with constraint propagation. It returns nil if no valid
// placement is found.
func (s *PropagationSolver) GenerateSolution(gridSize int, markersPerUnit int) [][]bool {
	state := newPropagationState(gridSize, markersPerUnit)
	if !propGenRow(state, 0) {
		return nil
	}

	grid := make([][]bool, gridSize)
	for r := 0; r < gridSize; r++ {
		grid[r] = make([]bool, gridSize)
		for _, c := range state.markerCols[r] {
			grid[r][c] = true
		}
	}
	return grid
}

// propGenRow recursively places markers row by row with propagation.
func propGenRow(s *propagationState, row int) bool {
	if row == s.gridSize {
		return true
	}

	// Skip rows that already have enough markers (from forced moves).
	if s.rowPlaced[row] >= s.markersPerUnit {
		return propGenRow(s, row+1)
	}

	return propGenCols(s, row, 0)
}

// propGenCols recursively selects columns for a row during generation.
// Uses random ordering for non-deterministic generation. startCol ensures
// combination ordering (columns in increasing order within each row).
func propGenCols(s *propagationState, row, startCol int) bool {
	if s.rowPlaced[row] >= s.markersPerUnit {
		return propGenRow(s, row+1)
	}

	remaining := s.markersPerUnit - s.rowPlaced[row]

	// Collect all valid candidate columns >= startCol.
	candidates := make([]int, 0, s.gridSize-startCol)
	for c := startCol; c < s.gridSize; c++ {
		if !s.available[row][c] {
			continue
		}
		if s.colPlaced[c] >= s.markersPerUnit {
			continue
		}
		candidates = append(candidates, c)
	}

	// Need at least `remaining` candidates to fill this row.
	if len(candidates) < remaining {
		return false
	}

	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for _, col := range candidates {
		// Double-check availability (may have changed from forced moves).
		if !s.available[row][col] {
			continue
		}
		if s.colPlaced[col] >= s.markersPerUnit {
			continue
		}

		// Place marker.
		s.markerCols[row] = append(s.markerCols[row], col)
		s.rowPlaced[row]++
		s.colPlaced[col]++

		eliminated, ok := s.propagatePlace(row, col)
		// Copy eliminated since the scratch buffer will be reused.
		elimCopy := make([][2]int, len(eliminated))
		copy(elimCopy, eliminated)

		if ok {
			// Try forced moves via propagation chain.
			forcedElim, forcedOk := propApplyForced(s)
			if forcedOk {
				if propGenCols(s, row, col+1) {
					return true
				}
			}
			// Undo forced eliminations.
			propUndoForced(s, forcedElim)
		}

		// Undo placement.
		s.undoEliminations(elimCopy)
		s.markerCols[row] = s.markerCols[row][:len(s.markerCols[row])-1]
		s.rowPlaced[row]--
		s.colPlaced[col]--
	}
	return false
}

// forcedResult tracks marker placements and eliminations from forced moves
// so they can be undone on backtrack.
type forcedResult struct {
	placements   [][2]int
	eliminations [][][2]int
}

// propApplyForced repeatedly finds rows, columns, or regions (when tracked)
// with exactly the right number of available cells to fill and places markers
// there. Returns the forced results for undo, and whether a contradiction was
// found.
func propApplyForced(s *propagationState) (forcedResult, bool) {
	result := forcedResult{}

	changed := true
	for changed {
		changed = false

		// Check rows for forced moves.
		for r := 0; r < s.gridSize; r++ {
			needed := s.markersPerUnit - s.rowPlaced[r]
			if needed <= 0 {
				continue
			}
			if s.rowAvail[r] < needed {
				return result, false // contradiction
			}
			if s.rowAvail[r] == needed {
				// All remaining available cells in this row must get markers.
				for c := 0; c < s.gridSize; c++ {
					if !s.available[r][c] {
						continue
					}
					if s.colPlaced[c] >= s.markersPerUnit {
						return result, false // contradiction
					}
					if s.regionMap != nil {
						rid := s.regionMap[r][c]
						if s.regionPlaced[rid] >= s.markersPerUnit {
							return result, false // contradiction
						}
					}

					s.markerCols[r] = append(s.markerCols[r], c)
					s.rowPlaced[r]++
					s.colPlaced[c]++
					if s.regionMap != nil {
						s.regionPlaced[s.regionMap[r][c]]++
					}
					eliminated, ok := s.propagatePlace(r, c)
					// Copy since scratch buffer is reused.
					elimCopy := make([][2]int, len(eliminated))
					copy(elimCopy, eliminated)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, elimCopy)

					if !ok {
						return result, false
					}
					changed = true
				}
			}
		}

		// Check columns for forced moves.
		for c := 0; c < s.gridSize; c++ {
			needed := s.markersPerUnit - s.colPlaced[c]
			if needed <= 0 {
				continue
			}
			if s.colAvail[c] < needed {
				return result, false // contradiction
			}
			if s.colAvail[c] == needed {
				// All remaining available cells in this column must get markers.
				for r := 0; r < s.gridSize; r++ {
					if !s.available[r][c] {
						continue
					}
					if s.rowPlaced[r] >= s.markersPerUnit {
						return result, false // contradiction
					}
					if s.regionMap != nil {
						rid := s.regionMap[r][c]
						if s.regionPlaced[rid] >= s.markersPerUnit {
							return result, false // contradiction
						}
					}

					s.markerCols[r] = append(s.markerCols[r], c)
					s.rowPlaced[r]++
					s.colPlaced[c]++
					if s.regionMap != nil {
						s.regionPlaced[s.regionMap[r][c]]++
					}
					eliminated, ok := s.propagatePlace(r, c)
					elimCopy := make([][2]int, len(eliminated))
					copy(elimCopy, eliminated)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, elimCopy)

					if !ok {
						return result, false
					}
					changed = true
				}
			}
		}

		// Check regions for forced moves (only when region tracking is active).
		if s.regionMap != nil {
			for rid := 0; rid < s.numRegions; rid++ {
				needed := s.markersPerUnit - s.regionPlaced[rid]
				if needed <= 0 {
					continue
				}
				if s.regionAvail[rid] < needed {
					return result, false // contradiction
				}
				if s.regionAvail[rid] == needed {
					for _, cell := range s.regionCells[rid] {
						r, c := cell[0], cell[1]
						if !s.available[r][c] {
							continue
						}
						if s.rowPlaced[r] >= s.markersPerUnit {
							return result, false
						}
						if s.colPlaced[c] >= s.markersPerUnit {
							return result, false
						}

						s.markerCols[r] = append(s.markerCols[r], c)
						s.rowPlaced[r]++
						s.colPlaced[c]++
						s.regionPlaced[rid]++
						eliminated, ok := s.propagatePlace(r, c)
						elimCopy := make([][2]int, len(eliminated))
						copy(elimCopy, eliminated)
						result.placements = append(result.placements, [2]int{r, c})
						result.eliminations = append(result.eliminations, elimCopy)

						if !ok {
							return result, false
						}
						changed = true
					}
				}
			}
		}
	}

	return result, true
}

// propUndoForced reverses forced move placements and their eliminations.
func propUndoForced(s *propagationState, result forcedResult) {
	// Undo in reverse order.
	for i := len(result.placements) - 1; i >= 0; i-- {
		r, c := result.placements[i][0], result.placements[i][1]
		s.undoEliminations(result.eliminations[i])

		// Remove the marker.
		cols := s.markerCols[r]
		for j := len(cols) - 1; j >= 0; j-- {
			if cols[j] == c {
				s.markerCols[r] = append(cols[:j], cols[j+1:]...)
				break
			}
		}
		s.rowPlaced[r]--
		s.colPlaced[c]--
		if s.regionMap != nil {
			s.regionPlaced[s.regionMap[r][c]]--
		}
	}
}

// CountSolutions returns the number of valid solutions (stops at maxSolutions)
// using backtracking with constraint propagation including region constraints.
func (s *PropagationSolver) CountSolutions(regionMap [][]int, gridSize int, markersPerUnit int, maxSolutions int) int {
	state := newPropagationStateWithRegions(regionMap, gridSize, markersPerUnit)
	count := 0
	countPropRow(state, 0, maxSolutions, &count)
	return count
}

// countPropRow recursively counts solutions row by row with propagation.
func countPropRow(s *propagationState, row, maxSolutions int, count *int) {
	if *count >= maxSolutions {
		return
	}
	if row == s.gridSize {
		*count++
		return
	}

	// Skip rows that already have enough markers (from forced moves).
	if s.rowPlaced[row] >= s.markersPerUnit {
		countPropRow(s, row+1, maxSolutions, count)
		return
	}

	countPropCols(s, row, 0, maxSolutions, count)
}

// IsLogicallyDeducible checks whether a puzzle can be solved entirely through
// forced moves (naked singles) without any guessing/branching. Returns true
// if constraint propagation alone solves the puzzle.
//
// The algorithm walks the same search tree as CountSolutions (row-by-row marker
// placement with constraint propagation and forced-move cascades) but verifies
// that no backtracking ever occurs. A puzzle is deducible when the entire
// search traverses a single path — at every row, the first valid candidate
// (after forced moves) leads directly to the solution without any undo.
//
// Concretely: the solver places markers row by row. After each placement it
// propagates constraints and applies cascading forced moves. If at any point
// the solver would need to try a second candidate for the same row (i.e.,
// backtrack), the puzzle is not deducible.
func IsLogicallyDeducible(regionMap [][]int, gridSize int, markersPerUnit int) bool {
	state := newPropagationStateWithRegions(regionMap, gridSize, markersPerUnit)

	// Apply initial forced moves.
	forcedRes, ok := propApplyForced(state)
	if !ok {
		_ = forcedRes
		return false
	}

	return deducibleSearchRow(state, 0)
}

// deducibleSearchRow walks the search tree row by row. Returns true only if
// the solution is reached without any backtracking.
func deducibleSearchRow(s *propagationState, row int) bool {
	if row == s.gridSize {
		return true
	}

	if s.rowPlaced[row] >= s.markersPerUnit {
		return deducibleSearchRow(s, row+1)
	}

	return deducibleSearchCols(s, row, 0, false)
}

// deducibleSearchCols tries candidates in a row. The firstAttempted flag tracks
// whether we've already tried (and backtracked from) a candidate — if so, the
// puzzle requires branching and is not deducible.
func deducibleSearchCols(s *propagationState, row, startCol int, firstAttempted bool) bool {
	if s.rowPlaced[row] >= s.markersPerUnit {
		return deducibleSearchRow(s, row+1)
	}

	remaining := s.markersPerUnit - s.rowPlaced[row]

	for col := startCol; col <= s.gridSize-remaining; col++ {
		if !s.available[row][col] {
			continue
		}
		if s.colPlaced[col] >= s.markersPerUnit {
			continue
		}
		rid := s.regionMap[row][col]
		if s.regionPlaced[rid] >= s.markersPerUnit {
			continue
		}

		if firstAttempted {
			// A previous candidate was tried and failed — this means
			// backtracking is needed, so the puzzle is not deducible.
			return false
		}

		// Place marker.
		s.markerCols[row] = append(s.markerCols[row], col)
		s.rowPlaced[row]++
		s.colPlaced[col]++
		s.regionPlaced[rid]++

		eliminated, placedOk := s.propagatePlace(row, col)
		elimCopy := make([][2]int, len(eliminated))
		copy(elimCopy, eliminated)

		if placedOk {
			forcedRes, forcedOk := propApplyForced(s)
			if forcedOk {
				if deducibleSearchCols(s, row, col+1, false) {
					return true
				}
			}
			propUndoForced(s, forcedRes)
		}

		// Undo placement.
		s.undoEliminations(elimCopy)
		s.markerCols[row] = s.markerCols[row][:len(s.markerCols[row])-1]
		s.rowPlaced[row]--
		s.colPlaced[col]--
		s.regionPlaced[rid]--

		// Mark that we've attempted and failed with one candidate.
		firstAttempted = true
	}
	return false
}

// countPropCols recursively selects columns for a row during counting.
// Deterministic ordering (no shuffle) for completeness.
func countPropCols(s *propagationState, row, startCol, maxSolutions int, count *int) {
	if *count >= maxSolutions {
		return
	}
	if s.rowPlaced[row] >= s.markersPerUnit {
		countPropRow(s, row+1, maxSolutions, count)
		return
	}

	remaining := s.markersPerUnit - s.rowPlaced[row]
	for col := startCol; col <= s.gridSize-remaining; col++ {
		if *count >= maxSolutions {
			return
		}
		if !s.available[row][col] {
			continue
		}
		if s.colPlaced[col] >= s.markersPerUnit {
			continue
		}
		// Region check (always present in counting path).
		rid := s.regionMap[row][col]
		if s.regionPlaced[rid] >= s.markersPerUnit {
			continue
		}

		// Place marker.
		s.markerCols[row] = append(s.markerCols[row], col)
		s.rowPlaced[row]++
		s.colPlaced[col]++
		s.regionPlaced[rid]++

		eliminated, ok := s.propagatePlace(row, col)
		// Copy since scratch buffer is reused.
		elimCopy := make([][2]int, len(eliminated))
		copy(elimCopy, eliminated)

		if ok {
			forcedResult, forcedOk := propApplyForced(s)
			if forcedOk {
				countPropCols(s, row, col+1, maxSolutions, count)
			}
			propUndoForced(s, forcedResult)
		}

		// Undo placement.
		s.undoEliminations(elimCopy)
		s.markerCols[row] = s.markerCols[row][:len(s.markerCols[row])-1]
		s.rowPlaced[row]--
		s.colPlaced[col]--
		s.regionPlaced[rid]--

		if *count >= maxSolutions {
			return
		}
	}
}
