import type { PuzzleData, CellState } from '../engine/types';
import type { GameState } from './types';

/** Create a fresh GameState for a newly fetched puzzle. */
export function createFreshGameState(puzzle: PuzzleData): GameState {
  return {
    id: 'current',
    puzzle,
    cells: Array.from({ length: puzzle.gridSize }, () =>
      Array.from<CellState>({ length: puzzle.gridSize }).fill('empty'),
    ),
    timer: { elapsedAtLastPause: 0, lastResumedAt: null },
    status: 'in-progress',
    startedAt: Date.now(),
  };
}
