import { useState, type CSSProperties } from 'react';
import { MODES } from '@reign/core/engine';
import type { Mode } from '@reign/core/engine';
import type {
  PackView,
  PackCreateRequest,
  PackUpdateRequest,
} from '@shared/types/pack';
import { usePackCandidates } from '@features/admin/hooks/usePackCandidates';
import { useCreatePack } from '@features/admin/hooks/useCreatePack';
import { useUpdatePack } from '@features/admin/hooks/useUpdatePack';
import { useDeletePack } from '@features/admin/hooks/useDeletePack';
import { PackCandidatePreview } from '@features/admin/components/PackCandidatePreview';
import { addId, removeId, moveUp, moveDown } from '@features/admin/utils/reorder';

const SIZES = [5, 6, 7, 8, 9, 10] as const;

const panelStyle: CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: '16px',
  display: 'flex',
  flexDirection: 'column',
  gap: '12px',
  marginTop: '12px',
};

const labelStyle: CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  gap: '8px',
  fontSize: '0.875rem',
  color: 'var(--color-ink)',
};

const inputStyle: CSSProperties = {
  padding: '6px 10px',
  border: '2px solid var(--color-border)',
  borderRadius: '6px',
  backgroundColor: 'var(--color-background)',
  color: 'var(--color-ink)',
  fontSize: '0.875rem',
  fontFamily: 'inherit',
  minHeight: '36px',
};

const buttonStyle: CSSProperties = {
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

const accentButtonStyle: CSSProperties = {
  ...buttonStyle,
  backgroundColor: 'var(--color-accent)',
  color: '#fff',
};

const destructiveButtonStyle: CSSProperties = {
  ...buttonStyle,
  borderColor: 'var(--color-destructive)',
  color: 'var(--color-destructive)',
};

const smallButtonStyle: CSSProperties = {
  ...buttonStyle,
  fontSize: '0.75rem',
  padding: '4px 8px',
  minHeight: '36px',
  minWidth: '36px',
};

const headingStyle: CSSProperties = {
  margin: 0,
  fontSize: '1rem',
  fontWeight: 700,
  color: 'var(--color-ink)',
};

function comboLabel(size: number, mode: Mode): string {
  return `${size}x${size} ${mode === 'double' ? 'Double Queens' : 'Standard'}`;
}

interface PackAssemblyProps {
  /** When editing, the existing pack; when creating, undefined. */
  pack?: PackView;
  onClose: () => void;
}

/**
 * Create / edit assembly surface. For a new pack the admin picks slug +
 * name + size + mode (size/mode immutable after create); for an existing
 * pack those identity fields are read-only and a publish toggle appears.
 *
 * The candidate panel lists verdict-up puzzles for the combo as visual
 * preview cards; the admin adds them to an ordered list, reorders via
 * up/down, and removes. Save calls POST (create) or PUT (edit). Backend
 * errors (409 duplicate slug, 422 invalid/non-verdict puzzle or empty
 * publish, 400 validation) surface verbatim from the ApiError message.
 */
export function PackAssembly({ pack, onClose }: PackAssemblyProps) {
  const isEdit = pack !== undefined;

  const [name, setName] = useState(pack?.name ?? '');
  const [slug, setSlug] = useState(pack?.slug ?? '');
  const [size, setSize] = useState<number>(pack?.size ?? 7);
  const [mode, setMode] = useState<Mode>(pack?.mode ?? 'standard');
  const [published, setPublished] = useState(pack?.published ?? false);
  const [puzzleIds, setPuzzleIds] = useState<string[]>(pack?.puzzleIds ?? []);
  const [error, setError] = useState<string | null>(null);

  const candidatesQuery = usePackCandidates(size, mode);
  const createPack = useCreatePack();
  const updatePack = useUpdatePack();
  const deletePack = useDeletePack();

  const saving = createPack.isPending || updatePack.isPending;
  const candidates = candidatesQuery.data ?? [];
  const available = candidates.filter((c) => !puzzleIds.includes(c.puzzleId));

  const handleSave = () => {
    setError(null);
    if (isEdit) {
      const data: PackUpdateRequest = { name, puzzleIds, published };
      updatePack.mutate(
        { slug: pack.slug, data },
        {
          onSuccess: onClose,
          onError: (err) =>
            setError(err instanceof Error ? err.message : 'Save failed'),
        },
      );
      return;
    }
    const data: PackCreateRequest = {
      slug,
      name,
      size,
      mode,
      puzzleIds,
      published: false,
    };
    createPack.mutate(data, {
      onSuccess: onClose,
      onError: (err) =>
        setError(err instanceof Error ? err.message : 'Create failed'),
    });
  };

  const handleDelete = () => {
    if (!isEdit) return;
    setError(null);
    deletePack.mutate(pack.slug, {
      onSuccess: onClose,
      onError: (err) =>
        setError(err instanceof Error ? err.message : 'Delete failed'),
    });
  };

  return (
    <div style={panelStyle} data-testid="pack-assembly">
      <h3 style={headingStyle}>{isEdit ? `Edit ${pack.name}` : 'New Pack'}</h3>

      <label style={labelStyle}>
        Name
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-label="Pack name"
          style={{ ...inputStyle, flex: 1, minWidth: 0 }}
        />
      </label>

      {isEdit ? (
        <div style={labelStyle}>
          <span>Slug / Combo</span>
          <span style={{ color: 'var(--color-muted)' }}>
            {pack.slug} · {comboLabel(pack.size, pack.mode)}
          </span>
        </div>
      ) : (
        <>
          <label style={labelStyle}>
            Slug
            <input
              type="text"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              aria-label="Pack slug"
              placeholder="my-pack"
              style={{ ...inputStyle, flex: 1, minWidth: 0 }}
            />
          </label>
          <label style={labelStyle}>
            Size
            <select
              value={size}
              onChange={(e) => setSize(Number(e.target.value))}
              aria-label="Grid size"
              style={inputStyle}
            >
              {SIZES.map((s) => (
                <option key={s} value={s}>
                  {s}x{s}
                </option>
              ))}
            </select>
          </label>
          <label style={labelStyle}>
            Mode
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as Mode)}
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
        </>
      )}

      {isEdit && (
        <label style={labelStyle}>
          Published
          <input
            type="checkbox"
            checked={published}
            onChange={(e) => setPublished(e.target.checked)}
            aria-label="Published"
            style={{ width: '20px', height: '20px' }}
          />
        </label>
      )}

      {error && (
        <div
          data-testid="assembly-error"
          role="alert"
          style={{
            padding: '8px 12px',
            borderRadius: '6px',
            backgroundColor: 'var(--color-destructive-bg)',
            color: 'var(--color-destructive)',
            fontSize: '0.875rem',
          }}
        >
          {error}
        </div>
      )}

      {/* Ordered list of selected puzzles */}
      <h4 style={{ ...headingStyle, fontSize: '0.9375rem' }}>
        Pack puzzles ({puzzleIds.length})
      </h4>
      {puzzleIds.length === 0 ? (
        <div style={{ color: 'var(--color-muted)', fontSize: '0.875rem' }}>
          No puzzles added yet.
        </div>
      ) : (
        <ol
          data-testid="ordered-list"
          style={{
            listStyle: 'none',
            margin: 0,
            padding: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: '6px',
          }}
        >
          {puzzleIds.map((id, index) => (
            <li
              key={id}
              data-testid={`ordered-item-${id}`}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                padding: '6px 8px',
                border: '1px solid var(--color-border)',
                borderRadius: '6px',
              }}
            >
              <span style={{ color: 'var(--color-muted)', minWidth: 24 }}>
                {index + 1}.
              </span>
              <span
                style={{
                  flex: 1,
                  minWidth: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  fontSize: '0.8125rem',
                }}
              >
                {id}
              </span>
              <button
                type="button"
                onClick={() => setPuzzleIds((ids) => moveUp(ids, index))}
                disabled={index === 0}
                aria-label={`Move ${id} up`}
                style={smallButtonStyle}
              >
                ↑
              </button>
              <button
                type="button"
                onClick={() => setPuzzleIds((ids) => moveDown(ids, index))}
                disabled={index === puzzleIds.length - 1}
                aria-label={`Move ${id} down`}
                style={smallButtonStyle}
              >
                ↓
              </button>
              <button
                type="button"
                onClick={() => setPuzzleIds((ids) => removeId(ids, id))}
                aria-label={`Remove ${id}`}
                style={{ ...smallButtonStyle, color: 'var(--color-destructive)' }}
              >
                ✕
              </button>
            </li>
          ))}
        </ol>
      )}

      {/* Candidate panel */}
      <h4 style={{ ...headingStyle, fontSize: '0.9375rem' }}>
        Candidates — {comboLabel(size, mode)}
      </h4>
      {candidatesQuery.isPending ? (
        <div
          data-testid="candidates-loading"
          role="status"
          style={{ color: 'var(--color-muted)', fontSize: '0.875rem' }}
        >
          Loading candidates...
        </div>
      ) : candidatesQuery.isError ? (
        <div
          data-testid="candidates-error"
          role="alert"
          style={{ color: 'var(--color-destructive)', fontSize: '0.875rem' }}
        >
          Failed to load candidates.
        </div>
      ) : available.length === 0 ? (
        <div
          data-testid="candidates-empty"
          style={{ color: 'var(--color-muted)', fontSize: '0.875rem' }}
        >
          No verdict-up puzzles available for this combo.
        </div>
      ) : (
        <div
          data-testid="candidate-grid"
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: '12px',
          }}
        >
          {available.map((candidate) => (
            <div
              key={candidate.puzzleId}
              style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '6px' }}
            >
              <PackCandidatePreview candidate={candidate} mode={mode} />
              <button
                type="button"
                onClick={() =>
                  setPuzzleIds((ids) => addId(ids, candidate.puzzleId))
                }
                aria-label={`Add ${candidate.puzzleId}`}
                style={smallButtonStyle}
              >
                Add
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Actions */}
      <div
        style={{
          display: 'flex',
          gap: '8px',
          justifyContent: 'flex-end',
          marginTop: '4px',
          flexWrap: 'wrap',
        }}
      >
        {isEdit && (
          <button
            type="button"
            onClick={handleDelete}
            disabled={deletePack.isPending}
            data-testid="delete-pack"
            style={{ ...destructiveButtonStyle, marginRight: 'auto' }}
          >
            {deletePack.isPending ? 'Deleting...' : 'Delete'}
          </button>
        )}
        <button
          type="button"
          onClick={onClose}
          disabled={saving}
          style={buttonStyle}
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving}
          data-testid="save-pack"
          style={accentButtonStyle}
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  );
}
