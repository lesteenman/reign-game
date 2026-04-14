package generator

// BFSRegionGenerator implements RegionStrategy using BFS-based region growing.
// It wraps the existing GenerateRegionMap function.
type BFSRegionGenerator struct{}

// NewBFSRegionGenerator returns a new BFSRegionGenerator.
func NewBFSRegionGenerator() *BFSRegionGenerator {
	return &BFSRegionGenerator{}
}

// GenerateRegions creates a region map from a solution placement using
// round-robin BFS growth. For now, RegionOpts.Variance is ignored (always
// uniform sizes). Variable-size regions will be implemented in T-204.
func (g *BFSRegionGenerator) GenerateRegions(solution [][]bool, gridSize int, _ RegionOpts) ([][]int, error) {
	return GenerateRegionMap(solution, gridSize)
}
