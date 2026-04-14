import type { CellState, PuzzleData } from '../engine/types';

/** Persisted game state stored in IndexedDB. */
export interface GameState {
  id: 'current';
  puzzle: PuzzleData;
  cells: CellState[][];
  timer: {
    elapsedAtLastPause: number; // accumulated seconds
    lastResumedAt: number | null; // timestamp ms, null when paused
  };
  status: 'in-progress' | 'solved';
  startedAt: number; // timestamp ms
}

/** Record of a completed puzzle for history/stats. */
export interface CompletionRecord {
  puzzleId: string;
  time: number; // seconds
  completedAt: number; // timestamp ms
}
