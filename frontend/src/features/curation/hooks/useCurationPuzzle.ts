import { useQuery } from '@tanstack/react-query';
import {
  fetchNextPuzzle,
  NoPuzzlesAvailableError,
} from '@features/curation/services/fetch-next-puzzle-service';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import { createFreshGameState } from '@reign/core/storage';
import {
  EMPTY_HISTORY,
  buildCurationFlowId,
  type FlowType,
  type GameHistory,
} from '@reign/core/storage';
import type { CellState, Mode, PuzzleData } from '@reign/core/engine';

/**
 * Shape returned by `useCurationPuzzle` once loaded. Either the resumed
 * saved game's state (loadState hit) OR a freshly-fetched-and-persisted
 * puzzle (loadState miss / solved leftover). Caller (`PlayPuzzlePage`)
 * doesn't need to discriminate — it just forwards these to `<GameBoard>`.
 */
export interface CurationPuzzleData {
  puzzle: PuzzleData;
  flowType: FlowType;
  flowId: string;
  initialCells: CellState[][];
  initialHistory: GameHistory;
  timerElapsed: number;
  timerResumedAt: number | null;
  startedAt: number;
}

interface UseCurationPuzzleArgs {
  flowType: FlowType | null;
  size: number;
  mode: Mode;
}

/**
 * Composite useQuery wrapping the curation/practice play page's load
 * cascade — IndexedDB resume-or-fetch-and-save. Replaces #176's manual
 * `LoadState` discriminated union + `useEffect` + `fetchKey` retry
 * counter in `PlayPuzzlePage.tsx`.
 *
 * Cascade:
 *   1. `loadState(flowType, flowId)` — if a non-solved slot exists,
 *      surface it (resume path).
 *   2. Otherwise (miss or defensive solved-leftover) fetch a fresh
 *      puzzle via `fetchNextPuzzle(size, mode)` and persist a new
 *      `GameState` via `saveState`. Surface the persisted state.
 *
 * Errors:
 *   - `NoPuzzlesAvailableError` (404 from `/api/puzzles/next`) — the
 *     pool is empty for this (size, mode). `PlayPuzzlePage` renders a
 *     dedicated no-puzzles state with a retry button.
 *   - Any other error — generic error UI with a try-again button.
 *
 *   The hook lets BOTH bubble out via `query.error`; the page does
 *   `error instanceof NoPuzzlesAvailableError` to discriminate. We do
 *   NOT swallow either into the success channel — that would force
 *   the page to invent its own discriminated union again (defeating
 *   the migration).
 *
 * Retry: `query.refetch()` re-runs the cascade against the same key.
 * No `fetchKey` state is needed; consumers wire the retry button to
 * `refetch`.
 *
 * `retry: false` — the no-puzzles + generic-error UIs are user-visible
 * states that need explicit human action, not transient failures
 * worth auto-retrying.
 *
 * `refetchOnWindowFocus: false` — re-running the queryFn on focus is
 * idempotent on the saved-hit branch (returns the persisted state
 * untouched), but the saved-miss branch isn't: it calls
 * `createFreshGameState` which sets `startedAt: Date.now()` and
 * triggers a fresh `saveState`, resetting the visible timer. The
 * game's own visibility-handling logic already manages timer
 * resume/pause via `GameBoard` + `useTimer`, so an automatic
 * window-focus refetch is at best redundant and at worst regressing.
 */
export function useCurationPuzzle({ flowType, size, mode }: UseCurationPuzzleArgs) {
  const { loadState, saveState } = useGameStorage();
  const flowId = flowType !== null ? buildCurationFlowId(size, mode) : '';

  return useQuery<CurationPuzzleData>({
    queryKey: ['curationPuzzle', flowType, flowId],
    enabled: flowType !== null,
    retry: false,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      // `enabled: false` blocks the auto-trigger but NOT manual
      // `refetch()` calls. The actual guard is `PlayPuzzlePage`'s
      // `if (flowType === null) return <RedirectToHome />` early
      // return — the page never renders the Retry button when
      // `flowType` is null, so the queryFn can't be reached from the
      // UI. Defensive throw here in case some future caller wires
      // `refetch()` to a non-page surface (test, devtool, etc.).
      if (flowType === null) {
        throw new Error(
          'useCurationPuzzle: queryFn invoked with flowType === null. ' +
            'Caller must gate refetch on a non-null flowType.',
        );
      }
      const saved = await loadState(flowType, flowId);
      if (saved && saved.status !== 'solved') {
        return {
          puzzle: saved.puzzle,
          flowType: saved.flowType,
          flowId: saved.flowId,
          initialCells: saved.cells,
          initialHistory: saved.history ?? EMPTY_HISTORY,
          timerElapsed: saved.timer.elapsedAtLastPause,
          timerResumedAt: saved.timer.lastResumedAt,
          startedAt: saved.startedAt,
        };
      }

      // Miss or solved-leftover (defensive — clear-on-solve should
      // have removed it). Fetch fresh, then try to persist before
      // surfacing.
      const puzzle = await fetchNextPuzzle(size, mode);
      const gameState = createFreshGameState(flowType, flowId, puzzle);

      // Tolerate `saveState` failure: the puzzle was fetched
      // successfully and is playable in-memory, so we surface it
      // even if persistence fails (IDB quota, browser private-mode,
      // etc.). The degraded path is "no resume on refresh" — better
      // than blocking play and burning the fetched puzzle by
      // surfacing a generic error UI that would re-fetch on Retry.
      // Logged so devs notice via the console; not user-visible.
      try {
        await saveState(gameState);
      } catch (err) {
        console.warn(
          '[useCurationPuzzle] saveState failed; continuing without persistence',
          err,
        );
      }

      return {
        puzzle: gameState.puzzle,
        flowType: gameState.flowType,
        flowId: gameState.flowId,
        initialCells: gameState.cells,
        initialHistory: gameState.history ?? EMPTY_HISTORY,
        timerElapsed: 0,
        timerResumedAt: null,
        startedAt: gameState.startedAt,
      };
    },
  });
}

// Re-export so consumers can `instanceof`-discriminate errors without
// importing the service layer directly (architecture rule).
export { NoPuzzlesAvailableError };
