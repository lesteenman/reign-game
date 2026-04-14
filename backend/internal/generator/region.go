package generator

import (
	"fmt"
	"math/rand/v2"
)

// computeTargetSizes returns target cell counts for each of the gridSize regions.
// At variance=0.0, all regions get exactly gridSize cells (uniform).
// At higher variance, sizes spread out while respecting minSize and summing to gridSize*gridSize.
// The algorithm redistributes cells pairwise: pick two regions, move a cell from
// the larger to the smaller direction with probability weighted by variance.
func computeTargetSizes(gridSize int, variance float64, minSize int) []int {
	numRegions := gridSize
	total := gridSize * gridSize

	sizes := make([]int, numRegions)
	for i := range sizes {
		sizes[i] = gridSize // start uniform
	}

	if variance <= 0.0 {
		return sizes
	}

	// Number of redistribution rounds scales with variance and grid area.
	rounds := int(variance * float64(total))

	for round := 0; round < rounds; round++ {
		// Pick two distinct random regions.
		a := rand.IntN(numRegions)
		b := rand.IntN(numRegions - 1)
		if b >= a {
			b++
		}

		// Transfer one cell from a to b (or vice versa, randomly).
		donor, recipient := a, b
		if rand.IntN(2) == 0 {
			donor, recipient = b, a
		}

		// Only transfer if donor won't drop below minSize.
		if sizes[donor] <= minSize {
			continue
		}

		sizes[donor]--
		sizes[recipient]++
	}

	// Verify sum is correct (should be invariant, but defensive).
	sum := 0
	for _, s := range sizes {
		sum += s
	}
	if sum != total {
		// Fix any drift by adjusting the largest region.
		maxIdx := 0
		for i, s := range sizes {
			if s > sizes[maxIdx] {
				maxIdx = i
			}
		}
		sizes[maxIdx] += total - sum
	}

	return sizes
}

// regionCell is a row/column coordinate used during region map generation.
type regionCell struct{ r, c int }

// GenerateRegionMap builds a contiguous region map around a solution grid.
// Each region contains exactly one solution marker and exactly gridSize cells.
// Region IDs are assigned based on the order markers appear (row 0's marker gets
// region 0, row 1's marker gets region 1, etc.).
func GenerateRegionMap(solution [][]bool, gridSize int) ([][]int, error) {
	// Uniform target sizes: each region gets gridSize cells.
	targets := make([]int, gridSize)
	for i := range targets {
		targets[i] = gridSize
	}
	return generateRegionMapWithTargets(solution, gridSize, targets)
}

// GenerateRegionMapVariable builds a contiguous region map with variable region sizes.
// targetSizes specifies the number of cells each region should have.
// Each region contains exactly one solution marker.
func GenerateRegionMapVariable(solution [][]bool, gridSize int, targetSizes []int) ([][]int, error) {
	return generateRegionMapWithTargets(solution, gridSize, targetSizes)
}

// GenerateRegionMapDouble builds a contiguous region map around a Double Queens
// solution grid. Each region contains exactly 2 solution markers and gridSize
// cells. Region IDs are assigned by pairing markers: the first 2 markers (in
// row-major order) get region 0, the next 2 get region 1, etc.
func GenerateRegionMapDouble(solution [][]bool, gridSize int) ([][]int, error) {
	targets := make([]int, gridSize)
	for i := range targets {
		targets[i] = gridSize
	}
	return generateRegionMapDoubleWithTargets(solution, gridSize, targets)
}

// GenerateRegionMapDoubleVariable builds a contiguous region map for Double
// Queens with variable region sizes. Each region contains exactly 2 solution
// markers. targetSizes specifies the number of cells each region should have.
func GenerateRegionMapDoubleVariable(solution [][]bool, gridSize int, targetSizes []int) ([][]int, error) {
	return generateRegionMapDoubleWithTargets(solution, gridSize, targetSizes)
}

// pairMarkersForDoubleQueens assigns 2*gridSize markers into gridSize regions
// (2 markers per region). It uses a greedy approach: pair markers that are
// close together so regions can grow contiguously around them.
func pairMarkersForDoubleQueens(solution [][]bool, gridSize int) ([][2][2]int, error) {
	// Collect all marker positions.
	var markers [][2]int
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if solution[r][c] {
				markers = append(markers, [2]int{r, c})
			}
		}
	}

	expectedMarkers := 2 * gridSize
	if len(markers) != expectedMarkers {
		return nil, fmt.Errorf("pairing markers: found %d markers, expected %d", len(markers), expectedMarkers)
	}

	// Greedy pairing: repeatedly find the closest unpaired pair.
	paired := make([]bool, len(markers))
	pairs := make([][2][2]int, 0, gridSize)

	for len(pairs) < gridSize {
		bestDist := gridSize * gridSize * 2
		bestI, bestJ := -1, -1

		for i := 0; i < len(markers); i++ {
			if paired[i] {
				continue
			}
			for j := i + 1; j < len(markers); j++ {
				if paired[j] {
					continue
				}
				dr := markers[i][0] - markers[j][0]
				dc := markers[i][1] - markers[j][1]
				dist := dr*dr + dc*dc
				if dist < bestDist {
					bestDist = dist
					bestI = i
					bestJ = j
				}
			}
		}

		if bestI < 0 {
			return nil, fmt.Errorf("pairing markers: could not find pair")
		}

		paired[bestI] = true
		paired[bestJ] = true
		pairs = append(pairs, [2][2]int{markers[bestI], markers[bestJ]})
	}

	return pairs, nil
}

// generateRegionMapDoubleWithTargets grows regions from paired markers for
// Double Queens mode using round-robin BFS.
func generateRegionMapDoubleWithTargets(solution [][]bool, gridSize int, targetSizes []int) ([][]int, error) {
	pairs, err := pairMarkersForDoubleQueens(solution, gridSize)
	if err != nil {
		return nil, fmt.Errorf("generating double region map: %w", err)
	}

	regionMap := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		regionMap[r] = make([]int, gridSize)
		for c := 0; c < gridSize; c++ {
			regionMap[r][c] = -1
		}
	}

	regionSize := make([]int, gridSize)
	for rid, pair := range pairs {
		for _, m := range pair {
			regionMap[m[0]][m[1]] = rid
			regionSize[rid]++
		}
	}

	// Grow regions using round-robin BFS (same as standard mode).
	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	totalAssigned := 2 * gridSize // 2 markers per region seeded

	// Initialize frontiers for each region.
	frontiers := make([][]regionCell, gridSize)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if regionMap[r][c] < 0 {
				continue
			}
			rid := regionMap[r][c]
			for _, d := range dirs {
				nr, nc := r+d[0], c+d[1]
				if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize && regionMap[nr][nc] == -1 {
					frontiers[rid] = append(frontiers[rid], regionCell{nr, nc})
				}
			}
		}
	}

	target := gridSize * gridSize
	for totalAssigned < target {
		// Refresh frontiers.
		for rid := 0; rid < gridSize; rid++ {
			filtered := frontiers[rid][:0]
			for _, fc := range frontiers[rid] {
				if regionMap[fc.r][fc.c] == -1 {
					filtered = append(filtered, fc)
				}
			}
			frontiers[rid] = filtered
		}

		order := rand.Perm(gridSize)
		bestRid := -1
		bestDeficit := 0
		for _, rid := range order {
			if len(frontiers[rid]) == 0 {
				continue
			}
			deficit := targetSizes[rid] - regionSize[rid]
			if deficit > 0 && deficit > bestDeficit {
				bestDeficit = deficit
				bestRid = rid
			}
		}
		if bestRid == -1 {
			for _, rid := range order {
				if len(frontiers[rid]) > 0 {
					bestRid = rid
					break
				}
			}
		}
		if bestRid == -1 {
			return nil, fmt.Errorf("generating double region map: stuck with %d/%d cells assigned", totalAssigned, target)
		}

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

	if err := ValidateRegionMapWithMinSize(regionMap, gridSize, minRegionSizeDouble); err != nil {
		return nil, fmt.Errorf("generating double region map: validation failed: %w", err)
	}

	return regionMap, nil
}

// generateRegionMapWithTargets is the internal implementation that grows regions
// to their specified target sizes using round-robin BFS.
func generateRegionMapWithTargets(solution [][]bool, gridSize int, targetSizes []int) ([][]int, error) {
	regionMap := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		regionMap[r] = make([]int, gridSize)
		for c := 0; c < gridSize; c++ {
			regionMap[r][c] = -1
		}
	}

	// Seed each region with its solution marker cell.
	regionSize := make([]int, gridSize)

	markerIdx := 0
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if solution[r][c] {
				rid := markerIdx
				markerIdx++
				regionMap[r][c] = rid
				regionSize[rid] = 1
			}
		}
	}

	if markerIdx != gridSize {
		return nil, fmt.Errorf("generating region map: found %d markers, expected %d", markerIdx, gridSize)
	}

	// Grow regions using round-robin BFS. Each round, iterate regions in shuffled
	// order; the smallest region with frontier cells claims one random neighbor.
	// No hard cap on region size during growth -- sizes are checked at the end.
	// Prefer non-full regions, but allow full regions to grow if no other region
	// can claim cells (prevents stranding).
	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	totalAssigned := gridSize

	// Initialize frontiers for each region.
	frontiers := make([][]regionCell, gridSize)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			if regionMap[r][c] < 0 {
				continue
			}
			rid := regionMap[r][c]
			for _, d := range dirs {
				nr, nc := r+d[0], c+d[1]
				if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize && regionMap[nr][nc] == -1 {
					frontiers[rid] = append(frontiers[rid], regionCell{nr, nc})
				}
			}
		}
	}

	target := gridSize * gridSize
	for totalAssigned < target {
		// Refresh all frontiers: remove assigned cells.
		for rid := 0; rid < gridSize; rid++ {
			filtered := frontiers[rid][:0]
			for _, fc := range frontiers[rid] {
				if regionMap[fc.r][fc.c] == -1 {
					filtered = append(filtered, fc)
				}
			}
			frontiers[rid] = filtered
		}

		// Find the smallest region (relative to its target) that has frontier cells.
		// First pass: prefer regions below their target size.
		order := rand.Perm(gridSize)
		bestRid := -1
		bestDeficit := 0 // how far below target the best candidate is
		for _, rid := range order {
			if len(frontiers[rid]) == 0 {
				continue
			}
			deficit := targetSizes[rid] - regionSize[rid]
			if deficit > 0 && deficit > bestDeficit {
				bestDeficit = deficit
				bestRid = rid
			}
		}
		// Fallback: if all regions with frontiers are at or above their target, pick any
		// (this avoids stranding cells; validation will catch size imbalance).
		if bestRid == -1 {
			for _, rid := range order {
				if len(frontiers[rid]) > 0 {
					bestRid = rid
					break
				}
			}
		}
		if bestRid == -1 {
			return nil, fmt.Errorf("generating region map: stuck with %d/%d cells assigned", totalAssigned, target)
		}

		// Pick a random frontier cell from the chosen region.
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

	if err := ValidateRegionMap(regionMap, gridSize); err != nil {
		return nil, fmt.Errorf("generating region map: validation failed: %w", err)
	}

	return regionMap, nil
}

// minRegionSize is the default minimum number of cells a region must have (standard mode).
const minRegionSize = 3

// minRegionSizeDouble is the minimum number of cells a region must have in double mode.
const minRegionSizeDouble = 4

// ValidateRegionMap checks that a region map is well-formed for the given grid size.
// It verifies: grid dimensions match, all region IDs are in [0, gridSize),
// number of distinct regions equals gridSize, total cells equal gridSize*gridSize,
// each region has at least minRegionSize cells, and each region is contiguous
// (connected via horizontal/vertical adjacency).
func ValidateRegionMap(regionMap [][]int, gridSize int) error {
	return ValidateRegionMapWithMinSize(regionMap, gridSize, minRegionSize)
}

// ValidateRegionMapWithMinSize checks that a region map is well-formed for the
// given grid size and minimum region size. It verifies: grid dimensions match,
// all region IDs are in [0, gridSize), number of distinct regions equals gridSize,
// total cells equal gridSize*gridSize, each region has at least minSize cells,
// and each region is contiguous (connected via horizontal/vertical adjacency).
func ValidateRegionMapWithMinSize(regionMap [][]int, gridSize int, minSize int) error {
	// Check row count.
	if len(regionMap) != gridSize {
		return fmt.Errorf("validating region map: row count %d does not match grid size %d", len(regionMap), gridSize)
	}

	// Check column counts and collect cells per region.
	regionCells := make(map[int][][2]int)
	for r, row := range regionMap {
		if len(row) != gridSize {
			return fmt.Errorf("validating region map: column count %d in row %d does not match grid size %d", len(row), r, gridSize)
		}
		for c, id := range row {
			if id < 0 || id >= gridSize {
				return fmt.Errorf("validating region map: region ID %d at (%d,%d) is out of range [0, %d)", id, r, c, gridSize)
			}
			regionCells[id] = append(regionCells[id], [2]int{r, c})
		}
	}

	// Check that there are exactly gridSize distinct regions.
	if len(regionCells) != gridSize {
		return fmt.Errorf("validating region map: region count %d does not match grid size %d", len(regionCells), gridSize)
	}

	// Check each region meets minimum size, is contiguous, and total cells sum correctly.
	totalCells := 0
	for id := 0; id < gridSize; id++ {
		cells := regionCells[id]
		if len(cells) < minSize {
			return fmt.Errorf("validating region map: region %d has %d cells, below minimum %d", id, len(cells), minSize)
		}
		if err := checkContiguous(cells); err != nil {
			return fmt.Errorf("validating region map: region %d is not contiguous: %w", id, err)
		}
		totalCells += len(cells)
	}

	if totalCells != gridSize*gridSize {
		return fmt.Errorf("validating region map: total cell count %d does not match expected %d", totalCells, gridSize*gridSize)
	}

	return nil
}

// checkContiguous verifies that a set of cells forms a single connected component
// via horizontal/vertical adjacency using BFS.
func checkContiguous(cells [][2]int) error {
	if len(cells) == 0 {
		return nil
	}

	cellSet := make(map[[2]int]bool, len(cells))
	for _, c := range cells {
		cellSet[c] = true
	}

	// BFS from the first cell.
	visited := make(map[[2]int]bool)
	queue := [][2]int{cells[0]}
	visited[cells[0]] = true

	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			nb := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if cellSet[nb] && !visited[nb] {
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
	}

	if len(visited) != len(cells) {
		return fmt.Errorf("found %d connected cells out of %d", len(visited), len(cells))
	}
	return nil
}
