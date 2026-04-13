/** Cell state in the game grid */
export type CellState = 'empty' | 'excluded' | 'marked';

/** Zero-indexed position on the grid */
export interface Position {
  row: number;
  col: number;
}

/** A pair of cells that violate a constraint */
export interface Conflict {
  cells: [Position, Position];
}

/** Puzzle data from the API */
export interface PuzzleData {
  puzzleId: string;
  gridSize: number;
  mode: string;
  regionMap: number[][];
}
