# shared/game

Cross-cutting game-board surface consumed by multiple flows (pages/GamePage for curation/practice, features/daily for the daily challenge).

## Why "shared" and not "features/game"

There is no "game feature" in the Bulletproof React sense — no `features/game/pages/` route owns this UI exclusively. `GameBoard` and `VerdictSurface` are rendered by both `pages/GamePage` (curation/practice flow) and `features/daily/screens/DailyGameBoard` (daily flow). Placing them in either feature would require a cross-feature import, which violates the independence rule. `shared/` is the correct layer for components that multiple features and pages consume.

## Contents

- `components/GameBoard.tsx` — main game board (grid, timer, completion overlay, skip modal). Extracted from `pages/GamePage`.
- `components/VerdictSurface.tsx` — admin verdict surface (completion + skip variants). Moved from `components/game/`.
- `hooks/useSubmitVerdict.ts` — TanStack `useMutation` wrapper for the verdict service.
- `hooks/useUpdatePuzzleStatus.ts` — TanStack `useMutation` wrapper for the puzzle status service.

Tests are co-located: `components/GameBoard.test.tsx`, `components/GameBoardWallClock.test.tsx`, `components/VerdictSurface.test.tsx`.
