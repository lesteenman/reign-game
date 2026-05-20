# `src/shared/game/components/grid/`

Custom hand-built grid UI. Five production files plus four unit tests. Moved from `src/components/grid/` in #176 (game library consolidated into `shared/game/`).

## Responsibility

The puzzle grid is the project's signature UI surface. It's intentionally NOT a Tamagui primitive — it's custom HTML/SVG built on `<div>` + `<svg>` so we control every pixel. The grid measures itself, lays out cells in a CSS grid, draws region borders as an SVG overlay, and routes pointer/touch events into the `useGame` reducer.

## Data flow

- **In:** Rendered by `GameBoard` (`shared/game/components/GameBoard.tsx`) — used by both the curation flow (`features/curation/pages/PlayPuzzlePage.tsx`) and the daily flow (`features/daily/screens/DailyGameBoard.tsx`). Receives `puzzle`, `cells`, `conflicts`, `isSolved`, `draggedCells`, and three event handlers (`onPointerDown`, `onPointerUp`, `onDragEnter`) all wired to `useGame`'s reducer.
- **Out:** `Cell.tsx` reads `useTheme()` to pick the right `MarkerComponent` / `ExclusionMarkComponent` and the animation class names. Otherwise no external calls.

## Files

- **`Grid.tsx`** — Top-level grid. Measures the parent container, computes a cell size in `[getMinCellSize(gridSize), 72]`, defers rendering until measured (avoids first-paint flicker — lesson 2). Lays cells out in `inline-grid`; renders `Cell` per (row, col) plus a `RegionBorderOverlay` on top. Handles touch-move via `document.elementFromPoint` to translate touch coordinates into (row, col). Imports `cellKey` from `engine/cellKey`.
- **`Cell.tsx`** — Single cell. Picks a background color (region fill / conflict bg / drag-highlighted mix), renders the theme's marker or exclusion mark, and handles mouse + touch with a `touchedRef` to suppress synthesized-mouse-after-touch events (lesson 1).
- **`Marker.tsx`** — Rounded-square SVG marker for the Tactile theme. Takes `size` + `regionIndex`.
- **`ExclusionMark.tsx`** — Small dot SVG for excluded cells. Takes `size`.
- **`RegionBorderOverlay.tsx`** — Computes horizontal + vertical region boundary segments + corner junctions in `O(n²)`. Memoized via `useMemo` over `(regionMap, gridSize, cellSize)` and `memo()` at the component boundary.

## State management

No local state worth surfacing beyond:
- `Grid.tsx` — `cellSize: number | null` (measured at mount, recomputed on resize via rAF-throttled `requestAnimationFrame`).
- `Cell.tsx` — `touchedRef: boolean` (suppresses synthesized mouse events after touch), `touchTimerRef` (timeout handle).
- `RegionBorderOverlay` — pure memoization.

## Rules specific to this directory

- **First-paint correctness.** `Grid.tsx` defers rendering until the parent is measured (`cellSize === null` branch). Don't render a placeholder size then resize — layout flicker is a user-visible bug.
- **Touch double-fire suppression.** Lesson 1 (top of `frontend/CLAUDE.md`): for touch/pointer interaction code, write Playwright e2e tests before unit tests. jsdom does not simulate synthesized mouse events.
- **`__TEST_ATTRS__` gating.** Test-only `data-testid` attributes are gated on the build-time constant `__TEST_ATTRS__` (defined in `vite.config.ts:18`) so production builds don't ship test IDs.

