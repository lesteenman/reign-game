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
  borderTop: boolean;
  borderRight: boolean;
  borderBottom: boolean;
  borderLeft: boolean;
  onPointerDown: () => void;
  onPointerUp: () => void;
  onDragEnter: () => void;
}

function borderWidth(isRegionBoundary: boolean): string {
  return isRegionBoundary ? '2.5px' : '1px';
}

function borderColor(isRegionBoundary: boolean): string {
  return isRegionBoundary
    ? 'var(--color-ink)'
    : 'rgba(0,0,0,0.07)';
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
  borderTop,
  borderRight,
  borderBottom,
  borderLeft,
  onPointerDown,
  onPointerUp,
  onDragEnter,
}: CellProps) {
  const theme = useTheme();
  // Track whether the last interaction was touch to prevent
  // the synthesized mousedown from double-firing.
  const touchedRef = useRef(false);

  let backgroundColor = `var(--region-${regionIndex}-fill)`;
  if (hasConflict) {
    backgroundColor = 'var(--color-destructive-bg)';
  } else if (isDragHighlighted) {
    // Subtle brightness shift to indicate drag path
    backgroundColor = `color-mix(in srgb, var(--region-${regionIndex}-fill) 70%, var(--color-ink) 30%)`;
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

  // Determine animation class for marked cells
  let animationClass = '';
  if (state === 'marked' && hasConflict) {
    animationClass = theme.animations.conflict;
  } else if (state === 'marked') {
    animationClass = theme.animations.placement;
  }

  return (
    <div
      data-testid={`cell-${row}-${col}`}
      data-row={row}
      data-col={col}
      style={{
        width: cellSize,
        height: cellSize,
        backgroundColor,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        cursor: 'pointer',
        userSelect: 'none',
        borderTopWidth: borderWidth(borderTop),
        borderRightWidth: borderWidth(borderRight),
        borderBottomWidth: borderWidth(borderBottom),
        borderLeftWidth: borderWidth(borderLeft),
        borderTopColor: borderColor(borderTop),
        borderRightColor: borderColor(borderRight),
        borderBottomColor: borderColor(borderBottom),
        borderLeftColor: borderColor(borderLeft),
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
            color: 'var(--color-body)',
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
