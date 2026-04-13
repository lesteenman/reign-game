import { describe, it, expect } from 'vitest';
import {
  checkRowConstraint,
  checkColumnConstraint,
  checkRegionConstraint,
  checkAdjacencyConstraint,
  getAllConflicts,
} from './constraints';
import type { CellState } from './types';

// -- Test helpers --

/** Create an empty NxN grid */
function emptyGrid(n: number): CellState[][] {
  return Array.from({ length: n }, () =>
    Array.from({ length: n }, () => 'empty' as CellState),
  );
}

/** Place markers on a grid at given positions */
function placeMarkers(
  grid: CellState[][],
  positions: [number, number][],
): CellState[][] {
  const copy = grid.map((row) => [...row]);
  for (const [r, c] of positions) {
    copy[r]![c] = 'marked';
  }
  return copy;
}

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

// -------------------------------------------------
// checkRowConstraint
// -------------------------------------------------
describe('checkRowConstraint', () => {
  const cases = [
    {
      name: 'no markers → no conflicts',
      grid: emptyGrid(5),
      expected: 0,
    },
    {
      name: '1 marker per row → no conflicts',
      grid: validSolution,
      expected: 0,
    },
    {
      name: '2 markers in same row → 1 conflict',
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [0, 3],
      ]),
      expected: 1,
    },
    {
      name: '3 markers in same row → 3 conflicts (all pairs)',
      grid: placeMarkers(emptyGrid(5), [
        [1, 0],
        [1, 2],
        [1, 4],
      ]),
      expected: 3,
    },
  ] as const;

  for (const { name, grid, expected } of cases) {
    it(name, () => {
      const conflicts = checkRowConstraint(grid, 5);
      expect(conflicts).toHaveLength(expected);
    });
  }

  it('returns correct cell positions for row conflict', () => {
    const grid = placeMarkers(emptyGrid(5), [
      [2, 1],
      [2, 4],
    ]);
    const conflicts = checkRowConstraint(grid, 5);
    expect(conflicts).toHaveLength(1);
    expect(conflicts[0]!.cells).toEqual([
      { row: 2, col: 1 },
      { row: 2, col: 4 },
    ]);
  });
});

// -------------------------------------------------
// checkColumnConstraint
// -------------------------------------------------
describe('checkColumnConstraint', () => {
  const cases = [
    {
      name: 'no markers → no conflicts',
      grid: emptyGrid(5),
      expected: 0,
    },
    {
      name: '1 marker per column → no conflicts',
      grid: validSolution,
      expected: 0,
    },
    {
      name: '2 markers in same column → 1 conflict',
      grid: placeMarkers(emptyGrid(5), [
        [0, 2],
        [3, 2],
      ]),
      expected: 1,
    },
    {
      name: '3 markers in same column → 3 conflicts (all pairs)',
      grid: placeMarkers(emptyGrid(5), [
        [0, 1],
        [2, 1],
        [4, 1],
      ]),
      expected: 3,
    },
  ] as const;

  for (const { name, grid, expected } of cases) {
    it(name, () => {
      const conflicts = checkColumnConstraint(grid, 5);
      expect(conflicts).toHaveLength(expected);
    });
  }

  it('returns correct cell positions for column conflict', () => {
    const grid = placeMarkers(emptyGrid(5), [
      [0, 3],
      [4, 3],
    ]);
    const conflicts = checkColumnConstraint(grid, 5);
    expect(conflicts).toHaveLength(1);
    expect(conflicts[0]!.cells).toEqual([
      { row: 0, col: 3 },
      { row: 4, col: 3 },
    ]);
  });
});

// -------------------------------------------------
// checkRegionConstraint
// -------------------------------------------------
describe('checkRegionConstraint', () => {
  const cases = [
    {
      name: 'no markers → no conflicts',
      grid: emptyGrid(5),
      expected: 0,
    },
    {
      name: '1 marker per region → no conflicts',
      grid: validSolution,
      expected: 0,
    },
    {
      name: '2 markers in same region → 1 conflict',
      // Region 0 contains (0,0), (0,1), (1,0), (1,1)
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [1, 1],
      ]),
      expected: 1,
    },
    {
      name: 'markers in different regions → no conflicts',
      // Region 0: (0,0), Region 1: (0,2)
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [0, 2],
      ]),
      expected: 0,
    },
  ] as const;

  for (const { name, grid, expected } of cases) {
    it(name, () => {
      const conflicts = checkRegionConstraint(grid, regionMap);
      expect(conflicts).toHaveLength(expected);
    });
  }
});

// -------------------------------------------------
// checkAdjacencyConstraint
// -------------------------------------------------
describe('checkAdjacencyConstraint', () => {
  const cases = [
    {
      name: 'no markers → no conflicts',
      grid: emptyGrid(5),
      expected: 0,
    },
    {
      name: 'valid solution (no adjacent markers) → no conflicts',
      grid: validSolution,
      expected: 0,
    },
    {
      name: 'markers diagonally adjacent → 1 conflict',
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [1, 1],
      ]),
      expected: 1,
    },
    {
      name: 'markers horizontally adjacent → 1 conflict',
      grid: placeMarkers(emptyGrid(5), [
        [2, 2],
        [2, 3],
      ]),
      expected: 1,
    },
    {
      name: 'markers vertically adjacent → 1 conflict',
      grid: placeMarkers(emptyGrid(5), [
        [1, 3],
        [2, 3],
      ]),
      expected: 1,
    },
    {
      name: 'markers in same row but not adjacent → no conflicts',
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [0, 4],
      ]),
      expected: 0,
    },
    {
      name: 'markers separated by 1+ cells → no conflicts',
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [2, 2],
      ]),
      expected: 0,
    },
    {
      name: 'no double-counting: A-B conflict counted once',
      grid: placeMarkers(emptyGrid(5), [
        [0, 0],
        [0, 1],
      ]),
      expected: 1,
    },
  ] as const;

  for (const { name, grid, expected } of cases) {
    it(name, () => {
      const conflicts = checkAdjacencyConstraint(grid, 5);
      expect(conflicts).toHaveLength(expected);
    });
  }
});

// -------------------------------------------------
// getAllConflicts
// -------------------------------------------------
describe('getAllConflicts', () => {
  it('valid solution → no conflicts', () => {
    const conflicts = getAllConflicts(validSolution, regionMap, 5);
    expect(conflicts).toHaveLength(0);
  });

  it('combines violations from multiple constraint types', () => {
    // 2 markers in row 0, col 0, region 0, and adjacent
    // This triggers row, column won't (different cols), region, adjacency
    const grid = placeMarkers(emptyGrid(5), [
      [0, 0],
      [0, 1],
    ]);
    const conflicts = getAllConflicts(grid, regionMap, 5);
    // Row conflict: (0,0)-(0,1)
    // Region conflict: (0,0)-(0,1) — both in region 0
    // Adjacency conflict: (0,0)-(0,1)
    // All three involve the same pair → deduplicated to 1
    expect(conflicts).toHaveLength(1);
  });

  it('returns distinct conflicts for different pairs', () => {
    // Markers at (0,0), (0,2), (1,1) — all in different columns
    // Row conflict: (0,0)-(0,2)
    // Adjacency: (0,0)-(1,1) and (0,2)-(1,1)
    // Region: (0,0) is region 0, (0,2) is region 1, (1,1) is region 0
    //   → region conflict: (0,0)-(1,1)
    // So unique pairs: (0,0)-(0,2), (0,0)-(1,1), (0,2)-(1,1) = 3
    const grid = placeMarkers(emptyGrid(5), [
      [0, 0],
      [0, 2],
      [1, 1],
    ]);
    const conflicts = getAllConflicts(grid, regionMap, 5);
    expect(conflicts).toHaveLength(3);
  });
});
