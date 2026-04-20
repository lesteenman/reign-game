import { useState, useCallback, type CSSProperties } from 'react';

/** Selection from the puzzle selector: size and mode only. */
export interface PuzzleSelection {
  size: number;
  mode: 'standard' | 'double';
}

/** Preset puzzle configuration. */
interface Preset {
  label: string;
  size: number;
  mode: 'standard' | 'double';
}

const PRESETS: Preset[] = [
  { label: '5\u00D75 Standard', size: 5, mode: 'standard' },
  { label: '7\u00D77 Standard', size: 7, mode: 'standard' },
  { label: '9\u00D79 Standard', size: 9, mode: 'standard' },
  // Double Queens re-enabled (KI-007 closed by Phase 5 generator rework).
  // Only 9x9 is presetted: smaller N at k=2 is infeasible (R-063 data:
  // N<8 k=2 has 0 solutions; N=8 k=2 has only 2). R-066's solver-guided
  // grower + R-066c's mutator-plateau acceptance hit 100% generation
  // success at (N=9, k=2).
  { label: '9\u00D79 Double Queens', size: 9, mode: 'double' },
];

interface PuzzleSelectorProps {
  onSelect: (selection: PuzzleSelection) => void;
}

const presetButtonBase: CSSProperties = {
  padding: '12px 20px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 700,
  fontSize: '0.875rem',
  cursor: 'pointer',
  transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
  backgroundColor: 'var(--color-surface)',
  color: 'var(--color-ink)',
  boxShadow: '0 3px 0 var(--color-ink)',
  minWidth: '44px',
  minHeight: '44px',
};

const presetButtonSelected: CSSProperties = {
  ...presetButtonBase,
  backgroundColor: 'var(--color-accent)',
  color: 'var(--color-on-accent)',
  boxShadow: '0 3px 0 var(--color-accent-shadow)',
};

/** Puzzle size/mode selector with standard presets. */
export function PuzzleSelector({ onSelect }: PuzzleSelectorProps) {
  const [selectedPresetIndex, setSelectedPresetIndex] = useState(0);

  const handlePresetClick = useCallback((index: number) => {
    setSelectedPresetIndex(index);
  }, []);

  const handlePlay = useCallback(() => {
    const preset = PRESETS[selectedPresetIndex];
    const size = preset?.size ?? 5;
    const mode = preset?.mode ?? 'standard';
    onSelect({ size, mode });
  }, [selectedPresetIndex, onSelect]);

  return (
    <div
      data-testid="puzzle-selector"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '16px',
        width: '100%',
        maxWidth: '400px',
      }}
    >
      {/* Preset buttons */}
      <div
        data-testid="preset-buttons"
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: '8px',
          width: '100%',
        }}
      >
        {PRESETS.map((preset, index) => (
          <button
            key={preset.label}
            type="button"
            data-testid={`preset-${index}`}
            onClick={() => handlePresetClick(index)}
            style={
              selectedPresetIndex === index
                ? presetButtonSelected
                : presetButtonBase
            }
            aria-pressed={selectedPresetIndex === index}
          >
            {preset.label}
          </button>
        ))}
      </div>

      {/* Play button */}
      <button
        type="button"
        data-testid="play-button"
        onClick={handlePlay}
        style={{
          padding: '12px 32px',
          border: '2px solid var(--color-ink)',
          borderRadius: 'var(--radius)',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          fontWeight: 700,
          fontSize: '1rem',
          cursor: 'pointer',
          transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
          backgroundColor: 'var(--color-accent)',
          color: 'var(--color-on-accent)',
          boxShadow: '0 3px 0 var(--color-accent-shadow)',
          minHeight: '44px',
        }}
      >
        Play
      </button>
    </div>
  );
}

export { PRESETS };
export type { Preset };
