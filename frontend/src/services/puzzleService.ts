import type { PuzzleData } from '../engine/types';
import { apiFetch } from './api';

/** Request a new puzzle from the backend generate endpoint. */
export async function generatePuzzle(
  size: number,
  mode: string,
): Promise<PuzzleData> {
  return apiFetch<PuzzleData>('/puzzles/generate', {
    size: String(size),
    mode,
  });
}
