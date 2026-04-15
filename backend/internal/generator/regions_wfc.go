package generator

import (
	"fmt"
	"math/rand/v2"
)

// wfcMaxRetries is the maximum number of restart attempts before WFC gives up.
const wfcMaxRetries = 100

// WFCRegionGenerator implements RegionStrategy using Wave Function Collapse.
// It produces more organic, varied region shapes compared to BFS growth by
// iteratively collapsing cells with the fewest remaining possibilities and
// propagating size constraints after each collapse. Contiguity is enforced
// by only allowing a cell to be assigned to a region it is adjacent to.
type WFCRegionGenerator struct {
	markersPerUnit int // 1 for standard, 2 for double
}

// NewWFCRegionGenerator returns a new WFCRegionGenerator for standard mode
// (1 marker per region).
func NewWFCRegionGenerator() *WFCRegionGenerator {
	return &WFCRegionGenerator{markersPerUnit: 1}
}

// NewWFCRegionGeneratorDouble returns a new WFCRegionGenerator for double mode
// (2 markers per region).
func NewWFCRegionGeneratorDouble() *WFCRegionGenerator {
	return &WFCRegionGenerator{markersPerUnit: 2}
}

// GenerateRegions creates a region map from a solution placement using
// Wave Function Collapse. Each cell starts with all region IDs as possibilities.
// Marker cells are pre-collapsed to seed regions. The algorithm iteratively
// picks the lowest-entropy uncollapsed cell, assigns it a region, and propagates
// constraints. On contradiction (zero possibilities for any cell), it restarts
// from scratch up to wfcMaxRetries times.
//
// For double mode (markersPerUnit=2), markers are paired into regions using
// greedy closest-pair matching before WFC growth begins.
func (g *WFCRegionGenerator) GenerateRegions(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error) {
	defaultMin := minRegionSize
	if g.markersPerUnit >= 2 {
		defaultMin = minRegionSizeDouble
	}
	minSize := opts.MinSize
	if minSize < defaultMin {
		minSize = defaultMin
	}

	var targets []int
	if opts.Variance <= 0.0 {
		targets = make([]int, gridSize)
		for i := range targets {
			targets[i] = gridSize
		}
	} else {
		targets = computeTargetSizes(gridSize, opts.Variance, minSize)
	}

	validateMinSize := minRegionSize
	if g.markersPerUnit >= 2 {
		validateMinSize = minRegionSizeDouble
	}

	var lastErr error
	for attempt := 0; attempt < wfcMaxRetries; attempt++ {
		regionMap, err := g.tryWFC(solution, gridSize, targets)
		if err == nil {
			if valErr := ValidateRegionMapWithMinSize(regionMap, gridSize, validateMinSize); valErr == nil {
				return regionMap, nil
			}
			// Validation failed — treat as contradiction, retry.
			continue
		}
		lastErr = err
	}

	return nil, fmt.Errorf("generating WFC region map: failed after %d retries, last error: %w", wfcMaxRetries, lastErr)
}

// tryWFC attempts one full WFC pass. Returns the region map or an error on contradiction.
//
// The algorithm works as follows:
//  1. Initialize each cell with all region IDs as possibilities.
//  2. Collapse marker cells to their assigned regions (seeds).
//  3. Compute each uncollapsed cell's "frontier set": the set of regions it is
//     directly adjacent to (via a collapsed neighbor). Only frontier regions are
//     valid collapse options — this guarantees contiguity.
//  4. Pick the frontier cell with the fewest valid options (lowest entropy).
//  5. Collapse it to one of its frontier regions (weighted by deficit from target).
//  6. Propagate: update frontier sets, remove full regions from all possibility sets.
//  7. Repeat until all cells are collapsed or a contradiction occurs.
func (g *WFCRegionGenerator) tryWFC(solution [][]bool, gridSize int, targets []int) ([][]int, error) {
	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	numRegions := gridSize
	total := gridSize * gridSize

	// regionMap[r][c] = assigned region ID, or -1 if unassigned.
	regionMap := make([][]int, gridSize)
	for r := 0; r < gridSize; r++ {
		regionMap[r] = make([]int, gridSize)
		for c := 0; c < gridSize; c++ {
			regionMap[r][c] = -1
		}
	}

	regionSize := make([]int, numRegions)
	collapsedCount := 0

	// Collapse a cell to a region.
	collapse := func(r, c, rid int) {
		regionMap[r][c] = rid
		regionSize[rid]++
		collapsedCount++
	}

	// fullRegion[rid] = true if region rid has reached its target size.
	fullRegion := make([]bool, numRegions)

	// Seed markers. For standard mode (markersPerUnit=1), marker i gets
	// region i. For double mode (markersPerUnit=2), markers are paired
	// using greedy closest-pair matching.
	if g.markersPerUnit >= 2 {
		pairs, err := pairMarkersForDoubleQueens(solution, gridSize)
		if err != nil {
			return nil, fmt.Errorf("seeding WFC regions: %w", err)
		}
		for rid, pair := range pairs {
			for _, m := range pair {
				collapse(m[0], m[1], rid)
			}
		}
	} else {
		markerIdx := 0
		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if solution[r][c] {
					collapse(r, c, markerIdx)
					markerIdx++
				}
			}
		}
		if markerIdx != numRegions {
			return nil, fmt.Errorf("found %d markers, expected %d", markerIdx, numRegions)
		}
	}

	// Main collapse loop.
	for collapsedCount < total {
		// Update full-region flags.
		for rid := 0; rid < numRegions; rid++ {
			fullRegion[rid] = regionSize[rid] >= targets[rid]
		}

		// For each uncollapsed cell, compute the set of adjacent regions (frontier set).
		// A cell can only be assigned to a region it is directly adjacent to,
		// and that region must not be full.
		type frontierCell struct {
			r, c    int
			options []int // region IDs this cell can be assigned to
		}
		var candidates []frontierCell

		for r := 0; r < gridSize; r++ {
			for c := 0; c < gridSize; c++ {
				if regionMap[r][c] != -1 {
					continue
				}

				// Find unique adjacent region IDs that are not full.
				seen := make([]bool, numRegions)
				var options []int
				for _, d := range dirs {
					nr, nc := r+d[0], c+d[1]
					if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
						rid := regionMap[nr][nc]
						if rid >= 0 && !seen[rid] && !fullRegion[rid] {
							seen[rid] = true
							options = append(options, rid)
						}
					}
				}

				if len(options) > 0 {
					candidates = append(candidates, frontierCell{r, c, options})
				}
			}
		}

		if len(candidates) == 0 {
			// No frontier cells but grid not complete — some cells are stranded.
			// Check if there are uncollapsed cells that are only adjacent to full regions.
			// Allow them to join a full region (overflow) as a fallback.
			for r := 0; r < gridSize; r++ {
				for c := 0; c < gridSize; c++ {
					if regionMap[r][c] != -1 {
						continue
					}
					seen := make([]bool, numRegions)
					var options []int
					for _, d := range dirs {
						nr, nc := r+d[0], c+d[1]
						if nr >= 0 && nr < gridSize && nc >= 0 && nc < gridSize {
							rid := regionMap[nr][nc]
							if rid >= 0 && !seen[rid] {
								seen[rid] = true
								options = append(options, rid)
							}
						}
					}
					if len(options) > 0 {
						candidates = append(candidates, frontierCell{r, c, options})
					}
				}
			}
			if len(candidates) == 0 {
				return nil, fmt.Errorf("contradiction: no frontier cells with %d/%d collapsed", collapsedCount, total)
			}
		}

		// Find minimum entropy (fewest options).
		minOptions := numRegions + 1
		for _, fc := range candidates {
			if len(fc.options) < minOptions {
				minOptions = len(fc.options)
			}
		}

		// Collect all candidates with minimum entropy.
		var minCandidates []frontierCell
		for _, fc := range candidates {
			if len(fc.options) == minOptions {
				minCandidates = append(minCandidates, fc)
			}
		}

		// Pick a random minimum-entropy cell.
		chosen := minCandidates[rand.IntN(len(minCandidates))]

		// Choose a region weighted by deficit from target.
		rid := g.chooseWeighted(chosen.options, targets, regionSize)
		collapse(chosen.r, chosen.c, rid)
	}

	return regionMap, nil
}

// chooseWeighted selects a region from the given options, weighted by how many
// cells each region still needs (deficit from target). Regions further from
// their target are more likely to be chosen, which helps produce balanced sizes.
func (g *WFCRegionGenerator) chooseWeighted(options, targets, regionSize []int) int {
	if len(options) == 1 {
		return options[0]
	}

	totalWeight := 0
	weights := make([]int, len(options))
	for i, rid := range options {
		deficit := targets[rid] - regionSize[rid]
		if deficit <= 0 {
			deficit = 1 // minimal weight for full regions (overflow fallback)
		}
		w := deficit * deficit
		weights[i] = w
		totalWeight += w
	}

	pick := rand.IntN(totalWeight)
	for i, w := range weights {
		pick -= w
		if pick < 0 {
			return options[i]
		}
	}

	return options[len(options)-1]
}
