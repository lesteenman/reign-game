# `src/hooks/`

Reusable hooks. Three production files plus three unit tests.

## Responsibility

Encapsulate gameplay state machines and IndexedDB I/O. The hooks here are the closest the codebase comes today to the target "hooks own I/O" rule — but only `useGameStorage` actually owns I/O. Server I/O still lives in `services/` and is called directly from page-level `useEffect`s (the gap Track 3 closes).

## Data flow

- **In:** Called by `GameBoard` (in `pages/GamePage.tsx`) and `DailyGameBoard`. `useGameStorage` is also called directly by `DailyFlow.tsx`.
- **Out:**
  - `useGame` returns the gameplay state slice (cells, conflicts, draggedCells, isSolved, history, undo, redo, pointer handlers, reset).
  - `useTimer` returns elapsed seconds + control methods (start, pause, stop, reset, restore).
  - `useGameStorage` returns four IndexedDB CRUD methods (saveState, loadState, clearState, addCompletion).

## Files

- **`useGame.ts`** — Gameplay reducer. Owns the `HistoryState` machine (`cells`, `past`, `future`) via `useReducer` with `commit` / `undo` / `redo` actions. Snapshot stack with `HISTORY_LIMIT = 200`. Drag-intent state is in refs (`startCellRef`, `hasDraggedRef`, `dragIntentRef`, `draggedCellsRef`). Conflicts are `useMemo`'d via `getAllConflicts` (engine).
- **`useTimer.ts`** — Pause/resume timer. State: `elapsedAtLastPause`, `lastResumedAt`, `tick`, `stopped`. Display elapsed is computed from `Date.now() − lastResumedAt + elapsedAtLastPause` each render. `restore(state)` rehydrates from persistence; `stop()` is one-way (terminal). `setInterval(1s)` only ticks the display; the actual elapsed time is wall-clock-accurate.
- **`useGameStorage.ts`** — IndexedDB CRUD. Four methods wrapped in `useCallback` and returned through a `useMemo`'d object so callers can put it in dependency arrays. `saveState`, `loadState`, `clearState` all key on `idFor(flowType, flowId)`. `addCompletion` appends to a separate object store.

## State management

- `useGame`: `useReducer` for history; `useRef` for transient drag state; `useState` for the mirrored `draggedCells` set (re-render on highlight change).
- `useTimer`: `useState` × 4; `useRef<setInterval>`.
- `useGameStorage`: no state; `useCallback` + `useMemo` only.

## Rules specific to this directory

- **`useTimer.stop()` is terminal.** Once `stopped=true`, `start()` is a no-op until `reset()` clears the flag. This is intentional — solving the puzzle ends the timer permanently.
- **`useGame`'s reducer state mirrors to refs.** `cellsRef` mirrors `cells` so pointer handlers (which must be reference-stable for the grid's React-DOM listeners) can read state without depending on the latest cells. Same pattern for `draggedCellsRef`.
- **`useGameStorage` returns a `useMemo`'d object.** Don't destructure inline at every call site — the memoized identity is what makes the hook safe to put in effect dep lists.
- **Persisted shapes live in `storage/`.** Lesson 4: `GameHistory`, `CompletionRecord`, `GameState` are all defined in `storage/types.ts` and consumed from here. Don't redeclare.

