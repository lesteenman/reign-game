package generator

import (
	"fmt"
	"math/rand/v2"
)

// regionCell is a row/column coordinate used during region map generation.
type regionCell struct{ r, c int }

// GenerateRegionMap builds a contiguous region map around a solution grid.
// Each region contains exactly one solution marker and exactly gridSize cells.
// Region IDs are assigned based on the order markers appear (row 0's marker gets
// region 0, row 1's marker gets region 1, etc.).
func GenerateRegionMap(solution [][]bool, gridSize int) ([][]int, error) {
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

		// Find the smallest region that has frontier cells.
		// First pass: prefer regions below gridSize.
		order := rand.Perm(gridSize)
		bestRid := -1
		bestSize := target + 1
		for _, rid := range order {
			if len(frontiers[rid]) == 0 {
				continue
			}
			if regionSize[rid] < gridSize && regionSize[rid] < bestSize {
				bestSize = regionSize[rid]
				bestRid = rid
			}
		}
		// Fallback: if all regions with frontiers are already at gridSize, pick any
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

// ValidateRegionMap checks that a region map is well-formed for the given grid size.
// It verifies: grid dimensions match, all region IDs are in [0, gridSize),
// each region has exactly gridSize cells, and each region is contiguous
// (connected via horizontal/vertical adjacency).
func ValidateRegionMap(regionMap [][]int, gridSize int) error {
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

	// Check each region has exactly gridSize cells and is contiguous.
	for id := 0; id < gridSize; id++ {
		cells := regionCells[id]
		if len(cells) != gridSize {
			return fmt.Errorf("validating region map: region %d has cell count %d, expected %d", id, len(cells), gridSize)
		}
		if err := checkContiguous(cells); err != nil {
			return fmt.Errorf("validating region map: region %d is not contiguous: %w", id, err)
		}
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
