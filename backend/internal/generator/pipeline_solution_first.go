package generator

import (
	"fmt"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// SolutionFirstPipeline implements PipelineStrategy using the original approach:
// generate a random solution, grow regions around it, then verify uniqueness.
// This works well for small grids (5x5) but struggles at 7x7+ because randomly
// generated region maps around a specific solution rarely have a unique solution.
type SolutionFirstPipeline struct {
	solver  SolverStrategy
	regions RegionStrategy
}

// NewSolutionFirstPipeline returns a new SolutionFirstPipeline with the given
// solver and region strategies.
func NewSolutionFirstPipeline(solver SolverStrategy, regions RegionStrategy) *SolutionFirstPipeline {
	return &SolutionFirstPipeline{solver: solver, regions: regions}
}

// Generate creates a puzzle by generating a solution first, then growing regions
// around it, and verifying uniqueness. It retries until a unique puzzle is found
// or the timeout expires.
func (p *SolutionFirstPipeline) Generate(gridSize int, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error) {
	deadline := time.Now().Add(opts.Timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle: timeout after %v", opts.Timeout)
		}

		solution := p.solver.GenerateSolution(gridSize, markersPerUnit)
		if solution == nil {
			continue
		}

		regionMap, err := p.regions.GenerateRegions(solution, gridSize, opts.RegionOpts)
		if err != nil {
			continue
		}

		if p.solver.CountSolutions(regionMap, gridSize, markersPerUnit, 2) != 1 {
			continue
		}

		return &model.Puzzle{
			ID:        "",
			GridSize:  gridSize,
			RegionMap: regionMap,
			Solution:  solution,
		}, nil
	}
}

// NewDefaultPipeline returns a SolutionFirstPipeline with PropagationSolver and
// BFS regions. This is the default pipeline for backward compatibility.
func NewDefaultPipeline() PipelineStrategy {
	return NewSolutionFirstPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
}
