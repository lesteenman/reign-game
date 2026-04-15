import type { PuzzleData } from '../engine/types';
import { apiFetch } from './api';

/** Options for the puzzle generator endpoint. */
export interface GenerateOptions {
  size: number;
  mode: 'standard' | 'double';
  pipeline?: 'region-first' | 'iterative' | 'constraint-aware';
  solver?: 'backtrack' | 'propagation';
  regions?: 'bfs' | 'wfc';
  regionVariance?: number;
}

/** Request a new puzzle from the backend generate endpoint. */
export async function generatePuzzle(
  options: GenerateOptions,
): Promise<PuzzleData> {
  const params: Record<string, string> = {
    size: String(options.size),
    mode: options.mode,
  };

  if (options.pipeline !== undefined) {
    params.pipeline = options.pipeline;
  }
  if (options.solver !== undefined) {
    params.solver = options.solver;
  }
  if (options.regions !== undefined) {
    params.regions = options.regions;
  }
  if (options.regionVariance !== undefined) {
    params.regionVariance = String(options.regionVariance);
  }

  return apiFetch<PuzzleData>('/puzzles/generate', params);
}
