import { useState, useCallback, useRef, useMemo } from 'react';
import type { CellState, Conflict, PuzzleData } from '../engine/types';
import { getAllConflicts } from '../engine/constraints';
import { validateSolution } from '../engine/validator';

/** Return value of the useGame hook. */
export interface UseGameReturn {
  cells: CellState[][];
  conflicts: Conflict[];
  isSolved: boolean;
  /** Set of "row,col" strings for cells highlighted during current drag. */
  draggedCells: Set<string>;
  handlePointerDown: (row: number, col: number) => void;
  handleDragEnter: (row: number, col: number) => void;
  handlePointerUp: () => void;
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
 * All interactions are deferred to pointer-up:
 * - Tap (down + up, no drag): three-tap cycle on that cell
 * - Drag (down + move + up): exclude all highlighted empty cells
 *
 * During drag, cells are highlighted but not modified.
 */
export function useGame(puzzle: PuzzleData): UseGameReturn {
  const { gridSize, regionMap } = puzzle;
  const [cells, setCells] = useState<CellState[][]>(() =>
    createEmptyCells(gridSize),
  );
  const [draggedCells, setDraggedCells] = useState<Set<string>>(new Set());

  // Track the starting cell and whether we've dragged
  const startCellRef = useRef<{ row: number; col: number } | null>(null);
  const hasDraggedRef = useRef(false);

  const conflicts = useMemo(
    () => getAllConflicts(cells, regionMap, gridSize),
    [cells, regionMap, gridSize],
  );

  const isSolved = useMemo(
    () => validateSolution(cells, regionMap, gridSize),
    [cells, regionMap, gridSize],
  );

  const handlePointerDown = useCallback((row: number, col: number) => {
    startCellRef.current = { row, col };
    hasDraggedRef.current = false;
    setDraggedCells(new Set());
  }, []);

  const handleDragEnter = useCallback((row: number, col: number) => {
    if (!startCellRef.current) return;

    const key = `${row},${col}`;
    const startKey = `${startCellRef.current.row},${startCellRef.current.col}`;

    // If we enter a different cell than the start, we're dragging
    if (key !== startKey) {
      hasDraggedRef.current = true;
    }

    if (!hasDraggedRef.current) return;

    setCells((prev) => {
      const currentState = prev[row]![col]!;
      // Highlight empty and already-excluded cells (not markers)
      if (currentState === 'empty' || currentState === 'excluded') {
        setDraggedCells((prev) => {
          const next = new Set(prev);
          // Also add the starting cell to the highlight
          next.add(startKey);
          next.add(key);
          return next;
        });
      }
      return prev;
    });
  }, []);

  const handlePointerUp = useCallback(() => {
    const start = startCellRef.current;
    if (!start) return;

    if (!hasDraggedRef.current) {
      // Single tap: apply three-tap cycle to starting cell
      setCells((prev) => {
        const currentState = prev[start.row]![start.col]!;
        const next = cloneCells(prev);
        next[start.row]![start.col] = nextCellState(currentState);
        return next;
      });
    } else {
      // Drag: exclude all highlighted empty cells
      setCells((prev) => {
        const next = cloneCells(prev);
        // Include the starting cell in the drag exclusion
        const startKey = `${start.row},${start.col}`;
        const allKeys = new Set(draggedCells);
        allKeys.add(startKey);
        for (const key of allKeys) {
          const [rowStr, colStr] = key.split(',');
          const r = Number(rowStr);
          const c = Number(colStr);
          if (next[r]?.[c] === 'empty') {
            next[r]![c] = 'excluded';
          }
        }
        return next;
      });
    }

    startCellRef.current = null;
    hasDraggedRef.current = false;
    setDraggedCells(new Set());
  }, [draggedCells]);

  const resetGame = useCallback(() => {
    setCells(createEmptyCells(gridSize));
    startCellRef.current = null;
    hasDraggedRef.current = false;
    setDraggedCells(new Set());
  }, [gridSize]);

  return {
    cells,
    conflicts,
    isSolved,
    draggedCells,
    handlePointerDown,
    handleDragEnter,
    handlePointerUp,
    resetGame,
  };
}
