/** SVG overlay that draws region boundary lines on top of the grid cells. */
export function RegionBorderOverlay({
  regionMap,
  gridSize,
  cellSize,
}: {
  regionMap: number[][];
  gridSize: number;
  cellSize: number;
}) {
  const totalSize = gridSize * cellSize;
  const strokeWidth = 2.5;

  // Collect all boundary line segments.
  // Each segment is a horizontal or vertical line between two cells
  // of different regions. We merge adjacent segments into longer lines
  // to reduce SVG element count.
  const hLines: { y: number; x1: number; x2: number }[] = [];
  const vLines: { x: number; y1: number; y2: number }[] = [];

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
        hLines.push({ y: r * cellSize, x1: segStart * cellSize, x2: end * cellSize });
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
        vLines.push({ x: c * cellSize, y1: segStart * cellSize, y2: end * cellSize });
        segStart = null;
      }
    }
  }

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
      {hLines.map((l, i) => (
        <line
          key={`h-${i}`}
          x1={l.x1}
          y1={l.y}
          x2={l.x2}
          y2={l.y}
          stroke="var(--color-ink)"
          strokeWidth={strokeWidth}
        />
      ))}
      {vLines.map((l, i) => (
        <line
          key={`v-${i}`}
          x1={l.x}
          y1={l.y1}
          x2={l.x}
          y2={l.y2}
          stroke="var(--color-ink)"
          strokeWidth={strokeWidth}
        />
      ))}
    </svg>
  );
}
