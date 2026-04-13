import { useRef } from 'react';
import type { CellState } from '../../engine/types';
import { useTheme } from '../../theme/ThemeContext';

/** Props for the Cell component. */
export interface CellProps {
  row: number;
  col: number;
  state: CellState;
  regionIndex: number;
  hasConflict: boolean;
  isDragHighlighted: boolean;
  cellSize: number;
  /** Right side is a region boundary (different region to the right). */
  borderRight: boolean;
  /** Bottom side is a region boundary (different region below). */
  borderBottom: boolean;
  /** Adjacent cell to the LEFT has a region boundary on its right side. */
  adjacentLeftHasBorder: boolean;
  /** Adjacent cell ABOVE has a region boundary on its bottom side. */
  adjacentTopHasBorder: boolean;
  onPointerDown: () => void;
  onPointerUp: () => void;
  onDragEnter: () => void;
}

/** A single cell in the puzzle grid. */
export function Cell({
  row,
  col,
  state,
  regionIndex,
  hasConflict,
  isDragHighlighted,
  cellSize,
  borderRight,
  borderBottom,
  adjacentLeftHasBorder,
  adjacentTopHasBorder,
  onPointerDown,
  onPointerUp,
  onDragEnter,
}: CellProps) {
  const theme = useTheme();
  const touchedRef = useRef(false);

  let backgroundColor = `var(--region-${regionIndex}-fill)`;
  if (hasConflict) {
    backgroundColor = 'var(--color-destructive-bg)';
  } else if (isDragHighlighted) {
    backgroundColor = `color-mix(in srgb, var(--region-${regionIndex}-fill) 85%, var(--color-ink) 15%)`;
  }

  const MarkerComponent = theme.marker;
  const ExclusionMarkComponent = theme.exclusionMark;
  const markerSize = cellSize * 0.6;

  const handleMouseDown = (e: React.MouseEvent) => {
    if (touchedRef.current) {
      touchedRef.current = false;
      return;
    }
    e.preventDefault();
    onPointerDown();
  };

  const handleMouseUp = (e: React.MouseEvent) => {
    if (touchedRef.current) return;
    e.preventDefault();
    onPointerUp();
  };

  const handleMouseEnter = (e: React.MouseEvent) => {
    if (e.buttons === 1) {
      onDragEnter();
    }
  };

  const handleTouchStart = () => {
    touchedRef.current = true;
    onPointerDown();
    setTimeout(() => { touchedRef.current = false; }, 300);
  };

  let animationClass = '';
  if (state === 'marked' && hasConflict) {
    animationClass = theme.animations.conflict;
  } else if (state === 'marked') {
    animationClass = theme.animations.placement;
  }

  // Borders: each cell only draws its RIGHT and BOTTOM borders.
  // The top/left are handled by the cell above/to the left.
  // Internal cell borders (same region) are thin and subtle.
  // Right: region boundary or internal (if not suppressed by adjacent)
  const rightW = borderRight ? 2.5 : 1;
  const rightC = borderRight ? 'var(--color-ink)' : 'rgba(0,0,0,0.07)';
  // Bottom: region boundary or internal
  const bottomW = borderBottom ? 2.5 : 1;
  const bottomC = borderBottom ? 'var(--color-ink)' : 'rgba(0,0,0,0.07)';
  // Top: only an internal border if the cell above doesn't already have
  // a region border on its bottom. Skip entirely if it does.
  const topW = adjacentTopHasBorder ? 0 : 1;
  const topC = 'rgba(0,0,0,0.07)';
  // Left: same logic
  const leftW = adjacentLeftHasBorder ? 0 : 1;
  const leftC = 'rgba(0,0,0,0.07)';

  return (
    <div
      data-row={row}
      data-col={col}
      {...(__TEST_ATTRS__ ? {
        'data-testid': `cell-${row}-${col}`,
        'data-cell-state': state,
        'data-cell-conflict': hasConflict || undefined,
      } : {})}
      style={{
        width: cellSize,
        height: cellSize,
        backgroundColor,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        cursor: 'pointer',
        userSelect: 'none',
        borderRightWidth: rightW + 'px',
        borderBottomWidth: bottomW + 'px',
        borderTopWidth: topW + 'px',
        borderLeftWidth: leftW + 'px',
        borderRightColor: rightC,
        borderBottomColor: bottomC,
        borderTopColor: topC,
        borderLeftColor: leftC,
        borderStyle: 'solid',
        boxSizing: 'border-box',
      }}
      onMouseDown={handleMouseDown}
      onMouseUp={handleMouseUp}
      onMouseEnter={handleMouseEnter}
      onTouchStart={handleTouchStart}
    >
      {state === 'marked' && (
        <div
          className={animationClass}
          style={{
            color: hasConflict
              ? 'var(--color-destructive)'
              : 'var(--color-ink)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <MarkerComponent size={markerSize} regionIndex={regionIndex} />
        </div>
      )}
      {state === 'excluded' && (
        <div
          style={{
            color: 'var(--color-ink)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <ExclusionMarkComponent size={markerSize} />
        </div>
      )}
    </div>
  );
}
