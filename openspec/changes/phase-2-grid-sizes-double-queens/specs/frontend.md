# Spec: Frontend Changes

Covers R-022 (grid size selector) and R-033 (mode toggle).

## Requirements

### FE-01: Standard Selectors on Landing Page

- Landing page shows four preset buttons when starting a new game:
  - 5x5 Standard
  - 7x7 Standard
  - 9x9 Standard
  - 9x9 Double Queens
- Buttons are styled per BRAND_GUIDELINES.md
- Selected option is visually highlighted
- Flow with no active puzzle: selectors visible + "Play" button
- Flow with active puzzle: "Resume" + "New Puzzle". "New Puzzle" reveals selectors.
- Tests: all four options render, selection state updates, Play navigates with correct params

### FE-02: Advanced Mode Toggle

- Collapsible "Advanced" section below the standard selectors
- Collapsed by default, toggled by a text link or small button
- When expanded, shows:
  - Grid size: number input or slider, range 3-15
  - Mode: toggle between Standard and Double
  - Solver: dropdown with `Backtrack` and `Propagation`
  - Region generator: dropdown with `BFS` and `WFC`
  - Region variance: labeled slider with 5 discrete stops (Uniform, Slight, Moderate, High, Wild)
- Advanced options override the standard selector when used
- Tests: toggle reveals/hides advanced section, all controls update state, Play sends correct params

### FE-03: Region Variance Slider

- 5 discrete stops: Uniform (0.0), Slight (0.25), Moderate (0.5), High (0.75), Wild (1.0)
- Labels shown below or beside the slider
- Current value visually indicated
- Tests: slider snaps to discrete values, value maps correctly to API parameter

### FE-04: API Client Expansion

- `puzzleService.ts` accepts all generator parameters:
  ```typescript
  interface GenerateOptions {
    size: number
    mode: 'standard' | 'double'
    solver?: 'backtrack' | 'propagation'
    regions?: 'bfs' | 'wfc'
    regionVariance?: number
  }
  ```
- Optional params omitted from URL when using defaults (clean URLs for standard play)
- Tests: URL construction includes all params when set, omits defaults

### FE-05: Adaptive Cell Sizing

- Cell size minimum varies by grid size:
  - 5x5 and 7x7: 44px minimum (unchanged)
  - 9x9: 38px minimum
  - 10+: no minimum, full-width divided by grid size
- Maximum remains 72px
- Grid still defers rendering until measured (no flicker)
- Tests: cell size computation returns correct values for 5x5, 7x7, 9x9, 12x12

### FE-06: Constraint Parameterization

- `checkRowConstraint`, `checkColumnConstraint`, `checkRegionConstraint` accept `markersPerUnit` parameter
- Flag conflicts when count > `markersPerUnit` (was > 1)
- `checkAdjacencyConstraint` unchanged (no two markers adjacent, regardless of mode)
- `getAllConflicts` passes `markersPerUnit` through
- `markersPerUnit` derived from puzzle mode: 1 for `standard`, 2 for `double`
- Tests: table-driven for Standard (existing tests, markersPerUnit=1) and Double Queens (markersPerUnit=2, flag at 3+ markers per unit)

### FE-07: Completion Condition

- `validateSolution` accepts `markersPerUnit`
- Solved when `markerCount === gridSize * markersPerUnit && conflicts.length === 0`
- `useGame` hook derives `markersPerUnit` from the puzzle's mode field
- Tests: Standard completion at N markers, Double Queens completion at 2N markers, incomplete and conflicting states return false

### FE-08: Game Page Header

- Back button and dark mode toggle in a shared header row on the game page
- Back button navigates to landing page (`/`)
- Dark mode toggle is the existing `useDarkMode` hook, moved from PageShell into the game header
- Exact layout (positions, sizing) deferred to UI/UX designer, but both controls must be consistently positioned
- Tests: back button navigates to `/`, dark mode toggle changes theme class

### FE-09: Back Navigation

- Navigating back from the game page does NOT discard the active puzzle
- The game state remains in IndexedDB; returning to `/play` resumes it
- If the user explicitly starts a new puzzle, the old one is discarded (existing behavior)
- Tests: navigate away and back, game state preserved

### FE-10: GamePage Size Awareness

- GamePage works with any grid size (not hardcoded to 5)
- Grid, constraints, and completion logic all use the puzzle's `gridSize` and `mode`
- Loading state while fetching larger puzzles (which may take longer to generate)
- Tests: render a 9x9 puzzle, verify grid has 81 cells

## Acceptance Criteria

All FE-01 through FE-10 pass. Standard selectors offer 4 presets. Advanced mode exposes all generator parameters. Grid renders correctly at all sizes. Constraint checking and completion work for both Standard and Double Queens. Back navigation preserves game state. Game page header has both controls.
