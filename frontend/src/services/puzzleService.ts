import type { Mode } from '@engine/types';
import { apiPut } from '@services/api';

/**
 * Update a puzzle's status after the player completes or skips it.
 *
 * Used by `shared/game/hooks/useUpdatePuzzleStatus`, which is itself
 * consumed by `GameBoard` (curation flow) and `VerdictSurface`
 * (curation skip flow). Stays in `src/services/` because the call is
 * genuinely cross-feature — daily completion eventually writes the
 * same status field via the daily-result POST, and any future
 * pack/leaderboard flow would too.
 *
 * `fetchNextPuzzle` moved to `features/curation/services/
 * fetch-next-puzzle-service.ts` in #176 (single-feature consumer).
 */
export async function updatePuzzleStatus(
  puzzleId: string,
  size: number,
  mode: Mode,
  status: 'solved' | 'skipped',
): Promise<void> {
  await apiPut<Record<string, never>>(
    `/api/puzzles/${puzzleId}/status`,
    { status },
    { params: { size: String(size), mode } },
  );
}
