package generator

// BFSRegionGenerator implements RegionStrategy using BFS-based region growing.
// It wraps the existing GenerateRegionMap function.
type BFSRegionGenerator struct{}

// NewBFSRegionGenerator returns a new BFSRegionGenerator.
func NewBFSRegionGenerator() *BFSRegionGenerator {
	return &BFSRegionGenerator{}
}

// GenerateRegions creates a region map from a solution placement using
// round-robin BFS growth. When opts.Variance is 0.0, all regions get exactly
// gridSize cells (uniform). Higher variance values produce variable region sizes
// controlled by computeTargetSizes.
func (g *BFSRegionGenerator) GenerateRegions(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error) {
	if opts.Variance <= 0.0 {
		return GenerateRegionMap(solution, gridSize)
	}
	minSize := opts.MinSize
	if minSize < minRegionSize {
		minSize = minRegionSize
	}
	targets := computeTargetSizes(gridSize, opts.Variance, minSize)
	return GenerateRegionMapVariable(solution, gridSize, targets)
}
