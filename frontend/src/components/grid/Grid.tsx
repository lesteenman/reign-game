import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CellState, Conflict, PuzzleData } from '../../engine/types';
import { Cell } from './Cell';

/** Props for the Grid component. */
export interface GridProps {
  puzzle: PuzzleData;
  cells: CellState[][];
  conflicts: Conflict[];
  isSolved: boolean;
  draggedCells: Set<string>;
  onPointerDown: (row: number, col: number) => void;
  onPointerUp: () => void;
  onDragEnter: (row: number, col: number) => void;
}

/**
 * The main puzzle grid component.
 * Renders a CSS Grid of Cell components with region boundaries,
 * conflict highlighting, and drag support.
 */
export function Grid({
  puzzle,
  cells,
  conflicts,
  isSolved,
  draggedCells,
  onPointerDown,
  onPointerUp,
  onDragEnter,
}: GridProps) {
  const { gridSize, regionMap } = puzzle;
  const containerRef = useRef<HTMLDivElement>(null);
  const [cellSize, setCellSize] = useState(60);

  // Measure container width and compute cell size
  useEffect(() => {
    function measure() {
      const container = containerRef.current?.parentElement;
      if (!container) return;
      const available = container.clientWidth - 5;
      const size = Math.floor(available / gridSize);
      setCellSize(Math.max(44, Math.min(72, size)));
    }

    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [gridSize]);

  // Build a Set of conflicting cell positions for O(1) lookup
  const conflictSet = useMemo(() => {
    const set = new Set<string>();
    for (const conflict of conflicts) {
      for (const pos of conflict.cells) {
        set.add(`${pos.row},${pos.col}`);
      }
    }
    return set;
  }, [conflicts]);

  const handleTouchMove = useCallback(
    (e: React.TouchEvent) => {
      if (isSolved) return;
      const touch = e.touches[0];
      if (!touch) return;
      const element = document.elementFromPoint(touch.clientX, touch.clientY);
      if (element) {
        const cellEl =
          element.closest<HTMLElement>('[data-row][data-col]') ?? element;
        const rowAttr = cellEl.getAttribute('data-row');
        const colAttr = cellEl.getAttribute('data-col');
        if (rowAttr !== null && colAttr !== null) {
          onDragEnter(Number(rowAttr), Number(colAttr));
        }
      }
    },
    [isSolved, onDragEnter],
  );

  return (
    <div
      ref={containerRef}
      data-testid="game-grid"
      style={{
        display: 'inline-grid',
        gridTemplateColumns: `repeat(${gridSize}, ${cellSize}px)`,
        gridTemplateRows: `repeat(${gridSize}, ${cellSize}px)`,
        border: '2.5px solid var(--color-ink)',
        borderRadius: 'var(--radius)',
        boxShadow: '0 3px 0 var(--color-ink)',
        overflow: 'hidden',
        userSelect: 'none',
        touchAction: 'none',
        maxWidth: '100%',
      }}
      onMouseLeave={onPointerUp}
      onTouchEnd={onPointerUp}
      onTouchMove={handleTouchMove}
    >
      {Array.from({ length: gridSize }, (_, row) =>
        Array.from({ length: gridSize }, (_, col) => {
          const regionIndex = regionMap[row]![col]!;
          const cellState = cells[row]![col]!;
          const hasConflict = conflictSet.has(`${row},${col}`);
          const isDragHighlighted = draggedCells.has(`${row},${col}`);

          const bTop =
            row > 0 && regionMap[row]![col] !== regionMap[row - 1]![col];
          const bRight =
            col < gridSize - 1 &&
            regionMap[row]![col] !== regionMap[row]![col + 1];
          const bBottom =
            row < gridSize - 1 &&
            regionMap[row]![col] !== regionMap[row + 1]![col];
          const bLeft =
            col > 0 && regionMap[row]![col] !== regionMap[row]![col - 1];

          return (
            <Cell
              key={`${row}-${col}`}
              row={row}
              col={col}
              state={cellState}
              regionIndex={regionIndex}
              hasConflict={hasConflict}
              isDragHighlighted={isDragHighlighted}
              cellSize={cellSize}
              borderTop={bTop}
              borderRight={bRight}
              borderBottom={bBottom}
              borderLeft={bLeft}
              onPointerDown={
                isSolved ? () => {} : () => onPointerDown(row, col)
              }
              onPointerUp={
                isSolved ? () => {} : onPointerUp
              }
              onDragEnter={
                isSolved ? () => {} : () => onDragEnter(row, col)
              }
            />
          );
        }),
      )}
    </div>
  );
}
