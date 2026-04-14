package generator

import (
	"fmt"
	"math/rand/v2"
)

// randomRegionMaxRetries is the maximum number of attempts to generate a valid
// random region map before giving up.
const randomRegionMaxRetries = 50

// GenerateRandomRegions creates a valid region map without requiring a solution
// as input. It partitions the grid into gridSize contiguous regions by placing
// random seed cells and growing them via BFS until all cells are assigned.
// Target sizes are computed from computeTargetSizes when variance > 0, or
// uniform (gridSize cells each) when variance is 0. It retries internally if
// the generated map fails validation.
func GenerateRandomRegions(gridSize int, opts RegionOpts) ([][]int, error) {
	numRegions := gridSize
	total := gridSize * gridSize

	// Compute target sizes.
	var targets []int
	if opts.Variance <= 0.0 {
		targets = make([]int, numRegions)
		for i := range targets {
			targets[i] = gridSize
		}
	} else {
		minSize := opts.MinSize
		if minSize < minRegionSize {
			minSize = minRegionSize
		}
		targets = computeTargetSizes(gridSize, opts.Variance, minSize)
	}

	effectiveMinSize := opts.MinSize
	if effectiveMinSize < minRegionSize {
		effectiveMinSize = minRegionSize
	}

	var lastErr error
	for attempt := 0; attempt < randomRegionMaxRetries; attempt++ {
		regionMap, err := generateRandomRegionsWithTargets(gridSize, numRegions, total, targets, effectiveMinSize)
		if err == nil {
			return regionMap, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("generating random regions: failed after %d retries: %w", randomRegionMaxRetries, lastErr)
}

// generateRandomRegionsWithTargets places random seed cells and grows regions
// via round-robin BFS to their target sizes.
func generateRandomRegionsWithTargets(gridSize, numRegions, total int, targets []int, minSize int) ([][]int, error) {
	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	regionMap := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		regionMap[r] = make([]int, gridSize)
		for c := 0; c < gridSize; c++ {
			regionMap[r][c] = -1
		}
	}

	// Place N random seed cells, one per region. Seeds must be non-adjacent
	// to each other to avoid trivially invalid configurations (though this
	// is not strictly required for region validity).
	seeds := placeRandomSeeds(gridSize, numRegions)
	if seeds == nil {
		return nil, fmt.Errorf("generating random regions: failed to place %d seeds on %dx%d grid", numRegions, gridSize, gridSize)
	}

	regionSize := make([]int, numRegions)
	frontiers := make([][]regionCell, numRegions)

	for rid, seed := range seeds {
		regionMap[seed.r][seed.c] = rid
		regionSize[rid] = 1
		for _, d := range dirs {
			nr, nc := seed.r+d[0], seed.c+d[1]
			if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
				frontiers[rid] = append(frontiers[rid], regionCell{nr, nc})
			}
		}
	}

	totalAssigned := numRegions

	for totalAssigned < total {
		// Refresh frontiers: remove already-assigned cells.
		for rid := 0; rid < numRegions; rid++ {
			filtered := frontiers[rid][:0]
			for _, fc := range frontiers[rid] {
				if regionMap[fc.r][fc.c] == -1 {
					filtered = append(filtered, fc)
				}
			}
			frontiers[rid] = filtered
		}

		// Find the region with the largest deficit that has frontier cells.
		order := rand.Perm(numRegions)
		bestRid := -1
		bestDeficit := 0
		for _, rid := range order {
			if len(frontiers[rid]) == 0 {
				continue
			}
			deficit := targets[rid] - regionSize[rid]
			if deficit > 0 && deficit > bestDeficit {
				bestDeficit = deficit
				bestRid = rid
			}
		}
		// Fallback: any region with frontier cells.
		if bestRid == -1 {
			for _, rid := range order {
				if len(frontiers[rid]) > 0 {
					bestRid = rid
					break
				}
			}
		}
		if bestRid == -1 {
			return nil, fmt.Errorf("generating random regions: stuck with %d/%d cells assigned", totalAssigned, total)
		}

		// Pick a random frontier cell.
		idx := rand.IntN(len(frontiers[bestRid]))
		chosen := frontiers[bestRid][idx]
		frontiers[bestRid][idx] = frontiers[bestRid][len(frontiers[bestRid])-1]
		frontiers[bestRid] = frontiers[bestRid][:len(frontiers[bestRid])-1]

		if regionMap[chosen.r][chosen.c] != -1 {
			continue
		}

		regionMap[chosen.r][chosen.c] = bestRid
		regionSize[bestRid]++
		totalAssigned++

		for _, d := range dirs {
			nr, nc := chosen.r+d[0], chosen.c+d[1]
			if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize && regionMap[nr][nc] == -1 {
				frontiers[bestRid] = append(frontiers[bestRid], regionCell{nr, nc})
			}
		}
	}

	if err := ValidateRegionMapWithMinSize(regionMap, gridSize, minSize); err != nil {
		return nil, fmt.Errorf("generating random regions: validation failed: %w", err)
	}

	return regionMap, nil
}

// placeRandomSeeds selects numRegions random cells on a gridSize x gridSize grid.
// Seeds are spread out to give each region room to grow. Returns nil if placement
// fails after reasonable attempts.
func placeRandomSeeds(gridSize, numRegions int) []regionCell {
	// Try to place seeds with some minimum spacing.
	// For small grids, just pick random distinct cells.
	const maxAttempts = 200

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate random permutation of all cells and pick the first numRegions.
		allCells := make([]regionCell, 0, gridSize*gridSize)
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				allCells = append(allCells, regionCell{r, c})
			}
		}
		rand.Shuffle(len(allCells), func(i, j int) {
			allCells[i], allCells[j] = allCells[j], allCells[i]
		})

		// Pick the first numRegions cells, checking minimum spacing.
		// Minimum spacing = 1 (no two seeds share a 4-neighbor).
		seeds := make([]regionCell, 0, numRegions)
		used := make([][]bool, gridSize)
		for r := 0; r < gridSize; r++ {
			used[r] = make([]bool, gridSize)
		}

		for _, cell := range allCells {
			if len(seeds) >= numRegions {
				break
			}
			if used[cell.r][cell.c] {
				continue
			}
			seeds = append(seeds, cell)
			// Mark cell and its 4-neighbors as used for spacing.
			used[cell.r][cell.c] = true
			dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
			for _, d := range dirs {
				nr, nc := cell.r+d[0], cell.c+d[1]
				if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
					used[nr][nc] = true
				}
			}
		}

		if len(seeds) == numRegions {
			return seeds
		}
	}

	// Fallback: just pick numRegions distinct random cells without spacing constraint.
	allCells := make([]regionCell, 0, gridSize*gridSize)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			allCells = append(allCells, regionCell{r, c})
		}
	}
	rand.Shuffle(len(allCells), func(i, j int) {
		allCells[i], allCells[j] = allCells[j], allCells[i]
	})
	if len(allCells) < numRegions {
		return nil
	}
	return allCells[:numRegions]
}
