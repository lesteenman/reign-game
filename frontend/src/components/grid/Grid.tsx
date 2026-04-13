import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CellState, Conflict, PuzzleData } from '../../engine/types';
import { Cell } from './Cell';
import { RegionBorderOverlay } from './RegionBorderOverlay';

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
 * Cells render with subtle internal borders. Region boundaries
 * are drawn as an SVG overlay on top.
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
  const [cellSize, setCellSize] = useState<number | null>(null);

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

  // Don't render grid content until we've measured the container
  if (cellSize === null) {
    return <div ref={containerRef} style={{ width: '100%' }} />;
  }

  return (
    <div
      ref={containerRef}
      {...(__TEST_ATTRS__ ? { 'data-testid': 'game-grid' } : {})}
      style={{
        position: 'relative',
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
      <RegionBorderOverlay
        regionMap={regionMap}
        gridSize={gridSize}
        cellSize={cellSize}
      />
    </div>
  );
}
