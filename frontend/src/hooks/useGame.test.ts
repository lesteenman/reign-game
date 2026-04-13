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

  it('clicking empty cell changes to excluded', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    act(() => {
      result.current.handleCellPointerDown(0, 0);
    });

    expect(result.current.cells[0]![0]).toBe('excluded');
  });

  it('clicking excluded cell changes to marked', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    act(() => {
      result.current.handleCellPointerDown(0, 0); // empty -> excluded
    });
    act(() => {
      result.current.handleCellPointerDown(0, 0); // excluded -> marked
    });

    expect(result.current.cells[0]![0]).toBe('marked');
  });

  it('clicking marked cell changes to empty', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    act(() => {
      result.current.handleCellPointerDown(0, 0); // empty -> excluded
    });
    act(() => {
      result.current.handleCellPointerDown(0, 0); // excluded -> marked
    });
    act(() => {
      result.current.handleCellPointerDown(0, 0); // marked -> empty
    });

    expect(result.current.cells[0]![0]).toBe('empty');
  });

  it('completes full three-tap cycle', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // empty -> excluded -> marked -> empty
    act(() => result.current.handleCellPointerDown(2, 3));
    expect(result.current.cells[2]![3]).toBe('excluded');

    act(() => result.current.handleCellPointerDown(2, 3));
    expect(result.current.cells[2]![3]).toBe('marked');

    act(() => result.current.handleCellPointerDown(2, 3));
    expect(result.current.cells[2]![3]).toBe('empty');
  });

  it('detects conflicts when two markers in same row', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Place marker at (0,0): two taps
    act(() => result.current.handleCellPointerDown(0, 0)); // excluded
    act(() => result.current.handleCellPointerDown(0, 0)); // marked

    // Place marker at (0,1): two taps
    act(() => result.current.handleCellPointerDown(0, 1)); // excluded
    act(() => result.current.handleCellPointerDown(0, 1)); // marked

    expect(result.current.conflicts.length).toBeGreaterThan(0);
    expect(result.current.isSolved).toBe(false);
  });

  it('isSolved is true when valid solution placed', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Valid solution: (0,4), (1,1), (2,3), (3,0), (4,2)
    const solution: [number, number][] = [
      [0, 4],
      [1, 1],
      [2, 3],
      [3, 0],
      [4, 2],
    ];

    for (const [row, col] of solution) {
      act(() => result.current.handleCellPointerDown(row, col)); // excluded
      act(() => result.current.handleCellPointerDown(row, col)); // marked
    }

    expect(result.current.conflicts).toEqual([]);
    expect(result.current.isSolved).toBe(true);
  });

  it('drag from empty: starting cell excluded, entering empty cells excludes them', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Pointer down on empty cell starts drag with exclude mode
    act(() => {
      result.current.handleCellPointerDown(0, 0);
    });
    expect(result.current.cells[0]![0]).toBe('excluded');

    act(() => {
      result.current.handleDragEnter(0, 1);
    });
    expect(result.current.cells[0]![1]).toBe('excluded');

    act(() => {
      result.current.handleDragEnter(0, 2);
    });
    expect(result.current.cells[0]![2]).toBe('excluded');

    act(() => {
      result.current.handleDragEnd();
    });
  });

  it('drag from excluded: starting cell becomes marked, entering excluded cells clears them', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // First exclude some cells
    act(() => result.current.handleCellPointerDown(1, 0)); // excluded
    act(() => result.current.handleDragEnd());
    act(() => result.current.handleCellPointerDown(1, 1)); // excluded
    act(() => result.current.handleDragEnd());
    act(() => result.current.handleCellPointerDown(1, 2)); // excluded
    act(() => result.current.handleDragEnd());

    // Pointer down on excluded cell: three-tap cycles to marked, drag mode is 'clear'
    act(() => {
      result.current.handleCellPointerDown(1, 0);
    });
    expect(result.current.cells[1]![0]).toBe('marked');

    // Drag enter on excluded cells clears them
    act(() => {
      result.current.handleDragEnter(1, 1);
    });
    expect(result.current.cells[1]![1]).toBe('empty');

    act(() => {
      result.current.handleDragEnter(1, 2);
    });
    expect(result.current.cells[1]![2]).toBe('empty');

    act(() => {
      result.current.handleDragEnd();
    });
  });

  it('drag from marker: no drag effect', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Place a marker at (0,0): two taps
    act(() => result.current.handleCellPointerDown(0, 0)); // excluded
    act(() => result.current.handleDragEnd());
    act(() => result.current.handleCellPointerDown(0, 0)); // marked
    act(() => result.current.handleDragEnd());

    // Pointer down on marker: three-tap cycles to empty, no drag
    act(() => {
      result.current.handleCellPointerDown(0, 0);
    });
    expect(result.current.cells[0]![0]).toBe('empty');

    // Drag entering empty cell should not change it (dragMode is null for marked)
    act(() => {
      result.current.handleDragEnter(0, 1);
    });
    expect(result.current.cells[0]![1]).toBe('empty');

    act(() => {
      result.current.handleDragEnd();
    });
  });

  it('drag exclude mode does not affect non-empty cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Place a marker at (0,1)
    act(() => result.current.handleCellPointerDown(0, 1)); // excluded
    act(() => result.current.handleDragEnd());
    act(() => result.current.handleCellPointerDown(0, 1)); // marked
    act(() => result.current.handleDragEnd());

    // Start exclude drag from (0,0) which is empty
    act(() => {
      result.current.handleCellPointerDown(0, 0);
    });
    expect(result.current.cells[0]![0]).toBe('excluded');

    // Enter the marked cell - should not change it
    act(() => {
      result.current.handleDragEnter(0, 1);
    });
    expect(result.current.cells[0]![1]).toBe('marked');
  });

  it('resetGame clears all cells', () => {
    const { result } = renderHook(() => useGame(testPuzzle));

    // Make some changes
    act(() => result.current.handleCellPointerDown(0, 0)); // excluded
    act(() => result.current.handleCellPointerDown(1, 1)); // excluded
    act(() => result.current.handleCellPointerDown(1, 1)); // marked

    act(() => {
      result.current.resetGame();
    });

    for (const row of result.current.cells) {
      for (const cell of row) {
        expect(cell).toBe('empty');
      }
    }
    expect(result.current.conflicts).toEqual([]);
    expect(result.current.isSolved).toBe(false);
  });
});
