import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '../components/common/PageShell';
import type {
  ComboStatus,
  ConfigData,
  CreateConfigRequest,
} from '../services/adminService';
import {
  fetchPoolStatus,
  updateConfig,
  createConfig as createConfigApi,
  triggerReplenish,
} from '../services/adminService';

const PIPELINE_OPTIONS = ['region-first', 'iterative', 'constraint-aware'];
const SOLVER_OPTIONS = ['backtrack', 'propagation'];
const REGIONS_OPTIONS = ['bfs', 'wfc'];
const MODE_OPTIONS = ['standard', 'double'];

const labelStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  gap: '8px',
  fontSize: '0.875rem',
  color: 'var(--color-ink)',
};

const inputStyle: React.CSSProperties = {
  padding: '6px 10px',
  border: '2px solid var(--color-border)',
  borderRadius: '6px',
  backgroundColor: 'var(--color-background)',
  color: 'var(--color-ink)',
  fontSize: '0.875rem',
  fontFamily: 'inherit',
  minHeight: '36px',
};

const buttonStyle: React.CSSProperties = {
  padding: '8px 14px',
  border: '2px solid var(--color-border)',
  borderRadius: '8px',
  backgroundColor: 'var(--color-surface)',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  fontFamily: 'inherit',
  fontSize: '0.8125rem',
  fontWeight: 600,
  minHeight: '44px',
  minWidth: '44px',
};

const accentButtonStyle: React.CSSProperties = {
  ...buttonStyle,
  backgroundColor: 'var(--color-accent)',
  color: '#fff',
};

/** Format a combo label like "5x5 Standard" or "7x7 Double Queens". */
function comboLabel(size: number, mode: string): string {
  const modeLabel = mode === 'double' ? 'Double Queens' : 'Standard';
  return `${size}x${size} ${modeLabel}`;
}

/** Default config values for a new combo. */
function defaultConfig(): ConfigData {
  return {
    pipeline: 'region-first',
    solver: 'backtrack',
    regions: 'bfs',
    regionVariance: 0.3,
    deducible: true,
    concurrency: 2,
    threshold: 3,
    enabled: true,
  };
}

interface ConfigFormProps {
  config: ConfigData;
  isCreate: boolean;
  createSize: number;
  createMode: string;
  onConfigChange: (config: ConfigData) => void;
  onCreateSizeChange: (size: number) => void;
  onCreateModeChange: (mode: string) => void;
  onSave: () => void;
  onCancel: () => void;
  saving: boolean;
}

function ConfigForm({
  config,
  isCreate,
  createSize,
  createMode,
  onConfigChange,
  onCreateSizeChange,
  onCreateModeChange,
  onSave,
  onCancel,
  saving,
}: ConfigFormProps) {
  return (
    <div
      data-testid="config-form"
      style={{
        backgroundColor: 'var(--color-surface)',
        border: '2px solid var(--color-border)',
        borderRadius: '8px',
        padding: '16px',
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        marginTop: '12px',
      }}
    >
      <h3
        style={{
          margin: 0,
          fontSize: '1rem',
          fontWeight: 700,
          color: 'var(--color-ink)',
        }}
      >
        {isCreate ? 'Add New Combo' : 'Edit Config'}
      </h3>

      {isCreate && (
        <>
          <label style={labelStyle}>
            Size
            <input
              type="number"
              min={3}
              max={15}
              value={createSize}
              onChange={(e) => onCreateSizeChange(Number(e.target.value))}
              aria-label="Grid size"
              style={{ ...inputStyle, width: '80px' }}
            />
          </label>
          <label style={labelStyle}>
            Mode
            <select
              value={createMode}
              onChange={(e) => onCreateModeChange(e.target.value)}
              aria-label="Game mode"
              style={inputStyle}
            >
              {MODE_OPTIONS.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </label>
        </>
      )}

      <label style={labelStyle}>
        Pipeline
        <select
          value={config.pipeline}
          onChange={(e) =>
            onConfigChange({ ...config, pipeline: e.target.value })
          }
          aria-label="Pipeline"
          style={inputStyle}
        >
          {PIPELINE_OPTIONS.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </select>
      </label>

      <label style={labelStyle}>
        Solver
        <select
          value={config.solver}
          onChange={(e) =>
            onConfigChange({ ...config, solver: e.target.value })
          }
          aria-label="Solver"
          style={inputStyle}
        >
          {SOLVER_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>

      <label style={labelStyle}>
        Regions
        <select
          value={config.regions}
          onChange={(e) =>
            onConfigChange({ ...config, regions: e.target.value })
          }
          aria-label="Regions"
          style={inputStyle}
        >
          {REGIONS_OPTIONS.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
      </label>

      <label style={labelStyle}>
        Region Variance
        <input
          type="number"
          step={0.1}
          min={0}
          max={1}
          value={config.regionVariance}
          onChange={(e) =>
            onConfigChange({
              ...config,
              regionVariance: Number(e.target.value),
            })
          }
          aria-label="Region variance"
          style={{ ...inputStyle, width: '80px' }}
        />
      </label>

      <label style={labelStyle}>
        Deducible
        <input
          type="checkbox"
          checked={config.deducible}
          onChange={(e) =>
            onConfigChange({ ...config, deducible: e.target.checked })
          }
          aria-label="Deducible"
          style={{ width: '20px', height: '20px' }}
        />
      </label>

      <label style={labelStyle}>
        Concurrency
        <input
          type="number"
          min={1}
          max={8}
          value={config.concurrency}
          onChange={(e) =>
            onConfigChange({ ...config, concurrency: Number(e.target.value) })
          }
          aria-label="Concurrency"
          style={{ ...inputStyle, width: '80px' }}
        />
      </label>

      <label style={labelStyle}>
        Threshold
        <input
          type="number"
          min={1}
          value={config.threshold}
          onChange={(e) =>
            onConfigChange({ ...config, threshold: Number(e.target.value) })
          }
          aria-label="Threshold"
          style={{ ...inputStyle, width: '80px' }}
        />
      </label>

      <label style={labelStyle}>
        Enabled
        <input
          type="checkbox"
          checked={config.enabled}
          onChange={(e) =>
            onConfigChange({ ...config, enabled: e.target.checked })
          }
          aria-label="Enabled"
          style={{ width: '20px', height: '20px' }}
        />
      </label>

      <div
        style={{
          display: 'flex',
          gap: '8px',
          justifyContent: 'flex-end',
          marginTop: '4px',
        }}
      >
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          style={{
            padding: '8px 16px',
            border: '2px solid var(--color-border)',
            borderRadius: '8px',
            backgroundColor: 'var(--color-surface)',
            color: 'var(--color-ink)',
            cursor: 'pointer',
            fontFamily: 'inherit',
            fontSize: '0.875rem',
            fontWeight: 600,
            minHeight: '44px',
          }}
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onSave}
          disabled={saving}
          data-testid="save-config"
          style={{
            padding: '8px 16px',
            border: '2px solid var(--color-border)',
            borderRadius: '8px',
            backgroundColor: 'var(--color-accent)',
            color: '#fff',
            cursor: 'pointer',
            fontFamily: 'inherit',
            fontSize: '0.875rem',
            fontWeight: 600,
            minHeight: '44px',
          }}
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  );
}

/** Admin page for puzzle pool management. */
export function AdminPage() {
  const navigate = useNavigate();
  const [combos, setCombos] = useState<ComboStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // Edit state
  const [editingCombo, setEditingCombo] = useState<ComboStatus | null>(null);
  const [editConfig, setEditConfig] = useState<ConfigData>(defaultConfig());

  // Create state
  const [showCreate, setShowCreate] = useState(false);
  const [newComboConfig, setCreateConfigState] = useState<ConfigData>(
    defaultConfig(),
  );
  const [createSize, setCreateSize] = useState(5);
  const [createMode, setCreateMode] = useState('standard');

  const [saving, setSaving] = useState(false);

  const loadPool = useCallback(async () => {
    try {
      setError(null);
      const data = await fetchPoolStatus();
      setCombos(data.combos);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to load pool status',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadPool();
  }, [loadPool]);

  const handleReplenishAll = async () => {
    try {
      setStatusMessage(null);
      const result = await triggerReplenish();
      setStatusMessage(
        `Triggered ${result.triggered.length} combo(s), skipped ${result.skipped.length}`,
      );
      await loadPool();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Replenish failed',
      );
    }
  };

  const handleReplenishCombo = async (size: number, mode: string) => {
    try {
      setStatusMessage(null);
      await triggerReplenish(size, mode);
      setStatusMessage(`Replenish triggered for ${comboLabel(size, mode)}`);
      await loadPool();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Replenish failed',
      );
    }
  };

  const handleEdit = (combo: ComboStatus) => {
    setShowCreate(false);
    setEditingCombo(combo);
    setEditConfig({ ...combo.config });
  };

  const handleSaveEdit = async () => {
    if (!editingCombo) return;
    setSaving(true);
    try {
      await updateConfig(editingCombo.size, editingCombo.mode, editConfig);
      setStatusMessage(
        `Config updated for ${comboLabel(editingCombo.size, editingCombo.mode)}`,
      );
      setEditingCombo(null);
      await loadPool();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Save failed',
      );
    } finally {
      setSaving(false);
    }
  };

  const handleShowCreate = () => {
    setEditingCombo(null);
    setShowCreate(true);
    setCreateConfigState(defaultConfig());
    setCreateSize(5);
    setCreateMode('standard');
  };

  const handleSaveCreate = async () => {
    setSaving(true);
    try {
      const payload: CreateConfigRequest = {
        ...newComboConfig,
        size: createSize,
        mode: createMode,
      };
      await createConfigApi(payload);
      setStatusMessage(
        `Created config for ${comboLabel(createSize, createMode)}`,
      );
      setShowCreate(false);
      await loadPool();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Create failed',
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <PageShell onBack={() => navigate('/')}>
      <div
        style={{
          width: '100%',
          maxWidth: 600,
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: '1.25rem',
              fontWeight: 700,
              color: 'var(--color-ink)',
            }}
          >
            Pool Management
          </h2>
          <button
            type="button"
            onClick={handleReplenishAll}
            data-testid="replenish-all"
            style={accentButtonStyle}
          >
            Replenish All
          </button>
        </div>

        {statusMessage && (
          <div
            data-testid="status-message"
            role="status"
            style={{
              padding: '8px 12px',
              borderRadius: '6px',
              backgroundColor: 'var(--color-surface)',
              border: '1px solid var(--color-success)',
              color: 'var(--color-success)',
              fontSize: '0.875rem',
            }}
          >
            {statusMessage}
          </div>
        )}

        {error && (
          <div
            data-testid="error-message"
            role="alert"
            style={{
              padding: '8px 12px',
              borderRadius: '6px',
              backgroundColor: 'var(--color-surface)',
              border: '1px solid var(--color-destructive)',
              color: 'var(--color-destructive)',
              fontSize: '0.875rem',
            }}
          >
            {error}
          </div>
        )}

        {loading && (
          <div data-testid="loading" role="status" style={{ textAlign: 'center', padding: '24px', color: 'var(--color-muted)' }}>
            Loading pool status...
          </div>
        )}

        {!loading && !error && (
          <div
            data-testid="pool-table"
            style={{
              backgroundColor: 'var(--color-surface)',
              border: '2px solid var(--color-border)',
              borderRadius: '8px',
              overflow: 'hidden',
            }}
          >
            {combos.length === 0 ? (
              <div style={{ padding: '16px', textAlign: 'center', color: 'var(--color-muted)' }}>
                No combos configured.
              </div>
            ) : (
              combos.map((combo) => (
                <div
                  key={`${combo.size}-${combo.mode}`}
                  data-testid={`combo-row-${combo.size}-${combo.mode}`}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '12px',
                    padding: '12px 16px',
                    borderBottom: '1px solid var(--color-border)',
                    flexWrap: 'wrap',
                  }}
                >
                  <span
                    aria-label={combo.config.enabled ? 'Enabled' : 'Disabled'}
                    style={{
                      width: '10px',
                      height: '10px',
                      borderRadius: '50%',
                      backgroundColor: combo.config.enabled
                        ? 'var(--color-success)'
                        : 'var(--color-muted)',
                      flexShrink: 0,
                    }}
                  />

                  <span
                    style={{
                      fontWeight: 600,
                      fontSize: '0.9375rem',
                      flex: 1,
                      minWidth: '120px',
                    }}
                  >
                    {comboLabel(combo.size, combo.mode)}
                  </span>

                  <span
                    style={{
                      fontSize: '0.875rem',
                      color: 'var(--color-muted)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {combo.readyCount} / {combo.config.threshold}
                  </span>

                  <button
                    type="button"
                    onClick={() =>
                      handleReplenishCombo(combo.size, combo.mode)
                    }
                    aria-label={`Replenish ${comboLabel(combo.size, combo.mode)}`}
                    style={{ ...buttonStyle, fontSize: '0.75rem', padding: '6px 10px' }}
                  >
                    Replenish
                  </button>
                  <button
                    type="button"
                    onClick={() => handleEdit(combo)}
                    aria-label={`Edit ${comboLabel(combo.size, combo.mode)}`}
                    data-testid={`edit-${combo.size}-${combo.mode}`}
                    style={{ ...buttonStyle, fontSize: '0.75rem', padding: '6px 10px' }}
                  >
                    Edit
                  </button>
                </div>
              ))
            )}
          </div>
        )}

        {editingCombo && (
          <ConfigForm
            config={editConfig}
            isCreate={false}
            createSize={0}
            createMode=""
            onConfigChange={setEditConfig}
            onCreateSizeChange={() => {}}
            onCreateModeChange={() => {}}
            onSave={handleSaveEdit}
            onCancel={() => setEditingCombo(null)}
            saving={saving}
          />
        )}

        {!loading && !showCreate && (
          <button
            type="button"
            onClick={handleShowCreate}
            data-testid="add-combo"
            style={{ ...buttonStyle, alignSelf: 'flex-start' }}
          >
            Add Combo
          </button>
        )}

        {showCreate && (
          <ConfigForm
            config={newComboConfig}
            isCreate={true}
            createSize={createSize}
            createMode={createMode}
            onConfigChange={setCreateConfigState}
            onCreateSizeChange={setCreateSize}
            onCreateModeChange={setCreateMode}
            onSave={handleSaveCreate}
            onCancel={() => setShowCreate(false)}
            saving={saving}
          />
        )}
      </div>
    </PageShell>
  );
}

