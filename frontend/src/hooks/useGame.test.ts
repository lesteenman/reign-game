import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useGame } from './useGame';
import type { PuzzleData } from '../engine/types';

const testPuzzle: PuzzleData = {
  puzzleId: 'test-1',
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

function tap(result: { current: ReturnType<typeof useGame> }, row: number, col: number) {
  act(() => result.current.handlePointerDown(row, col));
  act(() => result.current.handlePointerUp());
}

describe('useGame', () => {
  it('initializes with all cells empty', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    for (const row of result.current.cells) {
      for (const cell of row) {
        expect(cell).toBe('empty');
      }
    }
    expect(result.current.conflicts).toEqual([]);
    expect(result.current.isSolved).toBe(false);
  });

  it('nothing changes on pointer down alone', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    act(() => result.current.handlePointerDown(0, 0));
    expect(result.current.cells[0]![0]).toBe('empty');
  });

  it('three-tap cycle: empty → excluded → marked → empty', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0);
    expect(result.current.cells[0]![0]).toBe('excluded');
    tap(result, 0, 0);
    expect(result.current.cells[0]![0]).toBe('marked');
    tap(result, 0, 0);
    expect(result.current.cells[0]![0]).toBe('empty');
  });

  it('detects conflicts when two markers in same row', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); tap(result, 0, 0); // marked
    tap(result, 0, 1); tap(result, 0, 1); // marked
    expect(result.current.conflicts.length).toBeGreaterThan(0);
  });

  it('isSolved when valid solution placed', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    for (const [r, c] of [[0,4],[1,1],[2,3],[3,0],[4,2]] as [number,number][]) {
      tap(result, r, c); tap(result, r, c);
    }
    expect(result.current.isSolved).toBe(true);
  });

  it('drag from empty: highlights and excludes on release', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    act(() => result.current.handlePointerDown(0, 0));
    act(() => result.current.handleDragEnter(0, 1));
    act(() => result.current.handleDragEnter(0, 2));

    // Highlighted but not yet excluded
    expect(result.current.draggedCells.has('0,1')).toBe(true);
    expect(result.current.draggedCells.has('0,2')).toBe(true);
    expect(result.current.cells[0]![1]).toBe('empty');

    act(() => result.current.handlePointerUp());

    // Now excluded
    expect(result.current.cells[0]![0]).toBe('excluded');
    expect(result.current.cells[0]![1]).toBe('excluded');
    expect(result.current.cells[0]![2]).toBe('excluded');
    expect(result.current.draggedCells.size).toBe(0);
  });

  it('drag from excluded: highlights and clears on release', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Exclude some cells first
    tap(result, 1, 0); // excluded
    tap(result, 1, 1); // excluded
    tap(result, 1, 2); // excluded

    // Drag from excluded cell (1,0)
    act(() => result.current.handlePointerDown(1, 0));
    act(() => result.current.handleDragEnter(1, 1));
    act(() => result.current.handleDragEnter(1, 2));

    expect(result.current.draggedCells.has('1,1')).toBe(true);
    expect(result.current.cells[1]![1]).toBe('excluded'); // not cleared yet

    act(() => result.current.handlePointerUp());

    // All cleared
    expect(result.current.cells[1]![0]).toBe('empty');
    expect(result.current.cells[1]![1]).toBe('empty');
    expect(result.current.cells[1]![2]).toBe('empty');
  });

  it('drag from marked: treated as tap (three-tap cycle)', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); tap(result, 0, 0); // marked

    // Pressing and moving from a marked cell has no drag intent,
    // so it falls through as a tap: marked → empty
    act(() => result.current.handlePointerDown(0, 0));
    act(() => result.current.handleDragEnter(0, 1));

    expect(result.current.draggedCells.size).toBe(0);

    act(() => result.current.handlePointerUp());
    expect(result.current.cells[0]![0]).toBe('empty');
    expect(result.current.cells[0]![1]).toBe('empty'); // untouched
  });

  it('drag highlights marked cells but does not change them', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 1); tap(result, 0, 1); // marked

    act(() => result.current.handlePointerDown(0, 0)); // empty → exclude intent
    act(() => result.current.handleDragEnter(0, 1)); // marked → highlighted

    expect(result.current.draggedCells.has('0,1')).toBe(true);
    act(() => result.current.handlePointerUp());
    expect(result.current.cells[0]![1]).toBe('marked'); // unchanged
  });

  it('drag clear highlights empty cells but does not change them', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 1, 0); // excluded

    act(() => result.current.handlePointerDown(1, 0)); // excluded → clear intent
    act(() => result.current.handleDragEnter(1, 1)); // empty → highlighted

    expect(result.current.draggedCells.has('1,1')).toBe(true);
    act(() => result.current.handlePointerUp());
    expect(result.current.cells[1]![0]).toBe('empty'); // was excluded, now cleared
    expect(result.current.cells[1]![1]).toBe('empty'); // was already empty, stays empty
  });

  it('no drag highlight on single tap', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    act(() => result.current.handlePointerDown(0, 0));
    expect(result.current.draggedCells.size).toBe(0);
    act(() => result.current.handlePointerUp());
    expect(result.current.draggedCells.size).toBe(0);
  });

  it('resetGame clears all cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); tap(result, 1, 1); tap(result, 1, 1);

    act(() => result.current.resetGame());

    for (const row of result.current.cells) {
      for (const cell of row) {
        expect(cell).toBe('empty');
      }
    }
  });
});

describe('useGame (Double Queens, mode=double)', () => {
  // 4x4 grid with 4 regions, each 2x2
  const doublePuzzle: PuzzleData = {
    puzzleId: 'double-1',
    gridSize: 4,
    mode: 'double',
    regionMap: [
      [0, 0, 1, 1],
      [0, 0, 1, 1],
      [2, 2, 3, 3],
      [2, 2, 3, 3],
    ],
  };

  it('2 markers in same row → no conflict for double mode', () => {
    // Arrange
    const initialCells: import('../engine/types').CellState[][] = [
      ['marked', 'empty', 'empty', 'marked'],
      ['empty', 'empty', 'empty', 'empty'],
      ['empty', 'empty', 'empty', 'empty'],
      ['empty', 'empty', 'empty', 'empty'],
    ];

    // Act
    const { result } = renderHook(() => useGame(doublePuzzle, initialCells));

    // Assert
    expect(result.current.conflicts).toHaveLength(0);
  });

  it('3 markers in same row → conflict for double mode', () => {
    // Arrange
    const initialCells: import('../engine/types').CellState[][] = [
      ['marked', 'marked', 'empty', 'marked'],
      ['empty', 'empty', 'empty', 'empty'],
      ['empty', 'empty', 'empty', 'empty'],
      ['empty', 'empty', 'empty', 'empty'],
    ];

    // Act
    const { result } = renderHook(() => useGame(doublePuzzle, initialCells));

    // Assert
    expect(result.current.conflicts.length).toBeGreaterThan(0);
  });

  it('isSolved requires gridSize * 2 markers for double mode', () => {
    // Arrange
    // Only 4 markers (gridSize=4, markersPerUnit=2, need 8) — not solved
    const initialCells: import('../engine/types').CellState[][] = [
      ['marked', 'empty', 'empty', 'marked'],
      ['empty', 'empty', 'empty', 'empty'],
      ['empty', 'empty', 'empty', 'empty'],
      ['marked', 'empty', 'empty', 'marked'],
    ];

    // Act
    const { result } = renderHook(() => useGame(doublePuzzle, initialCells));

    // Assert
    expect(result.current.isSolved).toBe(false);
  });
});
