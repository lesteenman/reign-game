import type { CellState, PuzzleData } from '../engine/types';
import type { FlowType, GameState } from './types';
import { EMPTY_HISTORY } from './types';

/**
 * Composite Flow Slot key — the only place the `:` separator appears in
 * production code.
 */
export function idFor(flowType: FlowType, flowId: string): string {
  return `${flowType}:${flowId}`;
}

/**
 * Create a fresh GameState for a newly fetched puzzle in a given Flow Slot.
 * The composite `id` is derived once via `idFor`; callers never construct
 * the `:`-joined string by hand.
 */
export function createFreshGameState(
  flowType: FlowType,
  flowId: string,
  puzzle: PuzzleData,
): GameState {
  return {
    id: idFor(flowType, flowId),
    flowType,
    flowId,
    puzzle,
    cells: Array.from({ length: puzzle.gridSize }, () =>
      Array.from<CellState>({ length: puzzle.gridSize }).fill('empty'),
    ),
    timer: { elapsedAtLastPause: 0, lastResumedAt: null },
    status: 'in-progress',
    startedAt: Date.now(),
    history: EMPTY_HISTORY,
  };
}
