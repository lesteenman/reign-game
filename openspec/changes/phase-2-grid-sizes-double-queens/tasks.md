# Phase 2: Implementation Tasks

## Milestones

```
Milestone A (Larger Grids)              → Backend: 7x7, 9x9 Standard working + benchmarked
Milestone B (Generator Arch + DQ)       → Backend: pluggable strategies, Double Queens, all benchmarks
Milestone C (Frontend)                  → Full UI: selectors, advanced mode, adaptive grid, constraints
```

## Status

| Task | Title | Milestone | Status |
|------|-------|-----------|--------|
| T-200 | Parameterize solver for markersPerUnit | A | [x] |
| T-201 | Lift size restriction + region validation | A | [x] |
| T-202 | Benchmark suite for larger grids | A | [x] |
| T-203 | Pluggable generator interface | B | [x] |
| T-204 | Variable region sizes | B | [x] |
| T-205 | Constraint propagation solver | B | [x] |
| T-206 | WFC region generator | B | [x] |
| T-207 | Double Queens mode | B | [x] |
| T-208 | API strategy parameters | B | [x] |
| T-209 | Full benchmark suite | B | [x] |
| T-210 | Constraint parameterization | C | [x] |
| T-211 | Adaptive grid sizing | C | [x] |
| T-212 | API client expansion | C | [x] |
| T-213 | Landing page selectors | C | [x] |
| T-214 | Game page header + back nav | C | [x] |
| T-215 | Milestone C integration + playtest | C | [ ] |

## Tasks

### Milestone A: Larger Grids (backend, extend current generator)

Goal: 7x7 and 9x9 Standard Mode puzzles generate correctly. Benchmarks validate performance.

#### T-200: Parameterize Solver for markersPerUnit

- **Roadmap:** R-020, R-030
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-02)
- **Work:**
  - Refactor `solver.go`: `placedCol`/`placedRegion` from `[]bool` to `[]int`, `markerCols` from `[]int` to `[][]int`
  - `solve()` accepts `markersPerUnit` parameter
  - For `markersPerUnit=1`: behavior identical to Phase 1 (regression safety)
  - For `markersPerUnit=2`: iterate C(N,2) column pairs per row
  - Migrate existing solver tests to new signature
  - TDD: all existing tests pass with `markersPerUnit=1`
- **Acceptance:** GN-02 passes. Existing solver tests still pass.
- **Commit after completion.**

#### T-201: Lift Size Restriction + Region Validation Update

- **Roadmap:** R-020
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-07), specs/api.md (AP-01)
- **Work:**
  - `generator.go`: remove `gridSize != 5` restriction
  - `ValidateRegionMap`: accept variable region sizes (no longer requires exactly `gridSize` cells per region). Validate: IDs in range, contiguous, min size, total = gridSize^2, region count = gridSize
  - `handler/generate.go`: expand `size` validation to 3-15
  - TDD: generate 7x7 and 9x9 Standard puzzles, verify all constraints
- **Acceptance:** GN-07 and AP-01 pass for size expansion. 7x7 and 9x9 Standard puzzles generate.
- **Depends on:** T-200
- **Commit after completion.**

#### T-202: Benchmark Suite for Larger Grids

- **Roadmap:** R-020
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-10)
- **Work:**
  - Expand `generator_bench_test.go`: add 7x7 and 9x9 Standard benchmarks
  - Measure average, p95, retry count
  - Update `task bench:backend` if needed
  - Document baseline numbers in a comment or test output
- **Acceptance:** GN-10 partially passes (benchmarks run for current strategy at 5x5, 7x7, 9x9 Standard).
- **Depends on:** T-201
- **Commit after completion.**

**→ BENCHMARK CHECKPOINT: Review 9x9 performance. If >3s consistently, optimize before proceeding.**

### Milestone B: Generator Architecture + Double Queens

Goal: pluggable interface, all 4 strategy combos working, Double Queens at 9x9, full benchmark suite.

#### T-203: Pluggable Generator Interface

- **Roadmap:** R-020
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-01)
- **Work:**
  - `internal/generator/strategy.go`: define `SolverStrategy` and `RegionStrategy` interfaces
  - Refactor `Generate()` to accept strategy choices
  - Wrap existing solver as `BacktrackSolver` implementing `SolverStrategy`
  - Wrap existing region generator as `BFSRegionGenerator` implementing `RegionStrategy`
  - All existing tests and benchmarks pass through the new interface
- **Acceptance:** GN-01 passes. No behavior change from Phase 1 when using defaults.
- **Depends on:** T-202
- **Commit after completion.**

#### T-204: Variable Region Sizes

- **Roadmap:** R-031
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-06)
- **Work:**
  - Shared utility: `computeTargetSizes(gridSize, variance, minSize)` returns `[]int`
  - At variance=0.0: all sizes equal `gridSize`
  - At variance>0: redistribute cells, respecting minSize and total = gridSize^2
  - BFS region generator uses target sizes instead of fixed `gridSize`
  - TDD: target sizes sum correctly, respect min, uniform at 0.0, varied at 1.0
- **Acceptance:** GN-06 passes.
- **Depends on:** T-203
- **Commit after completion.**

#### T-205: Constraint Propagation Solver

- **Roadmap:** R-020
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-03)
- **Work:**
  - `internal/generator/solver_propagation.go` implementing `SolverStrategy`
  - Availability tracking: 2D grid of available cells per row
  - After placement: propagate unavailability to row, column, region, adjacent cells
  - Forced-move detection: if a unit has exactly one available cell, place and propagate
  - Contradiction detection: if a unit has zero available cells, fail fast
  - TDD: same test cases as backtrack solver (both must produce valid puzzles), plus forced-move tests
- **Acceptance:** GN-03 passes.
- **Depends on:** T-203
- **Commit after completion.**

#### T-206: WFC Region Generator

- **Roadmap:** R-031
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-05)
- **Work:**
  - `internal/generator/regions_wfc.go` implementing `RegionStrategy`
  - Cell possibility tracking: each cell has a set of possible region IDs
  - Pre-assign marker cells to their region
  - Collapse loop: pick lowest-entropy cell, assign region, propagate constraints
  - Constraints: connectivity (reachable from same-region cell), target size bounds
  - Backtrack or retry on contradiction
  - Supports variable region sizes via target size array
  - TDD: generated regions pass `ValidateRegionMap`, contiguous, size bounds respected
- **Acceptance:** GN-05 passes.
- **Depends on:** T-204
- **Commit after completion.**

#### T-207: Double Queens Mode

- **Roadmap:** R-030, R-031
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-08)
- **Work:**
  - Double Queens marker-to-region pairing logic
  - Implement at least one pairing approach (during or after solution generation)
  - If time permits, benchmark both approaches
  - Verify both solver strategies work with `markersPerUnit=2`
  - Verify both region strategies handle 2 markers per region
  - TDD: 9x9 Double Queens puzzles generate with unique solutions, each region has exactly 2 markers
- **Acceptance:** GN-08 passes. 9x9 Double Queens puzzles generate with all 4 strategy combos.
- **Depends on:** T-205, T-206
- **Commit after completion.**

#### T-208: API Strategy Parameters

- **Roadmap:** R-020, R-030
- **Agent:** backend-dev
- **Spec:** specs/api.md (AP-01, AP-02, AP-03, AP-04)
- **Work:**
  - `handler/generate.go`: parse `solver`, `regions`, `regionVariance` params
  - Map string values to strategy implementations
  - Defaults: `backtrack`, `bfs`, `0.0`
  - Add `mode=double` support with `markersPerUnit=2`
  - TDD: table-driven for all param combos, defaults, and error cases
- **Acceptance:** AP-01 through AP-04 pass.
- **Depends on:** T-207
- **Commit after completion.**

#### T-209: Full Benchmark Suite

- **Roadmap:** R-020, R-030
- **Agent:** backend-dev
- **Spec:** specs/generator.md (GN-10)
- **Work:**
  - Benchmark all 4 strategy combos at: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens
  - Region variance benchmarks at 0.0 and 0.5
  - Report: average, p95, retry count, success rate
  - Flag strategies exceeding 3s at any validated size
- **Acceptance:** GN-10 fully passes. Benchmark report available.
- **Depends on:** T-208
- **Commit after completion.**

**→ BENCHMARK REVIEW: Review all strategy performance. Decide which combos to expose in standard UI.**

### Milestone C: Frontend

Goal: full UI with selectors, advanced mode, adaptive grid, parameterized constraints, back navigation.

#### T-210: Constraint Parameterization

- **Roadmap:** R-030
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-06, FE-07)
- **Work:**
  - `constraints.ts`: add `markersPerUnit` param to row, column, region checks and `getAllConflicts`
  - `validator.ts`: add `markersPerUnit`, completion = `gridSize * markersPerUnit`
  - `useGame.ts`: derive `markersPerUnit` from puzzle mode
  - Migrate existing tests to new signatures (markersPerUnit=1)
  - TDD: Double Queens constraint tests (flag at 3+, not 2+)
- **Acceptance:** FE-06 and FE-07 pass.
- **Commit after completion.**

#### T-211: Adaptive Grid Sizing

- **Roadmap:** R-022
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-05, FE-10)
- **Work:**
  - `Grid.tsx`: cell size minimum varies by grid size (44px for <=7, 38px for 9, scale-down for 10+)
  - Verify grid renders at 7x7, 9x9, and arbitrary sizes
  - TDD: cell size computation for various grid sizes
- **Acceptance:** FE-05 and FE-10 pass.
- **Commit after completion.**

#### T-212: API Client Expansion

- **Roadmap:** R-022, R-033
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-04)
- **Work:**
  - `puzzleService.ts`: accept `GenerateOptions` with all params
  - Optional params omitted from URL when using defaults
  - TDD: URL construction for standard play and advanced mode
- **Acceptance:** FE-04 passes.
- **Commit after completion.**

#### T-213: Landing Page Selectors

- **Roadmap:** R-022, R-033
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-01, FE-02, FE-03)
- **Work:**
  - Four preset buttons: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens
  - Advanced toggle: grid size input, mode toggle, solver dropdown, regions dropdown, variance slider
  - Variance slider: 5 discrete stops with labels
  - Wire selections into `generatePuzzle` call
  - Styled per BRAND_GUIDELINES.md
  - TDD: selector rendering, selection state, advanced toggle, Play navigates with correct params
- **Acceptance:** FE-01, FE-02, FE-03 pass.
- **Depends on:** T-212
- **Commit after completion.**

#### T-214: Game Page Header + Back Navigation

- **Roadmap:** R-033
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-08, FE-09)
- **Work:**
  - Game page header: back button + dark mode toggle in shared row
  - Back button navigates to `/` without discarding game state
  - Dark mode toggle uses existing `useDarkMode` hook
  - Layout: deferred to UI/UX designer for exact positions, but both present and consistently placed
  - TDD: back navigation preserves state, dark mode toggle works
- **Acceptance:** FE-08 and FE-09 pass.
- **Commit after completion.**

#### T-215: Milestone C Integration + Playtest

- **Roadmap:** R-022, R-033
- **Agent:** frontend-dev
- **Spec:** Cross-spec integration
- **Work:**
  - Wire everything together: selectors → API → grid → constraints → completion
  - Test full loop at all 4 preset sizes + advanced mode combos
  - Verify advanced mode parameters reach the API correctly
  - Visual QA on mobile viewport
  - Test back navigation + resume flow
- **Acceptance:** Full game loop works at all sizes and modes. Advanced mode generates puzzles with selected strategies. Back navigation preserves state.
- **Depends on:** T-210, T-211, T-213, T-214
- **Commit after completion.**

**→ PLAYTEST CHECKPOINT: Review all sizes, modes, and strategy combos before merge.**

## Execution Summary

| Milestone | Tasks | Agent(s) | Notes |
|-----------|-------|----------|-------|
| A | T-200, T-201, T-202 | backend-dev | Sequential. Benchmark checkpoint after T-202. |
| B | T-203, T-204, T-205, T-206, T-207, T-208, T-209 | backend-dev | T-205 and T-206 can run in parallel after T-203/T-204. T-207 depends on both. |
| C | T-210, T-211, T-212, T-213, T-214, T-215 | frontend-dev | T-210, T-211, T-212 can run in parallel. T-213 depends on T-212. T-215 integrates all. |

## Dependency Graph

```
T-200 (parameterize solver)
  └→ T-201 (lift size restriction)
       └→ T-202 (initial benchmarks) ── CHECKPOINT
            └→ T-203 (pluggable interface)
                 ├→ T-204 (variable regions) → T-206 (WFC regions) ──┐
                 └→ T-205 (propagation solver) ──────────────────────┤
                                                                     └→ T-207 (Double Queens)
                                                                          └→ T-208 (API params)
                                                                               └→ T-209 (full benchmarks) ── CHECKPOINT

T-210 (constraint params) ────────┐
T-211 (adaptive grid) ────────────┤
T-212 (API client) → T-213 (selectors) ──┤
T-214 (game header + back nav) ───┤
                                  └→ T-215 (integration + playtest) ── CHECKPOINT
```

## Notes

- Milestone A and B are backend-only. No frontend changes until Milestone C.
- Benchmark checkpoints are mandatory: review numbers before proceeding. If 9x9 Standard exceeds 3s, optimize before Milestone B.
- All tasks follow TDD: failing test first, then implementation, then refactor.
- Code duplication between generator strategies is acceptable — each strategy is self-contained.
- Milestone C frontend tasks can begin as soon as Milestone B is complete and benchmarked.
- Playtest checkpoint after Milestone C covers all sizes, both modes, and all 4 strategy combos.
