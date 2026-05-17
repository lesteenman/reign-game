import type { CellState, Mode, PuzzleData } from '../engine/types';

/**
 * Top-level mode of play. Determines which API surface produces puzzles
 * and which Flow Slot tracks in-progress state. Phase 7 wires curation
 * only; daily and pack are pre-declared so future producers extend the
 * same union without storage-layer churn.
 */
export type FlowType = 'curation' | 'daily' | 'pack';

const FLOW_TYPES: ReadonlyArray<FlowType> = ['curation', 'daily', 'pack'];

/**
 * Validate a raw URL parameter against the FlowType union. Returns null
 * for missing or unrecognized values; callers redirect to home in that
 * case so a typo'd `?flow=curations` doesn't silently miss the IDB lookup.
 */
export function parseFlowType(raw: string | null): FlowType | null {
  if (raw === null) return null;
  return (FLOW_TYPES as ReadonlyArray<string>).includes(raw)
    ? (raw as FlowType)
    : null;
}

/**
 * Build a curation Flow Slot identifier from `(size, mode)`. Kept in one
 * helper so future flow producers (daily ISO date, pack slug) extend the
 * same module.
 */
export function buildCurationFlowId(size: number, mode: Mode): string {
  return `${size}x${size}-${mode}`;
}

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

/**
 * Persisted game state stored in IndexedDB. `id` is the composite Flow
 * Slot key (`"{flowType}:{flowId}"`); `flowType` and `flowId` are
 * duplicated onto the row body so DevTools inspection doesn't require
 * re-parsing the composite key.
 */
export interface GameState {
  id: string;
  flowType: FlowType;
  flowId: string;
  puzzle: PuzzleData;
  cells: CellState[][];
  timer: {
    elapsedAtLastPause: number;
    lastResumedAt: number | null;
  };
  status: 'in-progress' | 'solved';
  startedAt: number;
  history?: GameHistory;
  /**
   * Server-canonical solve time in milliseconds. Optional and only
   * populated for solved daily rows so the short-circuit can render
   * PostCompletionScreen without re-fetching `getDaily()`. Other flows
   * ignore this field.
   */
  serverElapsedMs?: number;
  /**
   * RFC3339 timestamp of the daily submit. Only populated for solved
   * daily rows. Drives the "Submitted at HH:MM" line in
   * PostCompletionScreen on the short-circuit render.
   */
  submittedAt?: string;
  /**
   * Player's leaderboard rank from the daily POST response, if the
   * server returned one (signed-in identities only). Optional and
   * only populated for solved daily rows.
   */
  leaderboardRank?: number;
}

/** Record of a completed puzzle for history/stats. */
export interface CompletionRecord {
  puzzleId: string;
  time: number;
  completedAt: number;
}
