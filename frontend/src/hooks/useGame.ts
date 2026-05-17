import { useState, useCallback, useRef, useMemo, useReducer } from 'react';
import type { CellState, Conflict, PuzzleData } from '../engine/types';
import { getAllConflicts } from '../engine/constraints';
import { cellKey } from '../engine/cellKey';
import type { GameHistory } from '../storage/types';
import { EMPTY_HISTORY } from '../storage/types';

type DragIntent = 'exclude' | 'clear' | null;

/** Cap on past-stack depth. Dropped from the oldest end when exceeded. */
const HISTORY_LIMIT = 200;

/** Return value of the useGame hook. */
export interface UseGameReturn {
  cells: CellState[][];
  conflicts: Conflict[];
  isSolved: boolean;
  /** Set of "row,col" strings for cells highlighted during current drag. */
  draggedCells: Set<string>;
  canUndo: boolean;
  canRedo: boolean;
  /** Current history stacks (for persistence). */
  history: GameHistory;
  handlePointerDown: (row: number, col: number) => void;
  handleDragEnter: (row: number, col: number) => void;
  handlePointerUp: () => void;
  resetGame: () => void;
  undo: () => void;
  redo: () => void;
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

function cellsEqual(a: CellState[][], b: CellState[][]): boolean {
  if (a.length !== b.length) return false;
  for (let r = 0; r < a.length; r++) {
    const rowA = a[r]!;
    const rowB = b[r]!;
    if (rowA.length !== rowB.length) return false;
    for (let c = 0; c < rowA.length; c++) {
      if (rowA[c] !== rowB[c]) return false;
    }
  }
  return true;
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
 *
 * GameHistory is a snapshot stack: every mutation that actually changes cells
 * pushes the pre-mutation grid onto `past` and clears `future`. Undo pops
 * from `past` onto the current grid (and pushes the current grid onto
 * `future`); redo is the mirror. Reset is treated as a normal mutation, so
 * Ctrl+Z can recover from an accidental reset.
 */
interface HistoryState {
  cells: CellState[][];
  past: CellState[][][];
  future: CellState[][][];
}

type HistoryAction =
  | { type: 'commit'; compute: (prev: CellState[][]) => CellState[][] }
  | { type: 'undo' }
  | { type: 'redo' };

function historyReducer(state: HistoryState, action: HistoryAction): HistoryState {
  switch (action.type) {
    case 'commit': {
      const next = action.compute(state.cells);
      if (cellsEqual(state.cells, next)) return state;
      // Drop the oldest entry in one allocation when at the cap, rather than
      // allocating a length-(LIMIT+1) array and slicing it back down.
      const past = state.past.length >= HISTORY_LIMIT
        ? [...state.past.slice(-(HISTORY_LIMIT - 1)), state.cells]
        : [...state.past, state.cells];
      return { cells: next, past, future: [] };
    }
    case 'undo': {
      if (state.past.length === 0) return state;
      const prev = state.past[state.past.length - 1]!;
      return {
        cells: prev,
        past: state.past.slice(0, -1),
        future: [...state.future, state.cells],
      };
    }
    case 'redo': {
      if (state.future.length === 0) return state;
      const next = state.future[state.future.length - 1]!;
      return {
        cells: next,
        past: [...state.past, state.cells],
        future: state.future.slice(0, -1),
      };
    }
  }
}

export function useGame(
  puzzle: PuzzleData,
  initialCells?: CellState[][],
  initialHistory?: GameHistory,
): UseGameReturn {
  const { gridSize, regionMap } = puzzle;
  const [{ cells, past, future }, dispatch] = useReducer(
    historyReducer,
    undefined,
    (): HistoryState => ({
      cells: initialCells ?? createEmptyCells(gridSize),
      past: initialHistory?.past ?? EMPTY_HISTORY.past,
      future: initialHistory?.future ?? EMPTY_HISTORY.future,
    }),
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

  // cellsRef mirrors the reducer's cells so handlePointerDown can stay
  // reference-stable — it reads cell state at the moment of pointer-down,
  // not at callback setup, so a ref is the right shape.
  const cellsRef = useRef(cells);
  cellsRef.current = cells;

  // Commit dispatches a pure compute function; the reducer owns the state
  // read, so no external cells mirror is needed on the write path.
  const commit = useCallback((compute: (prev: CellState[][]) => CellState[][]) => {
    dispatch({ type: 'commit', compute });
  }, []);

  // Clear all drag-tracking refs + state. Shared by pointerUp (end of drag)
  // and resetGame (throw away any in-progress drag).
  const clearDragState = useCallback(() => {
    startCellRef.current = null;
    hasDraggedRef.current = false;
    dragIntentRef.current = null;
    draggedCellsRef.current = new Set();
    setDraggedCells(new Set());
  }, []);

  const handlePointerDown = useCallback((row: number, col: number) => {
    startCellRef.current = { row, col };
    hasDraggedRef.current = false;
    draggedCellsRef.current = new Set();
    setDraggedCells(new Set());

    const state = cellsRef.current[row]?.[col];
    if (state === 'empty') {
      dragIntentRef.current = 'exclude';
    } else if (state === 'excluded') {
      dragIntentRef.current = 'clear';
    } else {
      dragIntentRef.current = null;
    }
  }, []);

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
      commit((prev) => {
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
      commit((prev) => {
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

    clearDragState();
  }, [commit, clearDragState]);

  const resetGame = useCallback(() => {
    commit(() => createEmptyCells(gridSize));
    clearDragState();
  }, [commit, clearDragState, gridSize]);

  const undo = useCallback(() => dispatch({ type: 'undo' }), []);
  const redo = useCallback(() => dispatch({ type: 'redo' }), []);

  // past and future come from reducer state; every commit/undo/redo produces
  // new array references already, so a useMemo wrapper would only add a
  // shallow-equal check that can never skip.
  const history: GameHistory = { past, future };

  return {
    cells,
    conflicts,
    isSolved,
    draggedCells,
    canUndo: past.length > 0,
    canRedo: future.length > 0,
    history,
    handlePointerDown,
    handleDragEnter,
    handlePointerUp,
    resetGame,
    undo,
    redo,
  };
}
