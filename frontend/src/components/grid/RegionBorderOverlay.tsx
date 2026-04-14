import { memo, useMemo } from 'react';

interface Segment {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

function computeSegments(regionMap: number[][], gridSize: number, cellSize: number): Segment[] {
  const segments: Segment[] = [];

  // Horizontal boundaries (between row r-1 and row r)
  for (let r = 1; r < gridSize; r++) {
    let segStart: number | null = null;
    for (let c = 0; c < gridSize; c++) {
      const boundary = regionMap[r]![c] !== regionMap[r - 1]![c];
      if (boundary && segStart === null) {
        segStart = c;
      }
      if ((!boundary || c === gridSize - 1) && segStart !== null) {
        const end = boundary ? c + 1 : c;
        segments.push({ x1: segStart * cellSize, y1: r * cellSize, x2: end * cellSize, y2: r * cellSize });
        segStart = null;
      }
    }
  }

  // Vertical boundaries (between col c-1 and col c)
  for (let c = 1; c < gridSize; c++) {
    let segStart: number | null = null;
    for (let r = 0; r < gridSize; r++) {
      const boundary = regionMap[r]![c] !== regionMap[r]![c - 1];
      if (boundary && segStart === null) {
        segStart = r;
      }
      if ((!boundary || r === gridSize - 1) && segStart !== null) {
        const end = boundary ? r + 1 : r;
        segments.push({ x1: c * cellSize, y1: segStart * cellSize, x2: c * cellSize, y2: end * cellSize });
        segStart = null;
      }
    }
  }

  return segments;
}

/** SVG overlay that draws region boundary lines on top of the grid cells. */
export const RegionBorderOverlay = memo(function RegionBorderOverlay({
  regionMap,
  gridSize,
  cellSize,
}: {
  regionMap: number[][];
  gridSize: number;
  cellSize: number;
}) {
  const segments = useMemo(
    () => computeSegments(regionMap, gridSize, cellSize),
    [regionMap, gridSize, cellSize],
  );

  const totalSize = gridSize * cellSize;

  return (
    <svg
      width={totalSize}
      height={totalSize}
      viewBox={`0 0 ${totalSize} ${totalSize}`}
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        pointerEvents: 'none',
      }}
      aria-hidden="true"
    >
      {segments.map((s, i) => (
        <line
          key={i}
          x1={s.x1}
          y1={s.y1}
          x2={s.x2}
          y2={s.y2}
          stroke="var(--color-ink)"
          strokeWidth={2.5}
          strokeLinecap="round"
        />
      ))}
    </svg>
  );
});
