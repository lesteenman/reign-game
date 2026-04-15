import { useState, useCallback, type CSSProperties } from 'react';
import type { GenerateOptions } from '../../services/puzzleService';

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
  { label: '9\u00D79 Double Queens', size: 9, mode: 'double' },
];

const PIPELINE_OPTIONS = ['region-first', 'iterative', 'constraint-aware'] as const;
const SOLVER_OPTIONS = ['backtrack', 'propagation'] as const;
const REGION_OPTIONS = ['bfs', 'wfc'] as const;

interface VarianceStop {
  label: string;
  value: number;
}

const VARIANCE_STOPS: VarianceStop[] = [
  { label: 'Uniform', value: 0.0 },
  { label: 'Slight', value: 0.25 },
  { label: 'Moderate', value: 0.5 },
  { label: 'High', value: 0.75 },
  { label: 'Wild', value: 1.0 },
];

interface PuzzleSelectorProps {
  onSelect: (options: GenerateOptions) => void;
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

const labelStyle: CSSProperties = {
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 600,
  fontSize: '0.875rem',
  color: 'var(--color-body)',
  marginBottom: '4px',
};

const selectStyle: CSSProperties = {
  padding: '8px 12px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 600,
  fontSize: '0.875rem',
  backgroundColor: 'var(--color-surface)',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  minHeight: '44px',
  width: '100%',
};

const numberInputStyle: CSSProperties = {
  ...selectStyle,
  width: '80px',
};

/** Puzzle size/mode selector with standard presets and an advanced options panel. */
export function PuzzleSelector({ onSelect }: PuzzleSelectorProps) {
  const [selectedPresetIndex, setSelectedPresetIndex] = useState(0);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [useAdvanced, setUseAdvanced] = useState(false);

  // Advanced state
  const [advSizeInput, setAdvSizeInput] = useState('5');
  const [advSizeError, setAdvSizeError] = useState('');
  const [advMode, setAdvMode] = useState<'standard' | 'double'>('standard');
  const [advPipeline, setAdvPipeline] = useState<GenerateOptions['pipeline']>('region-first');
  const [advSolver, setAdvSolver] = useState<GenerateOptions['solver']>('backtrack');
  const [advRegions, setAdvRegions] = useState<GenerateOptions['regions']>('bfs');
  const [advVarianceIndex, setAdvVarianceIndex] = useState(2); // Moderate

  const handlePresetClick = useCallback((index: number) => {
    setSelectedPresetIndex(index);
    setUseAdvanced(false);
  }, []);

  const handleAdvancedChange = useCallback(() => {
    setUseAdvanced(true);
  }, []);

  const handlePlay = useCallback(() => {
    if (useAdvanced) {
      const parsed = Number(advSizeInput);
      if (advSizeInput === '' || isNaN(parsed) || parsed < 3 || parsed > 15 || !Number.isInteger(parsed)) {
        setAdvSizeError('Grid size must be between 3 and 15');
        return;
      }
      const varianceValue = VARIANCE_STOPS[advVarianceIndex]?.value ?? 0.5;
      onSelect({
        size: parsed,
        mode: advMode,
        pipeline: advPipeline,
        solver: advSolver,
        regions: advRegions,
        regionVariance: varianceValue,
      });
    } else {
      const size = PRESETS[selectedPresetIndex]?.size ?? 5;
      const mode = PRESETS[selectedPresetIndex]?.mode ?? 'standard';
      onSelect({ size, mode });
    }
  }, [useAdvanced, advSizeInput, advMode, advPipeline, advSolver, advRegions, advVarianceIndex, selectedPresetIndex, onSelect]);

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
              !useAdvanced && selectedPresetIndex === index
                ? presetButtonSelected
                : presetButtonBase
            }
            aria-pressed={!useAdvanced && selectedPresetIndex === index}
          >
            {preset.label}
          </button>
        ))}
      </div>

      {/* Advanced toggle */}
      <button
        type="button"
        data-testid="advanced-toggle"
        onClick={() => setAdvancedOpen((prev) => !prev)}
        aria-expanded={advancedOpen}
        style={{
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          fontWeight: 600,
          fontSize: '0.875rem',
          color: 'var(--color-muted)',
          padding: '4px 8px',
          minHeight: '44px',
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
        }}
      >
        Advanced {advancedOpen ? '\u25B2' : '\u25BC'}
      </button>

      {/* Advanced section */}
      {advancedOpen && (
        <div
          data-testid="advanced-section"
          style={{
            width: '100%',
            display: 'flex',
            flexDirection: 'column',
            gap: '12px',
            padding: '16px',
            border: '2px solid var(--color-border)',
            borderRadius: 'var(--radius)',
            backgroundColor: 'var(--color-surface)',
          }}
        >
          {/* Grid size */}
          <div>
            <label htmlFor="adv-size" style={labelStyle}>Grid Size</label>
            <input
              id="adv-size"
              data-testid="adv-size"
              type="number"
              value={advSizeInput}
              onChange={(e) => {
                setAdvSizeInput(e.target.value);
                setAdvSizeError('');
                handleAdvancedChange();
              }}
              style={numberInputStyle}
            />
            {advSizeError && (
              <p
                data-testid="adv-size-error"
                style={{
                  color: 'var(--color-destructive)',
                  fontSize: '0.75rem',
                  fontFamily: '"Nunito Sans", system-ui, sans-serif',
                  fontWeight: 600,
                  marginTop: '4px',
                  marginBottom: 0,
                }}
              >
                {advSizeError}
              </p>
            )}
          </div>

          {/* Mode */}
          <div>
            <label htmlFor="adv-mode" style={labelStyle}>Mode</label>
            <select
              id="adv-mode"
              data-testid="adv-mode"
              value={advMode}
              onChange={(e) => {
                setAdvMode(e.target.value as 'standard' | 'double');
                handleAdvancedChange();
              }}
              style={selectStyle}
            >
              <option value="standard">Standard</option>
              <option value="double">Double</option>
            </select>
          </div>

          {/* Pipeline */}
          <div>
            <label htmlFor="adv-pipeline" style={labelStyle}>Pipeline</label>
            <select
              id="adv-pipeline"
              data-testid="adv-pipeline"
              value={advPipeline}
              onChange={(e) => {
                setAdvPipeline(e.target.value as GenerateOptions['pipeline']);
                handleAdvancedChange();
              }}
              style={selectStyle}
            >
              {PIPELINE_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {opt.split('-').map((w) => (w[0] ?? '').toUpperCase() + w.slice(1)).join(' ')}
                </option>
              ))}
            </select>
          </div>

          {/* Solver */}
          <div>
            <label htmlFor="adv-solver" style={labelStyle}>Solver</label>
            <select
              id="adv-solver"
              data-testid="adv-solver"
              value={advSolver}
              onChange={(e) => {
                setAdvSolver(e.target.value as GenerateOptions['solver']);
                handleAdvancedChange();
              }}
              style={selectStyle}
            >
              {SOLVER_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {(opt[0] ?? '').toUpperCase() + opt.slice(1)}
                </option>
              ))}
            </select>
          </div>

          {/* Region Generator */}
          <div>
            <label htmlFor="adv-regions" style={labelStyle}>Region Generator</label>
            <select
              id="adv-regions"
              data-testid="adv-regions"
              value={advRegions}
              onChange={(e) => {
                setAdvRegions(e.target.value as GenerateOptions['regions']);
                handleAdvancedChange();
              }}
              style={selectStyle}
            >
              {REGION_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {opt.toUpperCase()}
                </option>
              ))}
            </select>
          </div>

          {/* Region Variance */}
          <div>
            <label htmlFor="adv-variance" style={labelStyle}>
              Region Variance: {VARIANCE_STOPS[advVarianceIndex]?.label ?? 'Moderate'}
            </label>
            <input
              id="adv-variance"
              data-testid="adv-variance"
              type="range"
              min={0}
              max={4}
              step={1}
              value={advVarianceIndex}
              onChange={(e) => {
                setAdvVarianceIndex(Number(e.target.value));
                handleAdvancedChange();
              }}
              style={{
                width: '100%',
                cursor: 'pointer',
                accentColor: 'var(--color-accent)',
              }}
              aria-valuetext={VARIANCE_STOPS[advVarianceIndex]?.label ?? 'Moderate'}
            />
            <div
              data-testid="variance-labels"
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                fontSize: '0.75rem',
                color: 'var(--color-muted)',
                fontFamily: '"Nunito Sans", system-ui, sans-serif',
                marginTop: '4px',
              }}
            >
              {VARIANCE_STOPS.map((stop) => (
                <span key={stop.label}>{stop.label}</span>
              ))}
            </div>
          </div>
        </div>
      )}

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

export { PRESETS, VARIANCE_STOPS };
export type { Preset, VarianceStop };
