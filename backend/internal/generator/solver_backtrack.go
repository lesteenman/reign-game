package generator

// BacktrackSolver implements SolverStrategy using brute-force backtracking.
// It wraps the existing solve and generateSolution functions.
type BacktrackSolver struct{}

// NewBacktrackSolver returns a new BacktrackSolver.
func NewBacktrackSolver() *BacktrackSolver {
	return &BacktrackSolver{}
}

// GenerateSolution produces a random valid marker placement for the given grid
// using randomized backtracking. It returns nil if no valid placement is found.
func (s *BacktrackSolver) GenerateSolution(gridSize, markersPerUnit int) [][]bool {
	return generateSolution(gridSize, markersPerUnit)
}

// CountSolutions returns the number of valid solutions (stops at maxSolutions)
// using deterministic backtracking.
func (s *BacktrackSolver) CountSolutions(regionMap [][]int, gridSize, markersPerUnit, maxSolutions int) int {
	return len(solve(regionMap, gridSize, markersPerUnit, maxSolutions))
}
