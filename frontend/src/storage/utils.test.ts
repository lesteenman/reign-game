import { describe, it, expect } from 'vitest';
import { createFreshGameState } from './utils';
import type { PuzzleData } from '../engine/types';

const BASE_PUZZLE: PuzzleData = {
  puzzleId: 'test-001',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

const PUZZLE_WITH_METADATA: PuzzleData = {
  ...BASE_PUZZLE,
  puzzleId: 'test-002',
  metadata: {
    pipeline: 'iterative',
    solver: 'propagation',
    regions: 'bfs',
    regionVariance: 0.0,
    generationDurationMs: 4200,
    createdAt: '2026-04-15T10:30:00Z',
  },
};

describe('createFreshGameState', () => {
  it('creates game state with metadata when puzzle has metadata', () => {
    // Arrange
    const puzzle = PUZZLE_WITH_METADATA;

    // Act
    const state = createFreshGameState(puzzle);

    // Assert
    expect(state.puzzle.metadata).toEqual({
      pipeline: 'iterative',
      solver: 'propagation',
      regions: 'bfs',
      regionVariance: 0.0,
      generationDurationMs: 4200,
      createdAt: '2026-04-15T10:30:00Z',
    });
  });

  it('creates game state without metadata when puzzle has no metadata', () => {
    // Arrange
    const puzzle = BASE_PUZZLE;

    // Act
    const state = createFreshGameState(puzzle);

    // Assert
    expect(state.puzzle.metadata).toBeUndefined();
  });

  it('preserves all other game state fields when metadata is present', () => {
    // Arrange
    const puzzle = PUZZLE_WITH_METADATA;

    // Act
    const state = createFreshGameState(puzzle);

    // Assert
    expect(state.id).toBe('current');
    expect(state.puzzle.puzzleId).toBe('test-002');
    expect(state.puzzle.gridSize).toBe(5);
    expect(state.puzzle.mode).toBe('standard');
    expect(state.status).toBe('in-progress');
    expect(state.cells).toHaveLength(5);
    expect(state.cells[0]).toHaveLength(5);
    expect(state.cells[0]![0]).toBe('empty');
  });

  it('loads old saved game without metadata field cleanly', () => {
    // Arrange — simulate an old saved game without metadata
    const oldPuzzle: PuzzleData = {
      puzzleId: 'old-001',
      gridSize: 5,
      mode: 'standard',
      regionMap: BASE_PUZZLE.regionMap,
    };

    // Act
    const state = createFreshGameState(oldPuzzle);

    // Assert
    expect(state.puzzle.metadata).toBeUndefined();
    expect(state.puzzle.puzzleId).toBe('old-001');
    expect(state.status).toBe('in-progress');
  });
});
