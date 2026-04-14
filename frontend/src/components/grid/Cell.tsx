import { useEffect, useRef } from 'react';
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
  /** Which sides show the subtle internal cell border (not on edge or region boundary). */
  showBorderTop: boolean;
  showBorderRight: boolean;
  showBorderBottom: boolean;
  showBorderLeft: boolean;
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
  showBorderTop,
  showBorderRight,
  showBorderBottom,
  showBorderLeft,
  onPointerDown,
  onPointerUp,
  onDragEnter,
}: CellProps) {
  const theme = useTheme();
  const touchedRef = useRef(false);
  const touchTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => {
    return () => { clearTimeout(touchTimerRef.current); };
  }, []);

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

  const handleTouchStart = (e: React.TouchEvent) => {
    // preventDefault stops the browser from synthesising mousedown/mouseup
    // after the touch sequence. Without this, iOS WebKit (including Chrome)
    // fires synthesised mouse events after a 300-350ms delay that races past
    // the touchedRef guard and causes a double state-cycle.
    e.preventDefault();
    touchedRef.current = true;
    onPointerDown();
    clearTimeout(touchTimerRef.current);
    touchTimerRef.current = setTimeout(() => { touchedRef.current = false; }, 300);
  };

  let animationClass = '';
  if (state === 'marked' && hasConflict) {
    animationClass = theme.animations.conflict;
  } else if (state === 'marked') {
    animationClass = theme.animations.placement;
  }

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
        borderTop: showBorderTop ? '0.5px solid rgba(0,0,0,0.07)' : 'none',
        borderRight: showBorderRight ? '0.5px solid rgba(0,0,0,0.07)' : 'none',
        borderBottom: showBorderBottom ? '0.5px solid rgba(0,0,0,0.07)' : 'none',
        borderLeft: showBorderLeft ? '0.5px solid rgba(0,0,0,0.07)' : 'none',
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
