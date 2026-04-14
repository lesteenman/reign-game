# Design Grill Summary: Phase 2 — All Grid Sizes + Double Queens

## Final Design

Phase 2 extends Reign from 5x5 Standard-only to three grid sizes (5x5, 7x7, 9x9) and adds Double Queens mode (9x9 only). The backend ships first with a pluggable generator architecture: two solver strategies (backtrack, propagation) and two region generators (BFS, WFC) behind a common interface, selectable via API parameters. The API accepts arbitrary grid sizes 3-15 for experimentation; the UI restricts to validated sizes. An advanced mode on the landing page exposes all generator knobs for playtesting. The frontend follows in one pass with size/mode selectors, adaptive cell sizing, and parameterized constraint logic.

## Decisions

### Generator Architecture

1. **Pluggable generator strategies.** The generator is split into two pluggable components behind interfaces: a solver (places markers) and a region generator (grows regions around markers). Four combinations are playable:

   | Label | Solver | Region Generator |
   |-------|--------|-----------------|
   | backtrack-bfs | Backtrack (current) | Round-robin BFS (current) |
   | backtrack-wfc | Backtrack | Wave Function Collapse |
   | propagation-bfs | Constraint propagation | Round-robin BFS |
   | propagation-wfc | Constraint propagation | Wave Function Collapse |

   Code duplication between strategies is acceptable. Each strategy is self-contained so one can be tweaked without breaking others.

2. **Backtrack solver (current).** The existing row-by-row backtracker with random column permutations. Parameterized for `markersPerUnit` (1 for Standard, 2 for Double Queens). `placedCol`/`placedRegion` change from booleans to counters. `markerCols` changes from one column per row to a list.

3. **Constraint propagation solver (new).** After each marker placement, propagate consequences: mark same-row, same-column, same-region (at quota), and adjacent cells as unavailable. If any unit has zero available cells, backtrack immediately. If any unit has exactly one available cell, place automatically and propagate again. Produces different puzzle personality than pure backtrack because forced moves bias the solution distribution.

4. **BFS region generator (current).** Round-robin BFS growth from marker seed cells. Parameterized for variable region sizes.

5. **WFC region generator (new).** Wave Function Collapse for region assignment. Each cell's state is its region ID; constraints are connectivity and size bounds. "Lowest entropy first" heuristic fills tightest spots first, producing more organic, varied region shapes than BFS.

6. **All strategies available at all sizes.** Every solver + region generator combination is available for any grid size 3-15. The generate-solution, grow-regions, verify-uniqueness loop parameterizes on `gridSize`. Benchmarks validate performance for each strategy at each validated size. If a strategy exceeds 3 seconds at a given size, it needs optimization or gets flagged as unsuitable for that size. Single 5-second timeout.

### Region Sizes

7. **Variable region sizes.** Regions do not need to be the same size. A `regionVariance` parameter controls the spread: 0.0 (uniform) to 1.0 (maximum variation). Minimum region size is 3 cells for Standard mode, 4 cells for Double Queens. Maximum is determined by grid size and number of regions. The generator enforces these bounds.

8. **Region variance exposed as discrete slider.** Five stops: Uniform (0.0), Slight (0.25), Moderate (0.5), High (0.75), Wild (1.0). Discrete values are easier to reproduce during playtesting.

### Double Queens

9. **Double Queens: 9x9 only for Phase 2.** The UI only offers Double Queens at 9x9. The API accepts any size+mode combination for experimentation, but the standard UI restricts to validated combos. Expand to other sizes based on playtesting results.

### API

10. **Open API, restricted UI.** `GET /puzzles/generate` accepts:
    - `size`: 3-15 (UI offers 5, 7, 9)
    - `mode`: `standard` | `double` (UI offers `double` only at 9x9)
    - `solver`: `backtrack` | `propagation` (UI default: `backtrack`)
    - `regions`: `bfs` | `wfc` (UI default: `bfs`)
    - `regionVariance`: `0.0` - `1.0` (UI offers 5 discrete stops)

    Response shape unchanged: `{puzzleId, gridSize, mode, regionMap}`.

### Frontend

11. **Standard mode selector on landing page.** Four preset options: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens. Shown when starting a new game. No mid-game switching.

12. **Advanced mode toggle.** Collapsible section below the standard options. Exposes: free-form grid size (3-15), mode toggle, solver strategy dropdown, region generator dropdown, region variance slider. Not shown by default.

13. **Back navigation and dark mode in game page header.** A back button and dark mode toggle share a header row on the game page (e.g. back top-left, dark mode top-right). Exact placement deferred to UI/UX designer, but both must be consistently positioned.

14. **Adaptive cell sizing.** Minimum cell size is 44px for 5x5 and 7x7, 38px for 9x9. Scales down further for larger experimental sizes from advanced mode.

15. **Pass `markersPerUnit` to constraint functions.** `checkRowConstraint`, `checkColumnConstraint`, and `checkRegionConstraint` accept a `markersPerUnit` parameter. Flag when a unit has more than `markersPerUnit` markers. Adjacency constraint unchanged. Completion formula: `gridSize * markersPerUnit` markers with zero conflicts.

16. **Single active game.** Starting a new puzzle discards the current one regardless of mode or size.

### Implementation Order

17. **All backend first, then all frontend.** Milestone A: extend current generator for 7x7/9x9 Standard, add benchmarks. Milestone B: implement pluggable generator interface, constraint propagation solver, WFC region generator, variable region sizes, Double Queens mode. Benchmark all 4 strategy combos at 5/7/9 Standard + 9 Double. Milestone C: frontend — size/mode selectors, advanced mode, adaptive grid, parameterized constraints and validator, back navigation.

## Deferred Items

- Double Queens at sizes other than 9x9 (validate after playtesting)
- Difficulty rating for all sizes and modes (Phase 3)
- Multi-game persistence (Phase 4)
- Extended hit area or visual zoom for small cells (revisit after playtesting 9x9 on mobile)
- Additional generator strategies beyond the initial four (add as needed)
- Region variance tuning heuristics (part of future difficulty rating work)

## Constraints and Assumptions

- The current backtracking solver can handle 9x9 Standard within 5 seconds on Lambda. Benchmarks must confirm before shipping.
- WFC region generation is viable for this problem structure (connectivity + size constraints). If implementation proves too complex or slow, BFS remains the default.
- Minimum region size of 3 (Standard) / 4 (Double Queens) is sufficient for interesting puzzles. May need adjustment after playtesting.
- IndexedDB game state schema doesn't change. The existing `puzzle` field already stores `gridSize` and `mode`.
- The `PuzzleData` TypeScript type already has `mode: string`. Generator strategy parameters are not persisted in game state (only the resulting puzzle matters).
