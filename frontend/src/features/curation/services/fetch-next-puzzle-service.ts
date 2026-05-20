import type { Mode, PuzzleData } from '@engine/types';
import { apiFetch, ApiError } from '@services/api';

/** Error thrown when the puzzle pool has no puzzles for the requested size/mode. */
export class NoPuzzlesAvailableError extends Error {
  constructor(message = 'No puzzles available for this size and mode') {
    super(message);
    this.name = 'NoPuzzlesAvailableError';
  }
}

/**
 * Fetch the next puzzle from the pool for a given size and mode.
 *
 * Pool lookup endpoint. Distinct from the daily endpoint
 * (`features/daily/services/dailyService.ts`) — daily fetches an
 * assigned-for-today puzzle keyed by device id; curation pulls from
 * the verdict-ranked queue.
 *
 * Was `services/puzzleService.ts::fetchNextPuzzle` before #176; moved
 * here because the curation flow is the only consumer.
 */
export async function fetchNextPuzzle(
  size: number,
  mode: Mode,
): Promise<PuzzleData> {
  try {
    return await apiFetch<PuzzleData>('/api/puzzles/next', {
      params: { size: String(size), mode },
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      throw new NoPuzzlesAvailableError();
    }
    throw err;
  }
}
