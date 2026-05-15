# `src/engine/`

Pure-TS puzzle domain. Three production files plus two unit tests.

## Responsibility

The "what is a Reign puzzle and when is one solved" domain knowledge — separated from React, IndexedDB, and the network. The architecture rule is strict: **`engine/` may only depend on external libs and import nothing from React, no `fetch`, no DOM**. Verified clean via `grep -rn "from 'react'\|fetch(\|document\.\|window\." engine/` — no matches.

## Data flow

- **In:** Imported by `hooks/useGame.ts`, every `pages/*` file that handles puzzles, `services/puzzleService.ts`, `services/verdictService.ts`, `services/adminService.ts`, `storage/types.ts`, `storage/utils.ts`, and every grid component. The engine is the single source of truth for `Mode`, `CellState`, `PuzzleData`, `Conflict`.
- **Out:** Nothing. Pure functions and types.

## Files

- **`types.ts`** — Type primitives: `CellState` ('empty' | 'excluded' | 'marked'), `Position`, `Conflict`, `PuzzleMetadata`, `MODES` (tuple), `Mode` (union), `isMode` (type guard), `PuzzleData`.
- **`constraints.ts`** — Four constraint checkers (`checkRowConstraint`, `checkColumnConstraint`, `checkRegionConstraint`, `checkAdjacencyConstraint`) plus `getAllConflicts` that runs all four and deduplicates by canonical pair key.
- **`validator.ts`** — Solution validator (36 LOC).

## State management

None. All exports are pure functions or types.

## Rules specific to this directory

- **No React, no I/O, no DOM.** Enforced at design-time and review-time by the architecture skill. This will become `@reign/core` when the daily mobile companion app needs to share domain code with the web client.
- **`MODES` is the source of truth.** The backend mirrors these in `handler.ModeStandard` / `ModeDouble`. Backend and frontend keep this list in lockstep.
- **`isMode` is the validation gate.** Every site that reads a mode from an untyped source (URL params, JSON response, localStorage) goes through `isMode` before type-asserting (lesson 3).

## Track 3 mapping

Unchanged location. The directory may be promoted to `@reign/core` at the workspace level later but for Track 3 it stays where it is.
