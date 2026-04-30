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
    difficulty: 2,
    maxTier: 2,
    tierCounts: [0, 3, 1, 0, 0],
    traceLen: 4,
    generationDurationMs: 4200,
    createdAt: '2026-04-15T10:30:00Z',
  },
};

describe('createFreshGameState', () => {
  it('creates game state with metadata when puzzle has metadata', () => {
    // Arrange
    const puzzle = PUZZLE_WITH_METADATA;

    // Act
    const state = createFreshGameState('curation', '5x5-standard', puzzle);

    // Assert
    expect(state.puzzle.metadata).toEqual({
      difficulty: 2,
      maxTier: 2,
      tierCounts: [0, 3, 1, 0, 0],
      traceLen: 4,
      generationDurationMs: 4200,
      createdAt: '2026-04-15T10:30:00Z',
    });
  });

  it('creates game state without metadata when puzzle has no metadata', () => {
    // Arrange
    const puzzle = BASE_PUZZLE;

    // Act
    const state = createFreshGameState('curation', '5x5-standard', puzzle);

    // Assert
    expect(state.puzzle.metadata).toBeUndefined();
  });

  it('preserves all other game state fields when metadata is present', () => {
    // Arrange
    const puzzle = PUZZLE_WITH_METADATA;

    // Act
    const state = createFreshGameState('curation', '5x5-standard', puzzle);

    // Assert
    expect(state.id).toBe('curation:5x5-standard');
    expect(state.flowType).toBe('curation');
    expect(state.flowId).toBe('5x5-standard');
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
    const state = createFreshGameState('curation', '5x5-standard', oldPuzzle);

    // Assert
    expect(state.puzzle.metadata).toBeUndefined();
    expect(state.puzzle.puzzleId).toBe('old-001');
    expect(state.status).toBe('in-progress');
  });

  it('builds the composite id from a different flowType for future flows', () => {
    // Arrange
    const puzzle = BASE_PUZZLE;

    // Act
    const state = createFreshGameState('daily', '2026-04-29', puzzle);

    // Assert
    expect(state.id).toBe('daily:2026-04-29');
    expect(state.flowType).toBe('daily');
    expect(state.flowId).toBe('2026-04-29');
  });
});
