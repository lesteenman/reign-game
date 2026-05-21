import { useQuery } from '@tanstack/react-query';
import {
  fetchNextPuzzle,
  NoPuzzlesAvailableError,
} from '@features/curation/services/fetch-next-puzzle-service';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import { createFreshGameState } from '@storage/utils';
import {
  EMPTY_HISTORY,
  buildCurationFlowId,
  type FlowType,
  type GameHistory,
} from '@storage/types';
import type { CellState, Mode, PuzzleData } from '@engine/types';

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
 * `refetchOnWindowFocus: false` — re-running the queryFn on focus
 * would call `saveState` with a brand-new `startedAt` if the saved
 * slot was somehow missed, resetting the visible timer. The game's
 * own visibility-handling logic already manages timer resume/pause
 * via `GameBoard` + `useTimer`.
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
      // flowType is guaranteed non-null here because `enabled` gates the
      // query off when it's null. The `!` is the BR-acceptable shape
      // for "TanStack-gated invariant proven at the call boundary".
      const ft = flowType!;
      const saved = await loadState(ft, flowId);
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
      // have removed it). Fetch fresh, persist, surface.
      const puzzle = await fetchNextPuzzle(size, mode);
      const gameState = createFreshGameState(ft, flowId, puzzle);
      await saveState(gameState);
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
