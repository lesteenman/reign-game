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

/** Generation metadata from the puzzle pool. */
export interface PuzzleMetadata {
  difficulty: number;
  maxTier: number;
  tierCounts: number[];
  traceLen: number;
  generationDurationMs: number;
  createdAt: string;
  /**
   * Seed the generator ran with. Encoded as a string to survive int64
   * values past the 2^53 JS-safe-integer boundary. Absent for pre-R-06C
   * puzzles that don't have one on record.
   */
  seed?: string;
}

/**
 * Canonical mode identifiers. Exported as a typed tuple so callers get
 * compile-time errors on invalid literals instead of relying on `string`
 * equality. Backend mirrors these in `handler.ModeStandard` / `ModeDouble`.
 */
export const MODES = ['standard', 'double'] as const;

/** One of the two supported puzzle modes. */
export type Mode = (typeof MODES)[number];

/** Type guard for narrowing an unknown string to a Mode. */
export function isMode(value: unknown): value is Mode {
  return typeof value === 'string' && (MODES as readonly string[]).includes(value);
}

/** Puzzle data from the API */
export interface PuzzleData {
  puzzleId: string;
  gridSize: number;
  mode: Mode;
  regionMap: number[][];
  metadata?: PuzzleMetadata;
}
