import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '@shared/components/PageShell';
import { MODES } from '@engine/types';
import type { Mode } from '@engine/types';
import type {
  ComboStatus,
  ConfigBody,
  ConfigCreateRequest,
} from '@shared/types/admin';
import { useAdminPool } from '@features/admin/hooks/useAdminPool';
import { useReplenish } from '@features/admin/hooks/useReplenish';
import { useUpdateConfig } from '@features/admin/hooks/useUpdateConfig';
import { useCreateConfig } from '@features/admin/hooks/useCreateConfig';

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

const cancelButtonStyle: React.CSSProperties = {
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
};

const saveButtonStyle: React.CSSProperties = {
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
};

/** Format a combo label like "5x5 Standard" or "7x7 Double Queens". */
function comboLabel(size: number, mode: Mode): string {
  const modeLabel = mode === 'double' ? 'Double Queens' : 'Standard';
  return `${size}x${size} ${modeLabel}`;
}

/** Default config values for a new combo. */
function defaultConfig(): ConfigBody {
  return {
    threshold: 3,
    enabled: true,
  };
}

/**
 * ConfigFields renders the three config-body inputs — threshold, enabled,
 * maxAttempts — shared by the edit and create forms. Identity fields
 * (size, mode) live in CreateConfigForm only.
 */
interface ConfigFieldsProps {
  config: ConfigBody;
  onChange: (config: ConfigBody) => void;
}

function ConfigFields({ config, onChange }: ConfigFieldsProps) {
  return (
    <>
      <label style={labelStyle}>
        Threshold
        <input
          type="number"
          min={1}
          value={config.threshold}
          onChange={(e) =>
            onChange({ ...config, threshold: Number(e.target.value) })
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
          onChange={(e) => onChange({ ...config, enabled: e.target.checked })}
          aria-label="Enabled"
          style={{ width: '20px', height: '20px' }}
        />
      </label>

      <label style={labelStyle}>
        Max attempts (0 = default)
        <input
          type="number"
          min={0}
          value={config.maxAttempts ?? 0}
          onChange={(e) =>
            onChange({ ...config, maxAttempts: Number(e.target.value) })
          }
          aria-label="Max attempts"
          style={{ ...inputStyle, width: '80px' }}
        />
      </label>
    </>
  );
}

/** Form chrome — header + cancel/save buttons — shared by edit and create. */
interface FormShellProps {
  title: string;
  saving: boolean;
  onCancel: () => void;
  onSave: () => void;
  children: React.ReactNode;
}

function FormShell({ title, saving, onCancel, onSave, children }: FormShellProps) {
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
        {title}
      </h3>

      {children}

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
          style={cancelButtonStyle}
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onSave}
          disabled={saving}
          data-testid="save-config"
          style={saveButtonStyle}
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  );
}

/** Edit the config of an existing combo. Size and mode are fixed. */
interface EditConfigFormProps {
  config: ConfigBody;
  onConfigChange: (config: ConfigBody) => void;
  onSave: () => void;
  onCancel: () => void;
  saving: boolean;
}

function EditConfigForm({
  config,
  onConfigChange,
  onSave,
  onCancel,
  saving,
}: EditConfigFormProps) {
  return (
    <FormShell
      title="Edit Config"
      saving={saving}
      onCancel={onCancel}
      onSave={onSave}
    >
      <ConfigFields config={config} onChange={onConfigChange} />
    </FormShell>
  );
}

/** Create a new combo. Size and mode are user-chosen up front. */
interface CreateConfigFormProps {
  size: number;
  mode: Mode;
  config: ConfigBody;
  onSizeChange: (size: number) => void;
  onModeChange: (mode: Mode) => void;
  onConfigChange: (config: ConfigBody) => void;
  onSave: () => void;
  onCancel: () => void;
  saving: boolean;
}

function CreateConfigForm({
  size,
  mode,
  config,
  onSizeChange,
  onModeChange,
  onConfigChange,
  onSave,
  onCancel,
  saving,
}: CreateConfigFormProps) {
  return (
    <FormShell
      title="Add New Combo"
      saving={saving}
      onCancel={onCancel}
      onSave={onSave}
    >
      <label style={labelStyle}>
        Size
        <input
          type="number"
          min={3}
          max={15}
          value={size}
          onChange={(e) => onSizeChange(Number(e.target.value))}
          aria-label="Grid size"
          style={{ ...inputStyle, width: '80px' }}
        />
      </label>
      <label style={labelStyle}>
        Mode
        <select
          value={mode}
          // The select renders MODES-sourced options, so e.target.value
          // is always a Mode. Narrowing cast rather than a runtime check.
          onChange={(e) => onModeChange(e.target.value as Mode)}
          aria-label="Game mode"
          style={inputStyle}
        >
          {MODES.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </label>
      <ConfigFields config={config} onChange={onConfigChange} />
    </FormShell>
  );
}

/** Admin page for puzzle pool management. */
export function AdminPage() {
  const navigate = useNavigate();

  const poolQuery = useAdminPool();
  const replenish = useReplenish();
  const updateConfigMutation = useUpdateConfig();
  const createConfigMutation = useCreateConfig();

  const combos = poolQuery.data?.combos ?? [];
  const loading = poolQuery.isPending;
  // Mutation failures land here too — each handler maps its rejection to a
  // user-facing message. The query's own error takes precedence on first load.
  const [mutationError, setMutationError] = useState<string | null>(null);
  const error = poolQuery.isError
    ? poolQuery.error instanceof Error
      ? poolQuery.error.message
      : 'Failed to load pool status'
    : mutationError;

  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // Edit state
  const [editingCombo, setEditingCombo] = useState<ComboStatus | null>(null);
  const [editConfig, setEditConfig] = useState<ConfigBody>(defaultConfig());

  // Create state
  const [showCreate, setShowCreate] = useState(false);
  const [newComboConfig, setNewComboConfig] = useState<ConfigBody>(
    defaultConfig(),
  );
  const [createSize, setCreateSize] = useState(7);
  const [createMode, setCreateMode] = useState<Mode>('standard');

  const saving =
    updateConfigMutation.isPending || createConfigMutation.isPending;

  const handleReplenishAll = () => {
    setStatusMessage(null);
    setMutationError(null);
    replenish.mutate(
      {},
      {
        onSuccess: (result) =>
          setStatusMessage(
            `Triggered ${result.triggered.length} combo(s), skipped ${result.skipped.length}`,
          ),
        onError: (err) =>
          setMutationError(err instanceof Error ? err.message : 'Replenish failed'),
      },
    );
  };

  const handleReplenishCombo = (size: number, mode: Mode) => {
    setStatusMessage(null);
    setMutationError(null);
    replenish.mutate(
      { size, mode },
      {
        onSuccess: () =>
          setStatusMessage(`Replenish triggered for ${comboLabel(size, mode)}`),
        onError: (err) =>
          setMutationError(err instanceof Error ? err.message : 'Replenish failed'),
      },
    );
  };

  const handleEdit = (combo: ComboStatus) => {
    setShowCreate(false);
    setEditingCombo(combo);
    setEditConfig({ ...combo.config });
  };

  const handleSaveEdit = () => {
    if (!editingCombo) return;
    const { size, mode } = editingCombo;
    setMutationError(null);
    updateConfigMutation.mutate(
      { size, mode, config: editConfig },
      {
        onSuccess: () => {
          setStatusMessage(`Config updated for ${comboLabel(size, mode)}`);
          setEditingCombo(null);
        },
        onError: (err) =>
          setMutationError(err instanceof Error ? err.message : 'Save failed'),
      },
    );
  };

  const handleShowCreate = () => {
    setEditingCombo(null);
    setShowCreate(true);
    setNewComboConfig(defaultConfig());
    setCreateSize(7);
    setCreateMode('standard');
  };

  const handleSaveCreate = () => {
    const payload: ConfigCreateRequest = {
      ...newComboConfig,
      size: createSize,
      mode: createMode,
    };
    setMutationError(null);
    createConfigMutation.mutate(payload, {
      onSuccess: () => {
        setStatusMessage(`Created config for ${comboLabel(createSize, createMode)}`);
        setShowCreate(false);
      },
      onError: (err) =>
        setMutationError(err instanceof Error ? err.message : 'Create failed'),
    });
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
          <div
            data-testid="loading"
            role="status"
            style={{
              textAlign: 'center',
              padding: '24px',
              color: 'var(--color-muted)',
            }}
          >
            Loading pool status...
          </div>
        )}

        {poolQuery.isSuccess && (
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
              <div
                style={{
                  padding: '16px',
                  textAlign: 'center',
                  color: 'var(--color-muted)',
                }}
              >
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
                    style={{
                      ...buttonStyle,
                      fontSize: '0.75rem',
                      padding: '6px 10px',
                    }}
                  >
                    Replenish
                  </button>
                  <button
                    type="button"
                    onClick={() => handleEdit(combo)}
                    aria-label={`Edit ${comboLabel(combo.size, combo.mode)}`}
                    data-testid={`edit-${combo.size}-${combo.mode}`}
                    style={{
                      ...buttonStyle,
                      fontSize: '0.75rem',
                      padding: '6px 10px',
                    }}
                  >
                    Edit
                  </button>
                </div>
              ))
            )}
          </div>
        )}

        {editingCombo && (
          <EditConfigForm
            config={editConfig}
            onConfigChange={setEditConfig}
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
          <CreateConfigForm
            size={createSize}
            mode={createMode}
            config={newComboConfig}
            onSizeChange={setCreateSize}
            onModeChange={setCreateMode}
            onConfigChange={setNewComboConfig}
            onSave={handleSaveCreate}
            onCancel={() => setShowCreate(false)}
            saving={saving}
          />
        )}
      </div>
    </PageShell>
  );
}
