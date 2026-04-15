package generator

import (
	"fmt"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// RegionFirstPipeline implements PipelineStrategy by generating regions first
// (without a solution), then checking if the region map admits exactly one
// solution. This avoids the solution-first trap where regions tailored to one
// solution paradoxically admit many others.
//
// Many random region maps have unique solutions because the combination of
// region + row + column + adjacency constraints is highly restrictive.
type RegionFirstPipeline struct {
	solver SolverStrategy
}

// NewRegionFirstPipeline returns a new RegionFirstPipeline that uses the given
// solver for uniqueness checking and solution extraction.
func NewRegionFirstPipeline(solver SolverStrategy) *RegionFirstPipeline {
	return &RegionFirstPipeline{solver: solver}
}

// Generate creates a puzzle by generating random region maps and checking each
// for a unique solution. It retries until a unique-solution region map is found
// or the timeout expires.
func (p *RegionFirstPipeline) Generate(gridSize int, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error) {
	deadline := time.Now().Add(opts.Timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle (region-first): timeout after %v", opts.Timeout)
		}
		if opts.Ctx != nil {
			if err := opts.Ctx.Err(); err != nil {
				return nil, fmt.Errorf("generating puzzle (region-first): %w", err)
			}
		}

		regionMap, err := GenerateRandomRegions(gridSize, opts.RegionOpts)
		if err != nil {
			continue
		}

		solCount := p.solver.CountSolutions(regionMap, gridSize, markersPerUnit, 2)
		if solCount != 1 {
			continue
		}

		if opts.Deducible && !IsLogicallyDeducible(regionMap, gridSize, markersPerUnit) {
			continue
		}

		// Extract the unique solution.
		solution := p.extractSolution(regionMap, gridSize, markersPerUnit)
		if solution == nil {
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

// extractSolution finds the unique solution for a region map by counting
// exactly 1 solution and using the solver to produce it. It reuses the
// backtracking solver infrastructure via the Solve function.
func (p *RegionFirstPipeline) extractSolution(regionMap [][]int, gridSize, markersPerUnit int) [][]bool {
	sol, ok := Solve(regionMap, gridSize, markersPerUnit)
	if !ok {
		return nil
	}
	return sol
}
