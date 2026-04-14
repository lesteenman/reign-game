import type { CellState, Conflict, Position } from './types';

/** Collect positions of all marked cells */
function getMarkerPositions(cells: CellState[][]): Position[] {
  const positions: Position[] = [];
  for (let row = 0; row < cells.length; row++) {
    const rowCells = cells[row]!;
    for (let col = 0; col < rowCells.length; col++) {
      if (rowCells[col] === 'marked') {
        positions.push({ row, col });
      }
    }
  }
  return positions;
}

/** Create a canonical key for a pair of positions (for deduplication) */
function conflictKey(a: Position, b: Position): string {
  // Ensure consistent ordering: smaller row first, then smaller col
  if (a.row < b.row || (a.row === b.row && a.col < b.col)) {
    return `${a.row},${a.col}-${b.row},${b.col}`;
  }
  return `${b.row},${b.col}-${a.row},${a.col}`;
}

/** Create a Conflict with positions in canonical order */
function makeConflict(a: Position, b: Position): Conflict {
  if (a.row < b.row || (a.row === b.row && a.col < b.col)) {
    return { cells: [a, b] };
  }
  return { cells: [b, a] };
}

/**
 * Find all rows that have more than 1 marker.
 * Returns a Conflict for each pair of markers in the same row.
 */
export function checkRowConstraint(
  cells: CellState[][],
  _gridSize: number,
): Conflict[] {
  const conflicts: Conflict[] = [];
  for (let row = 0; row < cells.length; row++) {
    const rowCells = cells[row]!;
    const markerCols: number[] = [];
    for (let col = 0; col < rowCells.length; col++) {
      if (rowCells[col] === 'marked') {
        markerCols.push(col);
      }
    }
    // Generate all pairs
    for (let i = 0; i < markerCols.length; i++) {
      for (let j = i + 1; j < markerCols.length; j++) {
        conflicts.push({
          cells: [
            { row, col: markerCols[i]! },
            { row, col: markerCols[j]! },
          ],
        });
      }
    }
  }
  return conflicts;
}

/**
 * Find all columns that have more than 1 marker.
 * Returns a Conflict for each pair of markers in the same column.
 */
export function checkColumnConstraint(
  cells: CellState[][],
  gridSize: number,
): Conflict[] {
  const conflicts: Conflict[] = [];
  for (let col = 0; col < gridSize; col++) {
    const markerRows: number[] = [];
    for (let row = 0; row < cells.length; row++) {
      if (cells[row]![col] === 'marked') {
        markerRows.push(row);
      }
    }
    for (let i = 0; i < markerRows.length; i++) {
      for (let j = i + 1; j < markerRows.length; j++) {
        conflicts.push({
          cells: [
            { row: markerRows[i]!, col },
            { row: markerRows[j]!, col },
          ],
        });
      }
    }
  }
  return conflicts;
}

/**
 * Find all regions that have more than 1 marker.
 * Returns a Conflict for each pair of markers in the same region.
 */
export function checkRegionConstraint(
  cells: CellState[][],
  regionMap: number[][],
): Conflict[] {
  const markers = getMarkerPositions(cells);

  // Group markers by region
  const byRegion = new Map<number, Position[]>();
  for (const pos of markers) {
    const region = regionMap[pos.row]![pos.col]!;
    const group = byRegion.get(region);
    if (group) {
      group.push(pos);
    } else {
      byRegion.set(region, [pos]);
    }
  }

  const conflicts: Conflict[] = [];
  for (const group of byRegion.values()) {
    for (let i = 0; i < group.length; i++) {
      for (let j = i + 1; j < group.length; j++) {
        conflicts.push(makeConflict(group[i]!, group[j]!));
      }
    }
  }
  return conflicts;
}

/**
 * For every pair of markers that are horizontally, vertically, or diagonally
 * adjacent (8-directional), return a Conflict. No double-counting.
 */
export function checkAdjacencyConstraint(
  cells: CellState[][],
  _gridSize: number,
): Conflict[] {
  const markers = getMarkerPositions(cells);
  const conflicts: Conflict[] = [];

  for (let i = 0; i < markers.length; i++) {
    for (let j = i + 1; j < markers.length; j++) {
      const a = markers[i]!;
      const b = markers[j]!;
      const rowDiff = Math.abs(a.row - b.row);
      const colDiff = Math.abs(a.col - b.col);
      if (rowDiff <= 1 && colDiff <= 1) {
        conflicts.push(makeConflict(a, b));
      }
    }
  }
  return conflicts;
}

/**
 * Combines all four constraint checks and deduplicates conflicts
 * (same pair of cells from different constraint types).
 */
export function getAllConflicts(
  cells: CellState[][],
  regionMap: number[][],
  gridSize: number,
): Conflict[] {
  const all = [
    ...checkRowConstraint(cells, gridSize),
    ...checkColumnConstraint(cells, gridSize),
    ...checkRegionConstraint(cells, regionMap),
    ...checkAdjacencyConstraint(cells, gridSize),
  ];

  // Deduplicate by canonical key
  const seen = new Set<string>();
  const unique: Conflict[] = [];
  for (const conflict of all) {
    const key = conflictKey(conflict.cells[0], conflict.cells[1]);
    if (!seen.has(key)) {
      seen.add(key);
      unique.push(conflict);
    }
  }
  return unique;
}
