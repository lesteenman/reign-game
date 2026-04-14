package generator

import "fmt"

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
