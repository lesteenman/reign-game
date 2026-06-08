import { useState, useCallback, type CSSProperties } from 'react';
import type { Mode } from '@reign/core/engine';
import type { ModeEntry } from '@shared/types/modes';

/** Selection from the puzzle selector: size and mode. */
export interface PuzzleSelection {
  size: number;
  mode: Mode;
}

/** Label for a combo, e.g. "5\u00d75 Standard" / "9\u00d79 Double Queens". */
function presetLabel(entry: ModeEntry): string {
  const modeLabel = entry.mode === 'double' ? 'Double Queens' : 'Standard';
  return `${entry.size}\u00D7${entry.size} ${modeLabel}`;
}

interface PuzzleSelectorProps {
  /**
   * The enabled combos to render as buttons. Sourced from the public
   * GET /api/config/modes endpoint by the parent page. An empty array
   * is treated as "pool not configured" — the component renders a
   * friendly fallback message and no Play button.
   */
  modes: ModeEntry[];
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

/**
 * Puzzle size/mode selector driven by the enabled-combo list from the
 * backend. Renders one button per combo; first combo is selected by
 * default; the Play button emits the current selection.
 */
export function PuzzleSelector({ modes, onSelect }: PuzzleSelectorProps) {
  const [selectedIndex, setSelectedIndex] = useState(0);

  const handlePresetClick = useCallback((index: number) => {
    setSelectedIndex(index);
  }, []);

  const handlePlay = useCallback(() => {
    const entry = modes[selectedIndex];
    if (!entry) return;
    onSelect({ size: entry.size, mode: entry.mode });
  }, [modes, selectedIndex, onSelect]);

  if (modes.length === 0) {
    return (
      <div
        data-testid="puzzle-selector-empty"
        role="status"
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: '8px',
          padding: '24px',
          fontWeight: 600,
          textAlign: 'center',
          maxWidth: '400px',
        }}
      >
        <p style={{ margin: 0 }}>No puzzles available right now.</p>
        <p style={{ margin: 0, fontWeight: 400, color: 'var(--color-muted)' }}>
          Try again in a moment.
        </p>
      </div>
    );
  }

  // Clamp selectedIndex so a shorter list after re-mount still picks a
  // valid entry. `modes.length >= 1` past the empty-state branch above.
  const safeIndex = Math.min(selectedIndex, modes.length - 1);

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
      <div
        data-testid="puzzle-presets"
        style={{
          display: 'grid',
          gridTemplateColumns: modes.length === 1 ? '1fr' : '1fr 1fr',
          gap: '8px',
          width: '100%',
        }}
      >
        {modes.map((entry, index) => (
          <button
            key={`${entry.size}-${entry.mode}`}
            type="button"
            data-testid={`preset-${index}`}
            onClick={() => handlePresetClick(index)}
            style={
              safeIndex === index ? presetButtonSelected : presetButtonBase
            }
            aria-pressed={safeIndex === index}
          >
            {presetLabel(entry)}
          </button>
        ))}
      </div>

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
