# Spec: Interactive Grid

Covers R-016 (interactive grid component) and R-017 (client-side validation).

## Requirements

### GR-01: Grid Component

- `src/components/grid/Grid.tsx` renders an N×N CSS Grid
- Receives puzzle data (`gridSize`, `regionMap`) and game state (`cells[][]`, conflict info)
- Each cell colored by region: background from `--region-{regionId}-fill`
- Region boundaries: 2.5px `--color-ink` border between adjacent cells with different region IDs
- No visible border between cells in the same region
- Responsive: fills available width on mobile, max-width constrained on desktop
- Tests: renders correct number of cells, applies region colors

### GR-02: Cell Component

- `src/components/grid/Cell.tsx` renders a single grid cell
- Displays one of: empty, exclusion mark (from theme), marker (from theme)
- Conflict state: applies error color + conflict animation class when flagged
- Tap handler: cycles cell state (empty → excluded → marked → empty)
- Touch/mouse event handling for drag support
- Tests: tap cycles state correctly, conflict class applied when flagged

### GR-03: Three-Tap Interaction

- First tap on empty cell → exclusion mark
- Second tap (on excluded cell) → marker
- Third tap (on marked cell) → empty
- State transitions are immediate, no confirmation step
- Tests: full cycle from empty through all states and back

### GR-04: Drag Gesture

- Drag starting on an empty cell → applies exclusion marks to all empty cells in the drag path
- Drag starting on an excluded cell → clears exclusion marks on all excluded cells in the drag path
- Drag starting on a marker → no drag behavior (single tap still works)
- Drag skips cells that don't match the drag intent (markers during exclude-drag, empty cells during clear-drag)
- Works on touch (touchstart/touchmove/touchend) and mouse (mousedown/mousemove/mouseup)
- Debounce or throttle drag events to prevent excessive re-renders
- Tests: drag across empty cells excludes them, drag across excluded cells clears them, drag from marker does nothing

### GR-05: Conflict Detection

- `src/engine/constraints.ts` exports constraint check functions:
  - `checkRowConstraint(cells, gridSize): Conflict[]`
  - `checkColumnConstraint(cells, gridSize): Conflict[]`
  - `checkRegionConstraint(cells, regionMap): Conflict[]`
  - `checkAdjacencyConstraint(cells, gridSize): Conflict[]`
- `Conflict` type: `{ cells: [Position, Position] }` — the pair of conflicting markers
- All four checks run on every cell state change
- Tests: table-driven for each constraint with valid and conflicting states

### GR-06: Conflict Highlighting

- Conflicting markers receive an error visual treatment: error color border/background + pulse animation
- Both markers in a conflict pair are highlighted (not just the most recently placed)
- Highlights clear immediately when the conflict is resolved
- Multiple simultaneous conflicts are all shown
- Tests: conflicting markers have conflict class, resolved conflicts remove it

### GR-07: Solution Validation

- `src/engine/validator.ts` exports `validateSolution(cells, regionMap, gridSize): boolean`
- Returns true when: exactly N markers on an N×N grid AND all four constraints pass (zero conflicts)
- Completion is detected on every cell state change after validation passes
- Tests: valid solution returns true, incomplete board returns false, board with conflicts returns false

## Acceptance Criteria

All GR-01 through GR-07 requirements pass. Grid renders with correct region colors and boundaries. Three-tap cycle and drag work on touch and mouse. Conflicts highlight in real-time. Solution validation detects completion.
