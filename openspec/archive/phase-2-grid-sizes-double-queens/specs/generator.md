# Spec: Generator Architecture

Covers R-020 (generator/solver for larger grids), R-030 (Double Queens solver), R-031 (Double Queens generator).

## Requirements

### GN-01: Pluggable Generator Interface

- `internal/generator/strategy.go` defines `SolverStrategy` and `RegionStrategy` interfaces
- `SolverStrategy`: `GenerateSolution(gridSize, markersPerUnit, timeout)` and `CountSolutions(regionMap, gridSize, markersPerUnit)`
- `RegionStrategy`: `GenerateRegions(solution, gridSize, opts)` where `opts` includes `Variance` and `MinSize`
- The top-level `Generate` function accepts strategy choices and delegates to the selected implementations
- Tests: verify `Generate` calls through to the provided strategies

### GN-02: Backtrack Solver Strategy

- `internal/generator/solver_backtrack.go` implements `SolverStrategy`
- Refactored from the current `solver.go` — same algorithm, parameterized for `markersPerUnit`
- `placedCol` and `placedRegion` change from `[]bool` to `[]int` counters
- `markerCols` changes from `[]int` to `[][]int` (list of columns per row)
- For Standard (`markersPerUnit=1`): picks 1 column per row (same as Phase 1)
- For Double Queens (`markersPerUnit=2`): picks 2 columns per row, iterating over C(N,2) column pairs
- Adjacency check updated to handle multiple markers per row
- Tests: table-driven for Standard (existing tests migrated) and Double Queens at 9x9

### GN-03: Constraint Propagation Solver Strategy

- `internal/generator/solver_propagation.go` implements `SolverStrategy`
- Same backtracking structure as GN-02, plus forward propagation after each marker placement:
  1. Mark same-row, same-column, and adjacent cells as unavailable
  2. Mark same-region cells as unavailable if region has reached `markersPerUnit` markers
  3. If any row/column/region has zero available cells → fail fast, backtrack
  4. If any row/column/region has exactly one available cell → place automatically, propagate again
- The propagation chain continues until no more forced moves exist or a contradiction is found
- Tests: table-driven, same test cases as GN-02 (both strategies must produce valid puzzles). Additional tests verifying forced-move detection.

### GN-04: BFS Region Strategy

- `internal/generator/regions_bfs.go` implements `RegionStrategy`
- Refactored from the current `region.go` — same round-robin BFS growth, parameterized for variable region sizes
- Each region is seeded at a solution marker cell
- For Double Queens: each region is seeded at one of its 2 markers. The second marker must end up in the same region during growth. If growth fails to include both markers, retry with a different seed assignment.
- Target region sizes drawn from a distribution controlled by `Variance` (see GN-06)
- Tests: table-driven for uniform sizes (regression) and variable sizes

### GN-05: WFC Region Strategy

- `internal/generator/regions_wfc.go` implements `RegionStrategy`
- Wave Function Collapse for region assignment:
  1. Each cell starts with all region IDs (0 to gridSize-1) as possibilities
  2. Solution marker cells are pre-assigned to their region (collapsed)
  3. Iteratively: pick the uncollapsed cell with fewest possibilities (lowest entropy)
  4. Collapse it to one region (random weighted choice)
  5. Propagate constraints: connectivity (cell must be reachable from at least one same-region cell), size bounds (region can't exceed its target size)
  6. If contradiction (cell has zero possibilities) → backtrack or retry
- Target region sizes drawn from the same distribution as GN-04 (see GN-06)
- For Double Queens: both markers for a region are pre-assigned before WFC runs
- Tests: generated regions pass `ValidateRegionMap`. Regions are contiguous. Size bounds respected. Compare shape variety against BFS (qualitative, not automated).

### GN-06: Variable Region Sizes

- `RegionOpts.Variance` controls size distribution: 0.0 = all regions have `gridSize` cells, 1.0 = maximum spread
- `RegionOpts.MinSize`: 3 for Standard mode, 4 for Double Queens
- Size distribution: compute target sizes for each region such that they sum to `gridSize * gridSize`, respect `MinSize`, and spread increases with `Variance`
- A shared utility function generates the target size array from `gridSize`, `Variance`, and `MinSize`
- Tests: target sizes sum correctly, respect MinSize, uniform at Variance=0.0, varied at Variance=1.0

### GN-07: Region Validation Update

- `ValidateRegionMap` updated to accept variable region sizes (no longer requires exactly `gridSize` cells per region)
- Validates: all IDs in range, each region contiguous, each region meets minimum size, total cells = gridSize^2, number of regions = gridSize
- Tests: existing tests still pass. New tests for variable-size valid maps and edge cases (region below min size, total mismatch).

### GN-08: Double Queens Marker-to-Region Pairing

- For Double Queens, the 2N solution markers must be assigned into N pairs (one pair per region)
- The pairing must allow contiguous region growth around both markers in each pair
- Implementation approach TBD — benchmark both:
  - Pairing during solution generation (markers assigned to regions as they're placed)
  - Pairing after solution generation (find a valid pairing, then grow regions)
- At least one approach must be implemented. If both are implemented, expose as a parameter for benchmarking.
- Tests: each region contains exactly 2 markers. Regions are contiguous around both markers.

### GN-09: Generation Timeout

- Single 5-second timeout for all sizes and strategies
- The generation loop retries internally until timeout
- If timeout exceeded: return descriptive error
- Tests: verify timeout triggers correctly (mock slow strategy)

### GN-10: Benchmark Suite

- `internal/generator/generator_bench_test.go` expanded to cover:
  - All 4 strategy combinations (backtrack-bfs, backtrack-wfc, propagation-bfs, propagation-wfc)
  - Validated sizes: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens
  - Metrics: average generation time, p95, retry count, success rate
- Benchmarks run via `task bench:backend`
- Region variance benchmarks: run at Variance 0.0 and 0.5 for each combo
- Tests: benchmarks compile and run (correctness tested elsewhere)

## Acceptance Criteria

All GN-01 through GN-10 requirements pass. Both solver strategies produce valid puzzles with unique solutions at all tested sizes. Both region strategies produce valid contiguous regions. Variable region sizes work with both strategies. Benchmarks run and report metrics for all strategy combos.
