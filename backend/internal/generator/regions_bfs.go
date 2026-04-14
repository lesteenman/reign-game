package generator

// BFSRegionGenerator implements RegionStrategy using BFS-based region growing.
// It wraps the existing GenerateRegionMap function. It supports both standard
// mode (1 marker per region) and double mode (2 markers per region).
type BFSRegionGenerator struct {
	markersPerUnit int // 1 for standard, 2 for double
}

// NewBFSRegionGenerator returns a new BFSRegionGenerator for standard mode
// (1 marker per region).
func NewBFSRegionGenerator() *BFSRegionGenerator {
	return &BFSRegionGenerator{markersPerUnit: 1}
}

// NewBFSRegionGeneratorDouble returns a new BFSRegionGenerator for double mode
// (2 markers per region).
func NewBFSRegionGeneratorDouble() *BFSRegionGenerator {
	return &BFSRegionGenerator{markersPerUnit: 2}
}

// GenerateRegions creates a region map from a solution placement using
// round-robin BFS growth. When opts.Variance is 0.0, all regions get exactly
// gridSize cells (uniform). Higher variance values produce variable region sizes
// controlled by computeTargetSizes. For double mode (markersPerUnit=2), regions
// are seeded with paired markers instead of single markers.
func (g *BFSRegionGenerator) GenerateRegions(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error) {
	if g.markersPerUnit >= 2 {
		return g.generateDouble(solution, gridSize, opts)
	}
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

// generateDouble handles region generation for Double Queens mode.
func (g *BFSRegionGenerator) generateDouble(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error) {
	if opts.Variance <= 0.0 {
		return GenerateRegionMapDouble(solution, gridSize)
	}
	minSize := opts.MinSize
	if minSize < minRegionSizeDouble {
		minSize = minRegionSizeDouble
	}
	targets := computeTargetSizes(gridSize, opts.Variance, minSize)
	return GenerateRegionMapDoubleVariable(solution, gridSize, targets)
}
