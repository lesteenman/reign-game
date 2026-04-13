import type { CellState } from './types';
import { getAllConflicts } from './constraints';

/**
 * Validates whether the current board state is a complete, valid solution.
 *
 * Returns true ONLY when:
 * 1. The grid has exactly `gridSize` markers total
 * 2. `getAllConflicts()` returns zero conflicts
 */
export function validateSolution(
  cells: CellState[][],
  regionMap: number[][],
  gridSize: number,
): boolean {
  // Count total markers
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

  const conflicts = getAllConflicts(cells, regionMap, gridSize);
  return conflicts.length === 0;
}
