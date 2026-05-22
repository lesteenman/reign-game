import type { Mode } from '@engine/types';
import { apiPut } from '@shared/api';

/**
 * Update a puzzle's status after the player completes or skips it.
 * Lives under `shared/game/services/` because the endpoint is
 * cross-feature — `useUpdatePuzzleStatus` (the only caller) is
 * consumed by both the curation flow (GameBoard, VerdictSurface)
 * and daily completion.
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
