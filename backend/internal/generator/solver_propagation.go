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

// eliminateCell marks a cell as unavailable and updates counts.
// Returns the list of cells that were actually eliminated (for undo).
func (s *propagationState) eliminateCell(r, c int) bool {
	if !s.available[r][c] {
		return false
	}
	s.available[r][c] = false
	s.rowAvail[r]--
	s.colAvail[c]--
	return true
}

func (s *propagationState) restoreCell(r, c int) {
	s.available[r][c] = true
	s.rowAvail[r]++
	s.colAvail[c]++
}

// propagatePlace eliminates cells that conflict with placing a marker at (r, c).
// Returns the list of eliminated cells for backtracking, or nil if a
// contradiction is detected.
func (s *propagationState) propagatePlace(r, c int) (eliminated [][2]int, ok bool) {
	eliminated = make([][2]int, 0, 16)

	// Eliminate the placed cell itself from available.
	if s.eliminateCell(r, c) {
		eliminated = append(eliminated, [2]int{r, c})
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
					eliminated = append(eliminated, [2]int{nr, nc})
				}
			}
		}
	}

	// If this row now has markersPerUnit markers, eliminate all remaining
	// available cells in this row.
	if s.rowPlaced[r] >= s.markersPerUnit {
		for cc := 0; cc < s.gridSize; cc++ {
			if s.eliminateCell(r, cc) {
				eliminated = append(eliminated, [2]int{r, cc})
			}
		}
	}

	// If this column now has markersPerUnit markers, eliminate all remaining
	// available cells in this column.
	if s.colPlaced[c] >= s.markersPerUnit {
		for rr := 0; rr < s.gridSize; rr++ {
			if s.eliminateCell(rr, c) {
				eliminated = append(eliminated, [2]int{rr, c})
			}
		}
	}

	// Check for contradictions: any row or column that still needs markers
	// but doesn't have enough available cells.
	for i := 0; i < s.gridSize; i++ {
		rowNeeded := s.markersPerUnit - s.rowPlaced[i]
		if rowNeeded > 0 && s.rowAvail[i] < rowNeeded {
			return eliminated, false
		}
		colNeeded := s.markersPerUnit - s.colPlaced[i]
		if colNeeded > 0 && s.colAvail[i] < colNeeded {
			return eliminated, false
		}
	}

	return eliminated, true
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

		if ok {
			// Try forced moves via propagation chain.
			forcedElim, forcedOk := propApplyForced(s)
			if forcedOk {
				// For next marker in same row, use col+1 to maintain
				// combination ordering. But since we shuffle, just pass 0
				// and let availability filtering handle it. Actually we need
				// to allow any column > col for combination ordering.
				if propGenCols(s, row, col+1) {
					return true
				}
			}
			// Undo forced eliminations.
			propUndoForced(s, forcedElim)
		}

		// Undo placement.
		s.undoEliminations(eliminated)
		s.markerCols[row] = s.markerCols[row][:len(s.markerCols[row])-1]
		s.rowPlaced[row]--
		s.colPlaced[col]--
	}
	return false
}

// forcedResult tracks marker placements and eliminations from forced moves
// so they can be undone on backtrack.
type forcedResult struct {
	placements  [][2]int   // markers placed by forced moves
	eliminations [][][2]int // eliminations per placement
}

// propApplyForced repeatedly finds rows or columns with exactly the right
// number of available cells to fill and places markers there.
// Returns the forced results for undo, and whether a contradiction was found.
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

					s.markerCols[r] = append(s.markerCols[r], c)
					s.rowPlaced[r]++
					s.colPlaced[c]++
					eliminated, ok := s.propagatePlace(r, c)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, eliminated)

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

					s.markerCols[r] = append(s.markerCols[r], c)
					s.rowPlaced[r]++
					s.colPlaced[c]++
					eliminated, ok := s.propagatePlace(r, c)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, eliminated)

					if !ok {
						return result, false
					}
					changed = true
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
	}
}

// countPropagationState extends propagationState with region tracking for
// CountSolutions.
type countPropagationState struct {
	*propagationState
	regionMap    [][]int
	regionPlaced []int
	regionAvail  []int
	numRegions   int
}

func newCountPropagationState(regionMap [][]int, gridSize, markersPerUnit int) *countPropagationState {
	base := newPropagationState(gridSize, markersPerUnit)

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
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			regionAvail[regionMap[r][c]]++
		}
	}

	return &countPropagationState{
		propagationState: base,
		regionMap:        regionMap,
		regionPlaced:     make([]int, numRegions),
		regionAvail:      regionAvail,
		numRegions:       numRegions,
	}
}

// eliminateCellCount marks a cell as unavailable, updating region counts too.
func (cs *countPropagationState) eliminateCellCount(r, c int) bool {
	if !cs.available[r][c] {
		return false
	}
	cs.available[r][c] = false
	cs.rowAvail[r]--
	cs.colAvail[c]--
	cs.regionAvail[cs.regionMap[r][c]]--
	return true
}

func (cs *countPropagationState) restoreCellCount(r, c int) {
	cs.available[r][c] = true
	cs.rowAvail[r]++
	cs.colAvail[c]++
	cs.regionAvail[cs.regionMap[r][c]]++
}

// propagatePlaceCount propagates constraints after placing a marker at (r, c),
// including region constraints.
func (cs *countPropagationState) propagatePlaceCount(r, c int) (eliminated [][2]int, ok bool) {
	eliminated = make([][2]int, 0, 16)

	// Eliminate the placed cell itself.
	if cs.eliminateCellCount(r, c) {
		eliminated = append(eliminated, [2]int{r, c})
	}

	// Eliminate all 8 adjacent cells.
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			nr, nc := r+dr, c+dc
			if nr >= 0 && nr < cs.gridSize && nc >= 0 && nc < cs.gridSize {
				if cs.eliminateCellCount(nr, nc) {
					eliminated = append(eliminated, [2]int{nr, nc})
				}
			}
		}
	}

	// If row is full, eliminate remaining cells in row.
	if cs.rowPlaced[r] >= cs.markersPerUnit {
		for cc := 0; cc < cs.gridSize; cc++ {
			if cs.eliminateCellCount(r, cc) {
				eliminated = append(eliminated, [2]int{r, cc})
			}
		}
	}

	// If column is full, eliminate remaining cells in column.
	if cs.colPlaced[c] >= cs.markersPerUnit {
		for rr := 0; rr < cs.gridSize; rr++ {
			if cs.eliminateCellCount(rr, c) {
				eliminated = append(eliminated, [2]int{rr, c})
			}
		}
	}

	// If region is full, eliminate remaining cells in region.
	rid := cs.regionMap[r][c]
	if cs.regionPlaced[rid] >= cs.markersPerUnit {
		for rr := 0; rr < cs.gridSize; rr++ {
			for cc := 0; cc < cs.gridSize; cc++ {
				if cs.regionMap[rr][cc] == rid {
					if cs.eliminateCellCount(rr, cc) {
						eliminated = append(eliminated, [2]int{rr, cc})
					}
				}
			}
		}
	}

	// Check for contradictions.
	for i := 0; i < cs.gridSize; i++ {
		rowNeeded := cs.markersPerUnit - cs.rowPlaced[i]
		if rowNeeded > 0 && cs.rowAvail[i] < rowNeeded {
			return eliminated, false
		}
		colNeeded := cs.markersPerUnit - cs.colPlaced[i]
		if colNeeded > 0 && cs.colAvail[i] < colNeeded {
			return eliminated, false
		}
	}
	for rid := 0; rid < cs.numRegions; rid++ {
		regNeeded := cs.markersPerUnit - cs.regionPlaced[rid]
		if regNeeded > 0 && cs.regionAvail[rid] < regNeeded {
			return eliminated, false
		}
	}

	return eliminated, true
}

func (cs *countPropagationState) undoEliminationsCount(eliminated [][2]int) {
	for i := len(eliminated) - 1; i >= 0; i-- {
		cs.restoreCellCount(eliminated[i][0], eliminated[i][1])
	}
}

// countForcedResult tracks forced placements for CountSolutions.
type countForcedResult struct {
	placements   [][2]int
	eliminations [][][2]int
}

// applyForcedCount applies forced moves considering row, column, and region constraints.
func applyForcedCount(cs *countPropagationState) (countForcedResult, bool) {
	result := countForcedResult{}

	changed := true
	for changed {
		changed = false

		// Check rows.
		for r := 0; r < cs.gridSize; r++ {
			needed := cs.markersPerUnit - cs.rowPlaced[r]
			if needed <= 0 {
				continue
			}
			if cs.rowAvail[r] < needed {
				return result, false
			}
			if cs.rowAvail[r] == needed {
				for c := 0; c < cs.gridSize; c++ {
					if !cs.available[r][c] {
						continue
					}
					if cs.colPlaced[c] >= cs.markersPerUnit {
						return result, false
					}
					rid := cs.regionMap[r][c]
					if cs.regionPlaced[rid] >= cs.markersPerUnit {
						return result, false
					}

					cs.markerCols[r] = append(cs.markerCols[r], c)
					cs.rowPlaced[r]++
					cs.colPlaced[c]++
					cs.regionPlaced[rid]++
					eliminated, ok := cs.propagatePlaceCount(r, c)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, eliminated)

					if !ok {
						return result, false
					}
					changed = true
				}
			}
		}

		// Check columns.
		for c := 0; c < cs.gridSize; c++ {
			needed := cs.markersPerUnit - cs.colPlaced[c]
			if needed <= 0 {
				continue
			}
			if cs.colAvail[c] < needed {
				return result, false
			}
			if cs.colAvail[c] == needed {
				for r := 0; r < cs.gridSize; r++ {
					if !cs.available[r][c] {
						continue
					}
					if cs.rowPlaced[r] >= cs.markersPerUnit {
						return result, false
					}
					rid := cs.regionMap[r][c]
					if cs.regionPlaced[rid] >= cs.markersPerUnit {
						return result, false
					}

					cs.markerCols[r] = append(cs.markerCols[r], c)
					cs.rowPlaced[r]++
					cs.colPlaced[c]++
					cs.regionPlaced[rid]++
					eliminated, ok := cs.propagatePlaceCount(r, c)
					result.placements = append(result.placements, [2]int{r, c})
					result.eliminations = append(result.eliminations, eliminated)

					if !ok {
						return result, false
					}
					changed = true
				}
			}
		}

		// Check regions.
		for rid := 0; rid < cs.numRegions; rid++ {
			needed := cs.markersPerUnit - cs.regionPlaced[rid]
			if needed <= 0 {
				continue
			}
			if cs.regionAvail[rid] < needed {
				return result, false
			}
			if cs.regionAvail[rid] == needed {
				for r := 0; r < cs.gridSize; r++ {
					for c := 0; c < cs.gridSize; c++ {
						if cs.regionMap[r][c] != rid || !cs.available[r][c] {
							continue
						}
						if cs.rowPlaced[r] >= cs.markersPerUnit {
							return result, false
						}
						if cs.colPlaced[c] >= cs.markersPerUnit {
							return result, false
						}

						cs.markerCols[r] = append(cs.markerCols[r], c)
						cs.rowPlaced[r]++
						cs.colPlaced[c]++
						cs.regionPlaced[rid]++
						eliminated, ok := cs.propagatePlaceCount(r, c)
						result.placements = append(result.placements, [2]int{r, c})
						result.eliminations = append(result.eliminations, eliminated)

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

// undoForcedCount reverses forced placements.
func undoForcedCount(cs *countPropagationState, result countForcedResult) {
	for i := len(result.placements) - 1; i >= 0; i-- {
		r, c := result.placements[i][0], result.placements[i][1]
		cs.undoEliminationsCount(result.eliminations[i])

		rid := cs.regionMap[r][c]
		cols := cs.markerCols[r]
		for j := len(cols) - 1; j >= 0; j-- {
			if cols[j] == c {
				cs.markerCols[r] = append(cols[:j], cols[j+1:]...)
				break
			}
		}
		cs.rowPlaced[r]--
		cs.colPlaced[c]--
		cs.regionPlaced[rid]--
	}
}

// CountSolutions returns the number of valid solutions (stops at maxSolutions)
// using backtracking with constraint propagation including region constraints.
func (s *PropagationSolver) CountSolutions(regionMap [][]int, gridSize int, markersPerUnit int, maxSolutions int) int {
	cs := newCountPropagationState(regionMap, gridSize, markersPerUnit)
	count := 0
	countPropRow(cs, 0, maxSolutions, &count)
	return count
}

// countPropRow recursively counts solutions row by row with propagation.
func countPropRow(cs *countPropagationState, row, maxSolutions int, count *int) {
	if *count >= maxSolutions {
		return
	}
	if row == cs.gridSize {
		*count++
		return
	}

	// Skip rows that already have enough markers (from forced moves).
	if cs.rowPlaced[row] >= cs.markersPerUnit {
		countPropRow(cs, row+1, maxSolutions, count)
		return
	}

	countPropCols(cs, row, 0, maxSolutions, count)
}

// countPropCols recursively selects columns for a row during counting.
// Deterministic ordering (no shuffle) for completeness.
func countPropCols(cs *countPropagationState, row, startCol, maxSolutions int, count *int) {
	if *count >= maxSolutions {
		return
	}
	if cs.rowPlaced[row] >= cs.markersPerUnit {
		countPropRow(cs, row+1, maxSolutions, count)
		return
	}

	remaining := cs.markersPerUnit - cs.rowPlaced[row]
	for col := startCol; col <= cs.gridSize-remaining; col++ {
		if *count >= maxSolutions {
			return
		}
		if !cs.available[row][col] {
			continue
		}
		if cs.colPlaced[col] >= cs.markersPerUnit {
			continue
		}
		rid := cs.regionMap[row][col]
		if cs.regionPlaced[rid] >= cs.markersPerUnit {
			continue
		}

		// Place marker.
		cs.markerCols[row] = append(cs.markerCols[row], col)
		cs.rowPlaced[row]++
		cs.colPlaced[col]++
		cs.regionPlaced[rid]++

		eliminated, ok := cs.propagatePlaceCount(row, col)

		if ok {
			forcedResult, forcedOk := applyForcedCount(cs)
			if forcedOk {
				countPropCols(cs, row, col+1, maxSolutions, count)
			}
			undoForcedCount(cs, forcedResult)
		}

		// Undo placement.
		cs.undoEliminationsCount(eliminated)
		cs.markerCols[row] = cs.markerCols[row][:len(cs.markerCols[row])-1]
		cs.rowPlaced[row]--
		cs.colPlaced[col]--
		cs.regionPlaced[rid]--

		if *count >= maxSolutions {
			return
		}
	}
}
