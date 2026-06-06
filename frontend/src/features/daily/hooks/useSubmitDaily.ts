import { useMutation } from '@tanstack/react-query';
import { submitDailyResult } from '@features/daily/services/dailyService';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import { dateFromAssignedAt } from '@shared/dates';
import { ApiError } from '@shared/api';
import type {
  DailyPuzzlePayload,
  DailySubmitResponse,
} from '@shared/types/daily';
import type { GameState } from '@storage/types';
import type { CellState } from '@engine/types';

/** Args passed to `mutation.mutate(...)` when the user solves. */
export interface SubmitDailyArgs {
  payload: DailyPuzzlePayload;
  solution: number[][];
  elapsedMs: number;
}

/** Resolved payload of a successful submit (200 OR 409 short-circuit). */
export interface SubmitDailySuccess {
  result: DailySubmitResponse;
  submittedAt: string;
  payload: DailyPuzzlePayload;
  /** True when synthesized from a 409 "already solved" response. */
  synthetic409?: boolean;
}

/** Empty cells grid sized to the payload's grid dimension — minimal
 *  placeholder so the persisted GameState row carries a coherent
 *  shape for the daily flow. */
function makeEmptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from({ length: size }, () => 'empty' as const),
  );
}

/**
 * `useMutation` wrapping the daily-flow submit cascade. Replaces
 * #176's hand-rolled `setState({kind:'submitting'})` → POST →
 * `setState({kind:'solved'|'submit-error'})` plumbing in
 * `DailyFlow.tsx`'s `handleSolved`.
 *
 * mutationFn:
 *   1. POST `submitDailyResult` with the player's solution + elapsed
 *      time.
 *   2. Persist a solved `GameState` row to IDB at the per-flow slot
 *      `(daily, YYYY-MM-DD)` so a subsequent visit short-circuits
 *      via `useDailyPuzzle`'s IDB read.
 *   3. Resolve `{ result, submittedAt, payload }` for the render
 *      branch to feed PostCompletionScreen.
 *
 * 409 special case: server says "already solved" (e.g. user solved
 * elsewhere and is now submitting from a second tab). Treat as a
 * synthetic success with `serverElapsedMs: 0` — the canonical solved
 * row will be readable from the GET endpoint on the next visit, so
 * we don't try to persist anything here. Matches pre-#176 behaviour.
 *
 * Persistence failure tolerance: `saveState` write errors are wrapped
 * in try/catch with a `console.warn`. The submit succeeded on the
 * server; failing to persist locally just means the next visit will
 * re-GET (and short-circuit on the server's solved response). Same
 * design as `useCurationPuzzle` (#176 curation slice).
 *
 * Retry affordance: callers wire the submit-error UI's "Try again"
 * button to `mutation.reset()`, returning the surface to its
 * idle/playing state so the user can resubmit (the GameBoard's
 * `onSolved` re-fires the mutation on the next tap-out-then-back-in,
 * matching pre-#176 semantics where retry returned to `playing`).
 *
 * Errors:
 *   - `ApiError` (excluding 409) bubbles to `mutation.error`; the
 *     caller maps `err.status` to user-facing copy via
 *     `submitErrorCopy()`.
 *   - Generic `Error` bubbles too; the caller falls back to a
 *     status-less copy.
 */
export function useSubmitDaily() {
  const { saveState } = useGameStorage();

  return useMutation<SubmitDailySuccess, ApiError | Error, SubmitDailyArgs>({
    mutationFn: async ({ payload, solution, elapsedMs }) => {
      let result: DailySubmitResponse;
      try {
        result = await submitDailyResult({
          assignedAt: payload.assignedAt,
          outcome: 'solved',
          playTimeMs: elapsedMs,
          solution,
        });
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          // 409 = server says already solved. Treat as terminal
          // success with sensible defaults; canonical solved row is
          // readable from the GET endpoint on the next visit. Don't
          // persist locally — we'd write a row with serverElapsedMs=0
          // which would short-circuit subsequent useDailyPuzzle reads
          // to render a 0:00 PostCompletionScreen (worse than going
          // through the GET again).
          return {
            result: { serverElapsedMs: 0 },
            submittedAt: new Date().toISOString(),
            payload,
            synthetic409: true,
          };
        }
        throw err;
      }

      const submittedAt = new Date().toISOString();
      const flowId = dateFromAssignedAt(payload.assignedAt);
      const persistedState: GameState = {
        id: `daily:${flowId}`,
        flowType: 'daily',
        flowId,
        puzzle: {
          puzzleId: payload.puzzleId,
          gridSize: payload.grid,
          mode: 'standard',
          regionMap: payload.regions,
        },
        cells: makeEmptyCells(payload.grid),
        timer: {
          elapsedAtLastPause: result.serverElapsedMs,
          lastResumedAt: null,
        },
        status: 'solved',
        startedAt: Date.now() - result.serverElapsedMs,
        serverElapsedMs: result.serverElapsedMs,
        submittedAt,
        leaderboardRank: result.leaderboardRank,
      };
      try {
        await saveState(persistedState);
      } catch (err) {
        console.warn(
          '[useSubmitDaily] saveState failed; submit succeeded server-side, continuing without local persistence',
          err,
        );
      }

      return { result, submittedAt, payload };
    },
  });
}
