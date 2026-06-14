import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import type { CSSProperties } from 'react';
import { PageShell } from '@shared/components/PageShell';
import { SecondaryButton } from '@shared/components/Button';
import { Spinner } from '@shared/components/Spinner';
import { usePacks } from '@features/packs/hooks/usePacks';
import { useAllPackProgress } from '@features/packs/hooks/useAllPackProgress';
import type { PackSummary } from '@features/packs/types/pack';

/**
 * Packs-browse surface. Lists published packs (`GET /api/packs`) with
 * name, size/mode, puzzle count, and local progress ("3/8"); picking
 * one routes to `/play?flow=pack&pack={slug}`. Reached from the landing
 * Packs tile; lazy-loaded out of the landing first-paint bundle.
 *
 * Visual idiom mirrors LandingPage's tiles (BRAND_GUIDELINES §5.5) so
 * the browse list reads as a continuation of the landing surface.
 */

const containerStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '16px',
  width: '100%',
  maxWidth: '600px',
  alignItems: 'stretch',
};

const tileStyle: CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  boxShadow: '0 3px 0 var(--color-ink)',
  padding: '24px',
  display: 'flex',
  flexDirection: 'column',
  gap: '4px',
  textAlign: 'left',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  textDecoration: 'none',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  transition: 'transform 100ms ease-out',
};

const tileTitleStyle: CSSProperties = {
  fontSize: '1.25rem',
  fontWeight: 800,
  margin: 0,
};

const tileMetaStyle: CSSProperties = {
  fontSize: '0.875rem',
  color: 'var(--color-muted)',
  margin: 0,
};

const stateStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '16px',
  padding: '48px 0',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 600,
  color: 'var(--color-body)',
};

/** "5×5 Standard · 3/8" meta line for a pack tile. */
function packMeta(pack: PackSummary, solvedCount: number): string {
  const mode = pack.mode.charAt(0).toUpperCase() + pack.mode.slice(1);
  return `${pack.size}×${pack.size} ${mode} · ${solvedCount}/${pack.puzzleCount}`;
}

export function PacksBrowsePage() {
  const navigate = useNavigate();
  const packsQuery = usePacks();
  const progressQuery = useAllPackProgress();

  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  const retry = useCallback(() => {
    void packsQuery.refetch();
  }, [packsQuery]);

  if (packsQuery.isPending) {
    return (
      <PageShell onBack={handleBack} subtitle="Puzzle Packs">
        <div style={stateStyle} data-testid="packs-loading">
          <Spinner />
        </div>
      </PageShell>
    );
  }

  if (packsQuery.error) {
    return (
      <PageShell onBack={handleBack} subtitle="Puzzle Packs">
        <div style={stateStyle} data-testid="packs-error">
          <p style={{ margin: 0 }}>Could not load packs</p>
          <SecondaryButton onClick={retry} data-testid="packs-retry">
            Try again
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  const packs = packsQuery.data;

  if (packs.length === 0) {
    return (
      <PageShell onBack={handleBack} subtitle="Puzzle Packs">
        <div style={stateStyle} data-testid="packs-empty">
          <p style={{ margin: 0 }}>No packs available yet</p>
        </div>
      </PageShell>
    );
  }

  const progress = progressQuery.data ?? {};

  return (
    <PageShell onBack={handleBack} subtitle="Puzzle Packs">
      <div style={containerStyle} data-testid="packs-list">
        {packs.map((pack) => {
          const solved = progress[pack.slug]?.solvedPuzzleIds ?? [];
          // Clamp the badge to the served count so a stale solved-set
          // (admin re-curated the pack) never shows "9/8". The browse
          // page has no ordered puzzle-id list, so solved-count reaching
          // the served count is the complete signal here.
          const solvedCount = Math.min(solved.length, pack.puzzleCount);
          const complete =
            pack.puzzleCount > 0 && solvedCount >= pack.puzzleCount;
          return (
            <button
              key={pack.slug}
              type="button"
              data-testid={`pack-tile-${pack.slug}`}
              onClick={() => navigate(`/play?flow=pack&pack=${pack.slug}`)}
              style={tileStyle}
            >
              <h2 style={tileTitleStyle}>{pack.name}</h2>
              <p style={tileMetaStyle}>
                {packMeta(pack, solvedCount)}
                {complete ? ' · Complete' : ''}
              </p>
            </button>
          );
        })}
      </div>
    </PageShell>
  );
}
