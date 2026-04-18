import type { PuzzleData } from '../engine/types';
import { apiFetch, ApiError, apiPut } from './api';

/** Error thrown when the puzzle pool has no puzzles for the requested size/mode. */
export class NoPuzzlesAvailableError extends Error {
  constructor(message = 'No puzzles available for this size and mode') {
    super(message);
    this.name = 'NoPuzzlesAvailableError';
  }
}

/** Fetch the next puzzle from the pool for a given size and mode. */
export async function fetchNextPuzzle(
  size: number,
  mode: string,
): Promise<PuzzleData> {
  try {
    return await apiFetch<PuzzleData>('/api/puzzles/next', {
      size: String(size),
      mode,
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      throw new NoPuzzlesAvailableError();
    }
    throw err;
  }
}

/** Update a puzzle's status after the player completes or skips it. */
export async function updatePuzzleStatus(
  puzzleId: string,
  size: number,
  mode: string,
  status: 'solved' | 'skipped',
): Promise<void> {
  await apiPut<Record<string, never>>(
    `/api/puzzles/${puzzleId}/status`,
    { status },
    { size: String(size), mode },
  );
}
