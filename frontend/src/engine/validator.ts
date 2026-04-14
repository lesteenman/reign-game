import type { CellState, Conflict } from './types';
import { getAllConflicts } from './constraints';

/**
 * Validates whether the current board state is a complete, valid solution.
 *
 * Returns true ONLY when:
 * 1. The grid has exactly `gridSize` markers total
 * 2. There are zero conflicts
 *
 * Accepts optional pre-computed conflicts to avoid redundant work.
 */
export function validateSolution(
  cells: CellState[][],
  regionMap: number[][],
  gridSize: number,
  precomputedConflicts?: Conflict[],
): boolean {
  let markerCount = 0;
  for (const row of cells) {
    for (const cell of row) {
      if (cell === 'marked') {
        markerCount++;
      }
    }
  }

  if (markerCount !== gridSize) {
    return false;
  }

  const conflicts = precomputedConflicts ?? getAllConflicts(cells, regionMap, gridSize);
  return conflicts.length === 0;
}
