import { useState, type CSSProperties } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '@shared/components/PageShell';
import type { Mode } from '@reign/core/engine';
import type { PackView } from '@shared/types/pack';
import { useAdminPacks } from '@features/admin/hooks/useAdminPacks';
import { PackAssembly } from '@features/admin/screens/PackAssembly';

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

function comboLabel(size: number, mode: Mode): string {
  return `${size}x${size} ${mode === 'double' ? 'Double Queens' : 'Standard'}`;
}

type Editing = { kind: 'create' } | { kind: 'edit'; pack: PackView } | null;

/** Admin page for pack assembly — list, create, edit, and delete packs. */
export function AdminPacksPage() {
  const navigate = useNavigate();
  const packsQuery = useAdminPacks();
  const [editing, setEditing] = useState<Editing>(null);

  const packs = packsQuery.data ?? [];

  return (
    <PageShell onBack={() => navigate('/admin')}>
      <div
        style={{
          width: '100%',
          maxWidth: 640,
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
            Packs
          </h2>
          <button
            type="button"
            onClick={() => setEditing({ kind: 'create' })}
            data-testid="create-pack"
            style={accentButtonStyle}
          >
            Create Pack
          </button>
        </div>

        {packsQuery.isError && (
          <div
            data-testid="error-message"
            role="alert"
            style={{
              padding: '8px 12px',
              borderRadius: '6px',
              backgroundColor: 'var(--color-destructive-bg)',
              color: 'var(--color-destructive)',
              fontSize: '0.875rem',
            }}
          >
            {packsQuery.error instanceof Error
              ? packsQuery.error.message
              : 'Failed to load packs'}
          </div>
        )}

        {packsQuery.isPending && (
          <div
            data-testid="loading"
            role="status"
            style={{
              textAlign: 'center',
              padding: '24px',
              color: 'var(--color-muted)',
            }}
          >
            Loading packs...
          </div>
        )}

        {packsQuery.isSuccess && (
          <div
            data-testid="pack-table"
            style={{
              backgroundColor: 'var(--color-surface)',
              border: '2px solid var(--color-border)',
              borderRadius: 'var(--radius)',
              overflow: 'hidden',
            }}
          >
            {packs.length === 0 ? (
              <div
                style={{
                  padding: '16px',
                  textAlign: 'center',
                  color: 'var(--color-muted)',
                }}
              >
                No packs yet.
              </div>
            ) : (
              packs.map((pack) => (
                <div
                  key={pack.slug}
                  data-testid={`pack-row-${pack.slug}`}
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
                    aria-label={pack.published ? 'Published' : 'Draft'}
                    style={{
                      width: '10px',
                      height: '10px',
                      borderRadius: '50%',
                      backgroundColor: pack.published
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
                    {pack.name}
                    <span
                      style={{
                        display: 'block',
                        fontWeight: 400,
                        fontSize: '0.75rem',
                        color: 'var(--color-muted)',
                      }}
                    >
                      {pack.slug} · {comboLabel(pack.size, pack.mode)}
                    </span>
                  </span>
                  <span
                    style={{
                      fontSize: '0.875rem',
                      color: 'var(--color-muted)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {pack.puzzleIds.length} puzzles
                  </span>
                  <button
                    type="button"
                    onClick={() => setEditing({ kind: 'edit', pack })}
                    aria-label={`Edit ${pack.name}`}
                    data-testid={`edit-${pack.slug}`}
                    style={{ ...buttonStyle, fontSize: '0.75rem', padding: '6px 10px' }}
                  >
                    Edit
                  </button>
                </div>
              ))
            )}
          </div>
        )}

        {editing?.kind === 'create' && (
          <PackAssembly onClose={() => setEditing(null)} />
        )}
        {editing?.kind === 'edit' && (
          <PackAssembly
            key={editing.pack.slug}
            pack={editing.pack}
            onClose={() => setEditing(null)}
          />
        )}
      </div>
    </PageShell>
  );
}
