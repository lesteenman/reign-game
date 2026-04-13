import { describe, it, expect } from 'vitest';
import { validateSolution } from './validator';
import type { CellState } from './types';

// -- Known valid 5x5 puzzle --

const regionMap: number[][] = [
  [0, 0, 1, 1, 1],
  [0, 0, 1, 2, 2],
  [3, 3, 1, 2, 2],
  [3, 4, 4, 4, 2],
  [3, 3, 4, 4, 4],
];

const validSolution: CellState[][] = [
  ['empty', 'empty', 'empty', 'empty', 'marked'],
  ['empty', 'marked', 'empty', 'empty', 'empty'],
  ['empty', 'empty', 'empty', 'marked', 'empty'],
  ['marked', 'empty', 'empty', 'empty', 'empty'],
  ['empty', 'empty', 'marked', 'empty', 'empty'],
];

/** Create an empty NxN grid */
function emptyGrid(n: number): CellState[][] {
  return Array.from({ length: n }, () =>
    Array.from({ length: n }, () => 'empty' as CellState),
  );
}

describe('validateSolution', () => {
  const cases = [
    {
      name: 'valid complete 5x5 solution → true',
      grid: validSolution,
      expected: true,
    },
    {
      name: 'empty board → false',
      grid: emptyGrid(5),
      expected: false,
    },
    {
      name: 'incomplete board (fewer than 5 markers) → false',
      grid: [
        ['empty', 'empty', 'empty', 'marked', 'empty'],
        ['marked', 'empty', 'empty', 'empty', 'empty'],
        ['empty', 'empty', 'marked', 'empty', 'empty'],
        ['empty', 'empty', 'empty', 'empty', 'empty'],
        ['empty', 'marked', 'empty', 'empty', 'empty'],
      ] as CellState[][],
      expected: false,
    },
    {
      name: 'board with row conflict → false',
      grid: [
        ['marked', 'empty', 'empty', 'marked', 'empty'],
        ['empty', 'empty', 'empty', 'empty', 'empty'],
        ['empty', 'empty', 'marked', 'empty', 'empty'],
        ['empty', 'empty', 'empty', 'empty', 'marked'],
        ['empty', 'marked', 'empty', 'empty', 'empty'],
      ] as CellState[][],
      expected: false,
    },
    {
      name: 'board with adjacency conflict → false',
      // Move row 2 marker from col 2 to col 1 → adjacent to (1,0)
      grid: [
        ['empty', 'empty', 'empty', 'marked', 'empty'],
        ['marked', 'empty', 'empty', 'empty', 'empty'],
        ['empty', 'marked', 'empty', 'empty', 'empty'],
        ['empty', 'empty', 'empty', 'empty', 'marked'],
        ['empty', 'empty', 'marked', 'empty', 'empty'],
      ] as CellState[][],
      expected: false,
    },
    {
      name: 'too many markers → false',
      grid: [
        ['marked', 'empty', 'empty', 'marked', 'empty'],
        ['marked', 'empty', 'empty', 'empty', 'empty'],
        ['empty', 'empty', 'marked', 'empty', 'empty'],
        ['empty', 'empty', 'empty', 'empty', 'marked'],
        ['empty', 'marked', 'empty', 'empty', 'empty'],
      ] as CellState[][],
      expected: false,
    },
  ] as const;

  for (const { name, grid, expected } of cases) {
    it(name, () => {
      expect(validateSolution(grid, regionMap, 5)).toBe(expected);
    });
  }
});
