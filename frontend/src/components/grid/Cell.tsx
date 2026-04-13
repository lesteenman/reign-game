import type { CellState } from '../../engine/types';
import { useTheme } from '../../theme/ThemeContext';

/** Props for the Cell component. */
export interface CellProps {
  row: number;
  col: number;
  state: CellState;
  regionIndex: number;
  hasConflict: boolean;
  cellSize: number;
  borderTop: boolean;
  borderRight: boolean;
  borderBottom: boolean;
  borderLeft: boolean;
  onPointerDown: () => void;
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
  cellSize,
  borderTop,
  borderRight,
  borderBottom,
  borderLeft,
  onPointerDown,
  onDragEnter,
}: CellProps) {
  const theme = useTheme();

  const backgroundColor = hasConflict
    ? 'var(--color-destructive-bg)'
    : `var(--region-${regionIndex}-fill)`;

  const MarkerComponent = theme.marker;
  const ExclusionMarkComponent = theme.exclusionMark;

  const markerSize = cellSize * 0.6;

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    onPointerDown();
  };

  const handleMouseEnter = (e: React.MouseEvent) => {
    if (e.buttons === 1) {
      onDragEnter();
    }
  };

  const handleTouchStart = () => {
    onPointerDown();
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
            color: 'var(--color-muted)',
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
