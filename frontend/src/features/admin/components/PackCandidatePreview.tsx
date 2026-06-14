import type { CSSProperties } from 'react';
import { Grid } from '@shared/game/components/grid/Grid';
import type { PuzzleData, CellState } from '@reign/core/engine';
import type { PackCandidate } from '@shared/types/pack';
import type { Mode } from '@reign/core/engine';

/** Build an all-empty cell matrix for a read-only preview render. */
function emptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from<CellState>({ length: size }).fill('empty'),
  );
}

const containerStyle: CSSProperties = {
  // The shared Grid measures its parent's width to size cells, so the
  // fixed-width wrapper keeps every preview to a compact, uniform size.
  width: 160,
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
};

const metaStyle: CSSProperties = {
  marginTop: '6px',
  fontSize: '0.75rem',
  color: 'var(--color-muted)',
  textAlign: 'center',
};

interface PackCandidatePreviewProps {
  candidate: PackCandidate;
  mode: Mode;
}

/**
 * A small read-only render of a candidate puzzle's regionMap using the
 * shared Grid, plus its difficulty/tier metadata. The Grid renders with
 * empty cells, `isSolved` (which no-ops the interaction handlers), and
 * no-op pointer callbacks so it is purely visual.
 */
export function PackCandidatePreview({
  candidate,
  mode,
}: PackCandidatePreviewProps) {
  const size = candidate.regionMap.length;
  const puzzle: PuzzleData = {
    puzzleId: candidate.puzzleId,
    gridSize: size,
    mode,
    regionMap: candidate.regionMap,
  };

  return (
    <div style={containerStyle} data-testid={`candidate-preview-${candidate.puzzleId}`}>
      <Grid
        puzzle={puzzle}
        cells={emptyCells(size)}
        conflicts={[]}
        isSolved
        draggedCells={new Set()}
        onPointerDown={() => {}}
        onPointerUp={() => {}}
        onDragEnter={() => {}}
      />
      <span style={metaStyle}>
        {`Difficulty ${candidate.metadata.difficulty} · Tier ${candidate.metadata.maxTier}`}
      </span>
    </div>
  );
}
