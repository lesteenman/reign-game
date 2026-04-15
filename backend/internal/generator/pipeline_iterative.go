package generator

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// maxSwapIterations is the maximum number of cell swaps to attempt before
// giving up on an initial solution+region pair.
const maxSwapIterations = 500

// IterativeRefinementPipeline implements PipelineStrategy by starting with a
// solution and regions, then iteratively swapping cells between adjacent regions
// to reduce the solution count until uniqueness is achieved.
type IterativeRefinementPipeline struct {
	solver  SolverStrategy
	regions RegionStrategy
}

// NewIterativeRefinementPipeline returns a new IterativeRefinementPipeline with
// the given solver and region strategies.
func NewIterativeRefinementPipeline(solver SolverStrategy, regions RegionStrategy) *IterativeRefinementPipeline {
	return &IterativeRefinementPipeline{solver: solver, regions: regions}
}

// Generate creates a puzzle by generating an initial solution and region map,
// then iteratively swapping non-marker cells between adjacent regions to reduce
// the number of valid solutions. It retries from scratch if max iterations are
// exceeded or the timeout expires.
func (p *IterativeRefinementPipeline) Generate(gridSize int, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error) {
	deadline := time.Now().Add(opts.Timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle (iterative): timeout after %v", opts.Timeout)
		}
		if opts.Ctx != nil {
			if err := opts.Ctx.Err(); err != nil {
				return nil, fmt.Errorf("generating puzzle (iterative): %w", err)
			}
		}

		solution := p.solver.GenerateSolution(gridSize, markersPerUnit)
		if solution == nil {
			continue
		}

		regionMap, err := p.regions.GenerateRegions(solution, gridSize, opts.RegionOpts)
		if err != nil {
			continue
		}

		minSize := minRegionSize
		if markersPerUnit >= 2 {
			minSize = minRegionSizeDouble
		}
		result := p.refine(regionMap, gridSize, markersPerUnit, minSize, deadline)
		if result == nil {
			continue
		}

		// Validate the refined region map.
		if err := ValidateRegionMapWithMinSize(result, gridSize, minSize); err != nil {
			continue
		}

		// Extract solution from the refined region map.
		sol, ok := Solve(result, gridSize, markersPerUnit)
		if !ok {
			continue
		}

		if !IsLogicallyDeducible(result, gridSize, markersPerUnit) {
			continue
		}

		return &model.Puzzle{
			ID:        "",
			GridSize:  gridSize,
			RegionMap: result,
			Solution:  sol,
		}, nil
	}
}

// refine iteratively swaps non-marker cells between adjacent regions to reduce
// the solution count. Returns the refined region map if uniqueness is achieved,
// or nil if max iterations are exceeded. The minSize parameter prevents regions
// from shrinking below the mode-specific minimum.
func (p *IterativeRefinementPipeline) refine(regionMap [][]int, gridSize, markersPerUnit, minSize int, deadline time.Time) [][]int {
	currentMap := copyRegionMap(regionMap, gridSize)
	currentCount := p.solver.CountSolutions(currentMap, gridSize, markersPerUnit, 2)

	if currentCount == 1 {
		return currentMap
	}
	if currentCount == 0 {
		return nil
	}

	for iter := 0; iter < maxSwapIterations; iter++ {
		if time.Now().After(deadline) {
			return nil
		}

		// Pick a random cell.
		r := rand.IntN(gridSize)
		c := rand.IntN(gridSize)

		// Find an adjacent cell in a different region.
		dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
		shuffledDirs := rand.Perm(4)

		swapped := false
		for _, di := range shuffledDirs {
			d := dirs[di]
			nr, nc := r+d[0], c+d[1]
			if nr < 0 || nr >= gridSize || nc < 0 || nc >= gridSize {
				continue
			}
			if currentMap[r][c] == currentMap[nr][nc] {
				continue
			}

			// Try swapping cell (r,c) to the neighbor's region.
			oldRid := currentMap[r][c]
			newRid := currentMap[nr][nc]

			// Count cells in old region — don't let it drop below minimum.
			oldSize := countRegionCells(currentMap, gridSize, oldRid)
			if oldSize <= minSize {
				continue
			}

			currentMap[r][c] = newRid

			// Validate contiguity of the old region after removing this cell.
			if !isRegionStillContiguous(currentMap, gridSize, oldRid) {
				currentMap[r][c] = oldRid
				continue
			}

			// Check if the swap improves things.
			newCount := p.solver.CountSolutions(currentMap, gridSize, markersPerUnit, 2)
			if newCount == 1 {
				return currentMap
			}
			if newCount == 0 || newCount > currentCount {
				// Swap made things worse or invalid, undo.
				currentMap[r][c] = oldRid
				continue
			}

			// Swap is at least as good (newCount <= currentCount), keep it.
			currentCount = newCount
			swapped = true
			break
		}

		_ = swapped
	}

	return nil
}

// copyRegionMap creates a deep copy of a region map.
func copyRegionMap(regionMap [][]int, gridSize int) [][]int {
	result := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		result[r] = make([]int, gridSize)
		copy(result[r], regionMap[r])
	}
	return result
}

// countRegionCells counts how many cells belong to a specific region.
func countRegionCells(regionMap [][]int, gridSize, rid int) int {
	count := 0
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if regionMap[r][c] == rid {
				count++
			}
		}
	}
	return count
}

// isRegionStillContiguous checks whether a specific region is still contiguous
// in the given region map using BFS.
func isRegionStillContiguous(regionMap [][]int, gridSize, rid int) bool {
	// Collect all cells belonging to this region.
	var cells [][2]int
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if regionMap[r][c] == rid {
				cells = append(cells, [2]int{r, c})
			}
		}
	}

	if len(cells) == 0 {
		return false
	}

	return checkContiguous(cells) == nil
}
