import { useState, useCallback, useRef, useMemo } from 'react';
import type { CellState, Conflict, PuzzleData } from '../engine/types';
import { getAllConflicts } from '../engine/constraints';

type DragIntent = 'exclude' | 'clear' | null;

export function cellKey(row: number, col: number): string {
  return `${row},${col}`;
}

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
 * - Drag (down + move + up): intent-based
 *   - Started on empty cell → exclude all highlighted empty cells
 *   - Started on excluded cell → clear all highlighted excluded cells
 *   - Started on marked cell → no drag
 *
 * During drag, cells matching the intent are highlighted but not modified.
 */
export function useGame(puzzle: PuzzleData, initialCells?: CellState[][]): UseGameReturn {
  const { gridSize, regionMap } = puzzle;
  const [cells, setCells] = useState<CellState[][]>(() =>
    initialCells ?? createEmptyCells(gridSize),
  );
  // draggedCells is tracked in a ref (for stable callbacks) and mirrored
  // to state (for re-render on highlight changes).
  const draggedCellsRef = useRef<Set<string>>(new Set());
  const [draggedCells, setDraggedCells] = useState<Set<string>>(new Set());

  const startCellRef = useRef<{ row: number; col: number } | null>(null);
  const hasDraggedRef = useRef(false);
  const dragIntentRef = useRef<DragIntent>(null);

  const markersPerUnit = puzzle.mode === 'double' ? 2 : 1;

  const conflicts = useMemo(
    () => getAllConflicts(cells, regionMap, gridSize, markersPerUnit),
    [cells, regionMap, gridSize, markersPerUnit],
  );

  // Solved when exactly gridSize * markersPerUnit markers and zero conflicts
  const isSolved = useMemo(() => {
    let markerCount = 0;
    for (const row of cells) {
      for (const cell of row) {
        if (cell === 'marked') markerCount++;
      }
    }
    return markerCount === gridSize * markersPerUnit && conflicts.length === 0;
  }, [cells, gridSize, markersPerUnit, conflicts]);

  const handlePointerDown = useCallback((row: number, col: number) => {
    startCellRef.current = { row, col };
    hasDraggedRef.current = false;
    draggedCellsRef.current = new Set();
    setDraggedCells(new Set());

    // Read cell state directly from the cells ref captured by closure.
    // This is safe because cells state doesn't change during pointer-down.
    const state = cells[row]?.[col];
    if (state === 'empty') {
      dragIntentRef.current = 'exclude';
    } else if (state === 'excluded') {
      dragIntentRef.current = 'clear';
    } else {
      dragIntentRef.current = null;
    }
  }, [cells]);

  const handleDragEnter = useCallback((row: number, col: number) => {
    if (!startCellRef.current) return;
    if (dragIntentRef.current === null) return;

    const key = cellKey(row, col);
    const startKey = cellKey(startCellRef.current.row, startCellRef.current.col);

    if (key !== startKey) {
      hasDraggedRef.current = true;
    }

    if (!hasDraggedRef.current) return;

    // Update the ref (stable, no closure issue) then flush to state
    const next = new Set(draggedCellsRef.current);
    next.add(startKey);
    next.add(key);
    draggedCellsRef.current = next;
    setDraggedCells(next);
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
      // Drag: apply intent to all highlighted cells (including start)
      const intent = dragIntentRef.current;
      const allKeys = new Set(draggedCellsRef.current);
      allKeys.add(cellKey(start.row, start.col));
      setCells((prev) => {
        const next = cloneCells(prev);
        for (const key of allKeys) {
          const [rowStr, colStr] = key.split(',');
          const r = Number(rowStr);
          const c = Number(colStr);
          if (intent === 'exclude' && next[r]?.[c] === 'empty') {
            next[r]![c] = 'excluded';
          } else if (intent === 'clear' && next[r]?.[c] === 'excluded') {
            next[r]![c] = 'empty';
          }
        }
        return next;
      });
    }

    startCellRef.current = null;
    hasDraggedRef.current = false;
    dragIntentRef.current = null;
    draggedCellsRef.current = new Set();
    setDraggedCells(new Set());
  }, []);

  const resetGame = useCallback(() => {
    setCells(createEmptyCells(gridSize));
    startCellRef.current = null;
    hasDraggedRef.current = false;
    dragIntentRef.current = null;
    draggedCellsRef.current = new Set();
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
