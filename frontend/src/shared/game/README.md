# shared/game

Cross-cutting game-board surface consumed by multiple flows (`features/curation/pages/PlayPuzzlePage` for curation/practice, `features/daily/screens/DailyGameBoard` for the daily challenge).

## Why "shared" and not "features/game"

There is no "game feature" in the Bulletproof React sense — no `features/game/pages/` route owns this UI exclusively. `GameBoard` is rendered by both `features/curation/pages/PlayPuzzlePage` (curation/practice flow) and `features/daily/screens/DailyGameBoard` (daily flow). Placing it in either feature would require a cross-feature import, which violates the independence rule. `shared/` is the correct layer for components that multiple features consume.

## Contents

- `components/GameBoard.tsx` — main game board (grid, timer, completion overlay, skip modal). Was extracted from the original `pages/GamePage` in #196, then `GamePage` itself was split into `features/curation/pages/PlayPuzzlePage` in #176's GamePage-split slice.
- `components/grid/*` — Grid, Cell, Marker, ExclusionMark, RegionBorderOverlay (moved from `src/components/grid/` in #204).
- `hooks/useGame.ts` — gameplay reducer (history stack, drag intent, conflicts, isSolved).
- `hooks/useTimer.ts` — pause/resume timer with `restore()` + `stop()`.
- `hooks/useGameStorage.ts` — IndexedDB CRUD wrapper. Consumed by both curation flow (`PlayPuzzlePage`) and daily flow (`DailyFlow` + `DailyGameBoard`).
- `hooks/useUpdatePuzzleStatus.ts` — TanStack `useMutation` wrapper around `services/puzzleService.updatePuzzleStatus`. Used by both flows (via `GameBoard`).
- `types/admin-verdict-surface.ts` — slot contract for the curation flow's `VerdictSurface` (slot pattern keeps `GameBoard` shared without importing `features/curation/`).

Tests are co-located alongside each source file.
