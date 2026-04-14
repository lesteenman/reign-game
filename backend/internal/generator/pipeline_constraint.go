package generator

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// ConstraintAwarePipeline implements PipelineStrategy by generating a solution,
// then growing regions one cell at a time while actively checking how each
// assignment affects the solution count. It picks assignments that reduce the
// number of valid solutions most aggressively.
//
// This is slower per attempt but should need far fewer retries because the
// region growth is intentionally constraining.
type ConstraintAwarePipeline struct {
	solver SolverStrategy
}

// NewConstraintAwarePipeline returns a new ConstraintAwarePipeline that uses the
// given solver for solution counting.
func NewConstraintAwarePipeline(solver SolverStrategy) *ConstraintAwarePipeline {
	return &ConstraintAwarePipeline{solver: solver}
}

// Generate creates a puzzle by generating a solution, seeding regions from
// marker positions, then greedily assigning each remaining cell to the adjacent
// region that most reduces the solution count.
func (p *ConstraintAwarePipeline) Generate(gridSize int, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error) {
	deadline := time.Now().Add(opts.Timeout)

	mode := opts.Mode
	if mode == "" {
		mode = "standard"
	}

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("generating puzzle (constraint-aware): timeout after %v", opts.Timeout)
		}

		solution := p.solver.GenerateSolution(gridSize, markersPerUnit)
		if solution == nil {
			continue
		}

		regionMap := p.growConstraintAware(solution, gridSize, markersPerUnit, deadline)
		if regionMap == nil {
			continue
		}

		// Final uniqueness check.
		if p.solver.CountSolutions(regionMap, gridSize, markersPerUnit, 2) != 1 {
			continue
		}

		return &model.Puzzle{
			ID:        "",
			GridSize:  gridSize,
			Mode:      mode,
			RegionMap: regionMap,
			Solution:  solution,
		}, nil
	}
}

// growConstraintAware seeds regions from marker positions and grows them by
// assigning each frontier cell to the adjacent region that yields the fewest
// solutions. To keep performance manageable, it uses a heuristic: instead of
// checking every candidate cell against every region, it grows one region at
// a time and samples a limited number of candidates per step.
func (p *ConstraintAwarePipeline) growConstraintAware(solution [][]bool, gridSize, markersPerUnit int, deadline time.Time) [][]int {
	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	numRegions := gridSize
	total := gridSize * gridSize

	regionMap := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		regionMap[r] = make([]int, gridSize)
		for c := 0; c < gridSize; c++ {
			regionMap[r][c] = -1
		}
	}

	// Seed regions from marker positions.
	regionSize := make([]int, numRegions)
	if markersPerUnit >= 2 {
		// Double Queens: pair markers into regions.
		pairs, err := pairMarkersForDoubleQueens(solution, gridSize)
		if err != nil {
			return nil
		}
		for rid, pair := range pairs {
			for _, m := range pair {
				regionMap[m[0]][m[1]] = rid
				regionSize[rid]++
			}
		}
	} else {
		// Standard mode: one marker per region.
		markerIdx := 0
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					regionMap[r][c] = markerIdx
					regionSize[markerIdx] = 1
					markerIdx++
				}
			}
		}
		if markerIdx != numRegions {
			return nil
		}
	}

	targets := make([]int, numRegions)
	for i := range targets {
		targets[i] = gridSize
	}

	totalAssigned := 0
	for _, s := range regionSize {
		totalAssigned += s
	}

	// Grow regions round-robin by deficit, but use constraint-aware selection
	// for which frontier cell to add. Heuristic: pick the candidate that creates
	// the most "irregular" boundary (measured by number of same-region neighbors,
	// fewer = more irregular = more constraining).
	for totalAssigned < total {
		if time.Now().After(deadline) {
			return nil
		}

		// Collect all frontier cells: unassigned cells adjacent to at least one region.
		type candidate struct {
			r, c, rid int
			score     int // lower = more irregular = preferred
		}
		var candidates []candidate

		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if regionMap[r][c] != -1 {
					continue
				}

				// Find adjacent regions.
				adjacentRegions := make(map[int]bool)
				for _, d := range dirs {
					nr, nc := r+d[0], c+d[1]
					if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
						rid := regionMap[nr][nc]
						if rid >= 0 && regionSize[rid] < targets[rid] {
							adjacentRegions[rid] = true
						}
					}
				}

				for rid := range adjacentRegions {
					// Score: count how many of this cell's 4-neighbors are in the
					// same region. Fewer same-region neighbors = more irregular
					// boundary = more constraining.
					sameNeighbors := 0
					for _, d := range dirs {
						nr, nc := r+d[0], c+d[1]
						if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
							if regionMap[nr][nc] == rid {
								sameNeighbors++
							}
						}
					}
					candidates = append(candidates, candidate{r, c, rid, sameNeighbors})
				}
			}
		}

		if len(candidates) == 0 {
			// Fallback: allow overflowing regions.
			for r := 0; r < gridSize; r++ {
				for c := 0; c < gridSize; c++ {
					if regionMap[r][c] != -1 {
						continue
					}
					for _, d := range dirs {
						nr, nc := r+d[0], c+d[1]
						if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
							rid := regionMap[nr][nc]
							if rid >= 0 {
								candidates = append(candidates, candidate{r, c, rid, 0})
								break
							}
						}
					}
				}
			}
			if len(candidates) == 0 {
				return nil
			}
		}

		// Sort candidates: prefer regions with larger deficit, then lower score
		// (more irregular). Instead of sorting, just find the best candidate.
		// Among candidates, prefer those whose region has the largest deficit.
		bestDeficit := 0
		for _, cand := range candidates {
			deficit := targets[cand.rid] - regionSize[cand.rid]
			if deficit > bestDeficit {
				bestDeficit = deficit
			}
		}

		// Filter to candidates near the best deficit (within 1 to allow variety).
		var bestCandidates []candidate
		for _, cand := range candidates {
			deficit := targets[cand.rid] - regionSize[cand.rid]
			if deficit >= bestDeficit-1 {
				bestCandidates = append(bestCandidates, cand)
			}
		}
		if len(bestCandidates) == 0 {
			bestCandidates = candidates
		}

		// Among those, prefer lower score (more irregular boundary).
		minScore := 5
		for _, cand := range bestCandidates {
			if cand.score < minScore {
				minScore = cand.score
			}
		}

		var finalCandidates []candidate
		for _, cand := range bestCandidates {
			if cand.score <= minScore {
				finalCandidates = append(finalCandidates, cand)
			}
		}
		if len(finalCandidates) == 0 {
			finalCandidates = bestCandidates
		}

		// Pick a random one from the finalists.
		chosen := finalCandidates[rand.IntN(len(finalCandidates))]
		regionMap[chosen.r][chosen.c] = chosen.rid
		regionSize[chosen.rid]++
		totalAssigned++
	}

	validateMin := minRegionSize
	if markersPerUnit >= 2 {
		validateMin = minRegionSizeDouble
	}
	if err := ValidateRegionMapWithMinSize(regionMap, gridSize, validateMin); err != nil {
		return nil
	}

	return regionMap
}
