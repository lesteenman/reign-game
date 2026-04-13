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

/** Simulate a tap: pointer down then pointer up on the same cell. */
function tap(result: { current: ReturnType<typeof useGame> }, row: number, col: number) {
  act(() => result.current.handlePointerDown(row, col));
  act(() => result.current.handlePointerUp());
}

describe('useGame', () => {
  it('initializes with all cells empty, no conflicts, not solved', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    expect(result.current.cells).toHaveLength(5);
    for (const row of result.current.cells) {
      expect(row).toHaveLength(5);
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
    expect(result.current.draggedCells.size).toBe(0);
  });

  it('tap empty cell changes to excluded', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0);
    expect(result.current.cells[0]![0]).toBe('excluded');
  });

  it('tap excluded cell changes to marked', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); // excluded
    tap(result, 0, 0); // marked
    expect(result.current.cells[0]![0]).toBe('marked');
  });

  it('tap marked cell changes to empty', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); // excluded
    tap(result, 0, 0); // marked
    tap(result, 0, 0); // empty
    expect(result.current.cells[0]![0]).toBe('empty');
  });

  it('detects conflicts when two markers in same row', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    tap(result, 0, 0); // excluded
    tap(result, 0, 0); // marked
    tap(result, 0, 1); // excluded
    tap(result, 0, 1); // marked
    expect(result.current.conflicts.length).toBeGreaterThan(0);
  });

  it('isSolved is true when valid solution placed', () => {
    const { result } = renderHook(() => useGame(testPuzzle));
    const solution: [number, number][] = [[0, 4], [1, 1], [2, 3], [3, 0], [4, 2]];
    for (const [row, col] of solution) {
      tap(result, row, col); // excluded
      tap(result, row, col); // marked
    }
    expect(result.current.conflicts).toEqual([]);
    expect(result.current.isSolved).toBe(true);
  });

  it('drag highlights cells without changing them, applies on release', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    act(() => result.current.handlePointerDown(0, 0));
    // Nothing changed yet
    expect(result.current.cells[0]![0]).toBe('empty');

    // Drag to adjacent cells
    act(() => result.current.handleDragEnter(0, 1));
    act(() => result.current.handleDragEnter(0, 2));

    // Cells highlighted but still empty
    expect(result.current.draggedCells.has('0,0')).toBe(true);
    expect(result.current.draggedCells.has('0,1')).toBe(true);
    expect(result.current.draggedCells.has('0,2')).toBe(true);
    expect(result.current.cells[0]![0]).toBe('empty');
    expect(result.current.cells[0]![1]).toBe('empty');
    expect(result.current.cells[0]![2]).toBe('empty');

    // Release: all get excluded
    act(() => result.current.handlePointerUp());
    expect(result.current.cells[0]![0]).toBe('excluded');
    expect(result.current.cells[0]![1]).toBe('excluded');
    expect(result.current.cells[0]![2]).toBe('excluded');
    expect(result.current.draggedCells.size).toBe(0);
  });

  it('drag highlights already-excluded cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Exclude (1,0) via tap
    tap(result, 1, 0);
    expect(result.current.cells[1]![0]).toBe('excluded');

    // Start drag from empty (0,0)
    act(() => result.current.handlePointerDown(0, 0));
    act(() => result.current.handleDragEnter(1, 0)); // already excluded

    expect(result.current.draggedCells.has('1,0')).toBe(true);

    act(() => result.current.handlePointerUp());
    // Stays excluded
    expect(result.current.cells[1]![0]).toBe('excluded');
  });

  it('drag does not highlight marked cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Mark (0,1)
    tap(result, 0, 1); // excluded
    tap(result, 0, 1); // marked

    // Start drag from empty (0,0)
    act(() => result.current.handlePointerDown(0, 0));
    act(() => result.current.handleDragEnter(0, 1)); // marked

    expect(result.current.draggedCells.has('0,1')).toBe(false);

    act(() => result.current.handlePointerUp());
    expect(result.current.cells[0]![1]).toBe('marked');
  });

  it('resetGame clears all cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    tap(result, 0, 0); // excluded
    tap(result, 1, 1); // excluded
    tap(result, 1, 1); // marked

    act(() => result.current.resetGame());

    for (const row of result.current.cells) {
      for (const cell of row) {
        expect(cell).toBe('empty');
      }
    }
  });
});
