import { useQuery } from '@tanstack/react-query';
import { getDaily } from '@features/daily/services/dailyService';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import { todayUtcDate } from '@shared/dates';
import { ApiError } from '@shared/api';
import type {
  DailyPuzzlePayload,
  DailySubmitResponse,
} from '@shared/types/daily';

/**
 * Discriminated outcome surfaced by `useDailyPuzzle`. Either the
 * server (or IDB short-circuit) tells us the daily is already solved
 * — `kind: 'solved'` carries the data PostCompletionScreen needs — or
 * the daily is fetched and ready to play (`kind: 'playing'` carries
 * the GET payload for DailyGameBoard).
 *
 * The legacy 6-arm `FlowState` union split this across page-level
 * state; here the read concern owns just two branches, while the
 * write concern (submit + 409 + persistence) lives in `useSubmitDaily`.
 */
export type CurrentDailyState =
  | {
      kind: 'solved';
      payload: DailyPuzzlePayload;
      result: DailySubmitResponse;
      submittedAt: string;
    }
  | { kind: 'playing'; payload: DailyPuzzlePayload };

/**
 * Composite useQuery wrapping the daily-flow load cascade. Replaces
 * #176's hand-rolled `useState<FlowState>` + 76-line `useEffect` in
 * `DailyFlow.tsx`.
 *
 * Cascade:
 *   1. IDB short-circuit. Read the per-flow slot `(daily, todayUTC)`.
 *      If status='solved' with `serverElapsedMs > 0`, synthesize a
 *      `kind: 'solved'` payload from the persisted row and return —
 *      no network round-trip. (Older partial-write rows missing
 *      `serverElapsedMs` fall through to the GET so the server's
 *      canonical state drives the render.)
 *   2. `getDaily()` GET. If `outcome === 'solved'` with canonical
 *      `serverElapsedMs` + `submittedAt`, return `kind: 'solved'`
 *      from the GET payload (no POST needed; leaderboardRank stays
 *      `undefined` because the GET endpoint doesn't surface it).
 *   3. Otherwise return `kind: 'playing'` with the GET payload —
 *      `DailyGameBoard` mounts and the submit-mutation takes over
 *      after the user solves.
 *
 * Errors:
 *   - `ApiError` (404 / 500 / etc.) bubbles to `query.error`; the
 *     caller maps `err.status` to user-facing copy via
 *     `loadErrorCopy()` in `DailyFlow.tsx`.
 *   - Generic `Error` / network failures also bubble through; the
 *     caller falls back to a status-less copy.
 *
 * `retry: false` — the error state is a user-visible UI with an
 * explicit Try Again button; auto-retry would mask it.
 *
 * `refetchOnWindowFocus: false` — re-running the queryFn on focus
 * would re-issue the GET. The daily endpoint returns the same payload
 * for the same UTC day so the data wouldn't change, but the extra
 * round-trip is wasted. If the user is mid-play, refetch would also
 * race the submit-mutation's persistence on the same `(daily, date)`
 * IDB slot.
 *
 * Defensive `kind: 'solved'` from the GET requires both
 * `serverElapsedMs` AND `submittedAt`. A `console.warn` fires when
 * the server returns `outcome === 'solved'` without those canonical
 * fields and we fall through to `playing` — the gap is observable
 * rather than silently broken.
 */
export function useDailyPuzzle() {
  const { loadState } = useGameStorage();

  return useQuery<CurrentDailyState, ApiError | Error>({
    queryKey: ['daily', todayUtcDate()],
    retry: false,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      // Step 1: IDB short-circuit.
      try {
        const persisted = await loadState('daily', todayUtcDate());
        if (
          persisted &&
          persisted.status === 'solved' &&
          typeof persisted.serverElapsedMs === 'number' &&
          persisted.serverElapsedMs > 0
        ) {
          const payload: DailyPuzzlePayload = {
            puzzleId: persisted.puzzle.puzzleId,
            grid: persisted.puzzle.gridSize,
            regions: persisted.puzzle.regionMap,
            assignedAt: `${persisted.flowId}T00:00:00Z`,
            outcome: 'solved',
          };
          return {
            kind: 'solved',
            payload,
            result: {
              serverElapsedMs: persisted.serverElapsedMs,
              leaderboardRank: persisted.leaderboardRank,
            },
            submittedAt:
              persisted.submittedAt ?? new Date().toISOString(),
          };
        }
      } catch {
        // Storage read failed — fall through to the network path.
        // Better to fetch (and retry on next visit) than to surface a
        // phantom error UI for a transient IDB hiccup.
      }

      // Step 2 + 3: GET, then discriminate solved-vs-playing.
      const payload = await getDaily();
      if (payload.outcome === 'solved') {
        if (
          typeof payload.serverElapsedMs === 'number' &&
          payload.submittedAt
        ) {
          return {
            kind: 'solved',
            payload,
            result: {
              serverElapsedMs: payload.serverElapsedMs,
              leaderboardRank: undefined,
            },
            submittedAt: payload.submittedAt,
          };
        }
        console.warn(
          'useDailyPuzzle: getDaily returned outcome=solved without serverElapsedMs/submittedAt; falling back to playing',
        );
      }
      return { kind: 'playing', payload };
    },
  });
}
