import { memo, useMemo } from 'react';

interface Segment {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

const STROKE_WIDTH = 2;
const HALF = STROKE_WIDTH / 2;

function computeBorders(regionMap: number[][], gridSize: number, cellSize: number) {
  const segments: Segment[] = [];
  const hEndpoints = new Set<string>();
  const vEndpoints = new Set<string>();

  for (let r = 1; r < gridSize; r++) {
    let segStart: number | null = null;
    for (let c = 0; c < gridSize; c++) {
      const boundary = regionMap[r]![c] !== regionMap[r - 1]![c];
      if (boundary && segStart === null) segStart = c;
      if ((!boundary || c === gridSize - 1) && segStart !== null) {
        const end = boundary ? c + 1 : c;
        const x1 = segStart * cellSize;
        const y = r * cellSize;
        const x2 = end * cellSize;
        segments.push({ x1, y1: y, x2, y2: y });
        hEndpoints.add(`${x1},${y}`);
        hEndpoints.add(`${x2},${y}`);
        segStart = null;
      }
    }
  }

  for (let c = 1; c < gridSize; c++) {
    let segStart: number | null = null;
    for (let r = 0; r < gridSize; r++) {
      const boundary = regionMap[r]![c] !== regionMap[r]![c - 1];
      if (boundary && segStart === null) segStart = r;
      if ((!boundary || r === gridSize - 1) && segStart !== null) {
        const end = boundary ? r + 1 : r;
        const x = c * cellSize;
        const y1 = segStart * cellSize;
        const y2 = end * cellSize;
        segments.push({ x1: x, y1, x2: x, y2 });
        vEndpoints.add(`${x},${y1}`);
        vEndpoints.add(`${x},${y2}`);
        segStart = null;
      }
    }
  }

  // Fill corners where horizontal and vertical segments meet
  const junctions: { x: number; y: number }[] = [];
  for (const key of hEndpoints) {
    if (vEndpoints.has(key)) {
      const [x, y] = key.split(',').map(Number);
      junctions.push({ x: x!, y: y! });
    }
  }

  return { segments, junctions };
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
  const { segments, junctions } = useMemo(
    () => computeBorders(regionMap, gridSize, cellSize),
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
          key={`s${i}`}
          x1={s.x1}
          y1={s.y1}
          x2={s.x2}
          y2={s.y2}
          stroke="var(--color-ink)"
          strokeWidth={STROKE_WIDTH}
        />
      ))}
      {junctions.map((p, i) => (
        <rect
          key={`j${i}`}
          x={p.x - HALF}
          y={p.y - HALF}
          width={STROKE_WIDTH}
          height={STROKE_WIDTH}
          fill="var(--color-ink)"
        />
      ))}
    </svg>
  );
});
