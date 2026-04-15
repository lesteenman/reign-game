# Design Grill Summary: Phase 1 — First Playable (5x5 Standard)

## Final Design

Phase 1 delivers a playable 5x5 Standard Mode puzzle game in the browser. The Go backend generates puzzles on demand via a stateless API. The React frontend renders an interactive grid with the Tactile theme, validates solutions client-side, and persists game state in IndexedDB. Implementation is split into three milestones: playable grid (frontend, hardcoded puzzle), backend generator + API, and polish + persistence.

## Decisions

### Puzzle Engine

1. **Client-side validation only** — All 4 constraints (row, column, region, adjacency) checked locally. If all pass on a unique-solution puzzle, it's solved. No server round-trip.
2. **On-demand generation** — Generate a fresh puzzle per API call. No pre-generation pool. The 5x5 search space is small enough. Generator built TDD with a separate benchmark suite for performance comparison after heuristic changes.
3. **Region map format** — 2D int array where `regionMap[row][col]` is the region ID. Simple, flat, no shape definitions needed.

### Theme Architecture

4. **Hybrid CSS + React Context** — CSS custom properties for all color/spacing/shadow tokens (already in BRAND_GUIDELINES.md). Thin React Context for non-CSS parts: marker component, animation config. Fast theme switching via CSS class swap.
5. **Phase 1: Tactile theme only** — Architecture supports future themes without refactoring. One theme implementation ships.
6. **Rename "Minimalist" → "Tactile"** — The theme name reflects the visual style (layered shadows, thick borders), not the design philosophy. Update BRAND_GUIDELINES.md and GAME_DESIGN.md.
7. **Markers** — Filled circle for placement, cross for exclusion marks. Marker component is theme-swappable.

### Grid Interaction

8. **Three-tap cycle** — empty → exclusion mark → marker → empty. Exclusion first enables drag-to-exclude.
9. **Drag behavior determined by starting cell** — Start on empty cell: drag excludes. Start on excluded cell: drag clears. Start on marker: no drag.
10. **Real-time conflict highlighting** — Both conflicting markers pulse on constraint violation. Immediate feedback, no manual check.

### Game State

11. **IndexedDB with thin wrapper** — Structured data, large quota, works in service workers. Avoids localStorage→IndexedDB migration later.
12. **Timer via timestamps** — Persist `elapsedAtLastPause` + `lastResumedAt`. Write on cell state change, visibility change, and beforeunload. No 1-second polling.
13. **One active puzzle** — New puzzle fetch discards the old one. Completion records stored as lightweight list.

### Game Flow

14. **Two routes** — `/` (landing: resume/new puzzle) and `/play` (game with inline completion overlay).
15. **Landing page states** — Active puzzle: "Resume" + "New Puzzle" buttons. No active puzzle: "Play" button.
16. **Completion** — Timer stops, brief celebration animation, overlay shows solve time + "Play Again."

### PWA

17. **Manifest + app shell cache** — Installable, fast repeat loads. No custom install prompt.
18. **Offline resume works** — App shell cached, IndexedDB has puzzle data, client-side validation is local. Only new puzzle fetch requires connectivity.

### API Contract

19. **`GET /puzzles/generate?size=5&mode=standard`** — Returns `{puzzleId, gridSize, mode, regionMap}`. No solution in the response. Standard error codes (200, 400, 500).

## Deferred Items

- 7x7 and 9x9 grids (Phase 2)
- Double Queens mode (Phase 3)
- Puzzle database, curation, offline puzzle caching (Phase 4)
- Daily puzzles, accounts, premium, leaderboards (Phase 5+)
- Custom install prompt, background sync, push notifications

## Constraints & Assumptions

- The generator can produce valid 5x5 puzzles with unique solutions in under 500ms on Lambda.
- Client-side constraint checking is sufficient for validation because the generator guarantees unique solutions.
- IndexedDB is available in all target browsers (modern Chrome, Safari, Firefox).
- One hardcoded puzzle is enough for meaningful Milestone A playtesting.
