import type { CellState, PuzzleData } from '../engine/types';

/**
 * Undo/redo history persisted with the rest of the game state so Ctrl+Z
 * survives a reload. Optional for backwards compatibility with saved states
 * written before Phase 4.6 — loaders should default to EMPTY_HISTORY.
 */
export interface GameHistory {
  past: CellState[][][];
  future: CellState[][][];
}

/** Shared empty-history literal so hook initializers and loaders agree. */
export const EMPTY_HISTORY: GameHistory = { past: [], future: [] };

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
  history?: GameHistory;
}

/** Record of a completed puzzle for history/stats. */
export interface CompletionRecord {
  puzzleId: string;
  time: number; // seconds
  completedAt: number; // timestamp ms
}
