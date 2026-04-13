import { useState, useCallback, useRef, useMemo } from 'react';
import type { CellState, Conflict, PuzzleData } from '../engine/types';
import { getAllConflicts } from '../engine/constraints';
import { validateSolution } from '../engine/validator';

type DragMode = 'exclude' | 'clear' | null;

/** Return value of the useGame hook. */
export interface UseGameReturn {
  cells: CellState[][];
  conflicts: Conflict[];
  isSolved: boolean;
  /**
   * Handles a pointer-down on a cell: applies the three-tap cycle
   * AND sets up drag mode for subsequent drag-enter events.
   */
  handleCellPointerDown: (row: number, col: number) => void;
  handleDragEnter: (row: number, col: number) => void;
  handleDragEnd: () => void;
  resetGame: () => void;
}

function createEmptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from<CellState>({ length: size }).fill('empty'),
  );
}

function nextCellState(current: CellState): CellState {
  switch (current) {
    case 'empty':
      return 'excluded';
    case 'excluded':
      return 'marked';
    case 'marked':
      return 'empty';
  }
}

function cloneCells(cells: CellState[][]): CellState[][] {
  return cells.map((row) => [...row]);
}

/**
 * Custom hook that manages the full game state for a puzzle.
 * Handles cell interactions (three-tap cycle + drag) and game reset.
 */
export function useGame(puzzle: PuzzleData): UseGameReturn {
  const { gridSize, regionMap } = puzzle;
  const [cells, setCells] = useState<CellState[][]>(() =>
    createEmptyCells(gridSize),
  );
  const dragModeRef = useRef<DragMode>(null);

  const conflicts = useMemo(
    () => getAllConflicts(cells, regionMap, gridSize),
    [cells, regionMap, gridSize],
  );

  const isSolved = useMemo(
    () => validateSolution(cells, regionMap, gridSize),
    [cells, regionMap, gridSize],
  );

  const handleCellPointerDown = useCallback((row: number, col: number) => {
    setCells((prev) => {
      const currentState = prev[row]![col]!;
      const newState = nextCellState(currentState);
      const next = cloneCells(prev);
      next[row]![col] = newState;

      // Set drag mode based on the ORIGINAL state of the starting cell:
      // - Was empty (now excluded) → drag excludes other empty cells
      // - Was excluded (now marked) → drag clears other excluded cells
      // - Was marked (now empty) → no drag
      if (currentState === 'empty') {
        dragModeRef.current = 'exclude';
      } else if (currentState === 'excluded') {
        dragModeRef.current = 'clear';
      } else {
        dragModeRef.current = null;
      }

      return next;
    });
  }, []);

  const handleDragEnter = useCallback((row: number, col: number) => {
    const mode = dragModeRef.current;
    if (mode === null) return;

    setCells((prev) => {
      const currentState = prev[row]![col]!;

      if (mode === 'exclude' && currentState === 'empty') {
        const next = cloneCells(prev);
        next[row]![col] = 'excluded';
        return next;
      }

      if (mode === 'clear' && currentState === 'excluded') {
        const next = cloneCells(prev);
        next[row]![col] = 'empty';
        return next;
      }

      return prev;
    });
  }, []);

  const handleDragEnd = useCallback(() => {
    dragModeRef.current = null;
  }, []);

  const resetGame = useCallback(() => {
    setCells(createEmptyCells(gridSize));
    dragModeRef.current = null;
  }, [gridSize]);

  return {
    cells,
    conflicts,
    isSolved,
    handleCellPointerDown,
    handleDragEnter,
    handleDragEnd,
    resetGame,
  };
}
