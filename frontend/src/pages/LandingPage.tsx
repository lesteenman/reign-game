import { useNavigate } from 'react-router-dom';
import { useUser } from '@clerk/react';
import type { CSSProperties, ReactNode } from 'react';
import { PageShell } from '../components/common/PageShell';
import { getClerkUserRole } from '../components/auth/role';

/**
 * Landing page.
 *
 * Three top-level tiles for the project's eventual feature set:
 * Daily, Packs, and (admin-only) Curation. Daily is live and navigates
 * to `/play?flow=daily` for both Anonymous and User identities. Packs
 * remains a disabled placeholder — a future feature (curated bundles
 * built from verdict-up puzzles in the corpus).
 *
 * The Curation tile is rendered only when
 * `getClerkUserRole(user.publicMetadata) === 'admin'`.
 */

const containerStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '16px',
  width: '100%',
  maxWidth: '600px',
  alignItems: 'stretch',
};

const tileBaseStyle: CSSProperties = {
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

const tileDisabledStyle: CSSProperties = {
  ...tileBaseStyle,
  opacity: 0.5,
  cursor: 'not-allowed',
  pointerEvents: 'none',
};

const tileTitleStyle: CSSProperties = {
  fontSize: '1.25rem',
  fontWeight: 800,
  margin: 0,
};

const tileSubtitleStyle: CSSProperties = {
  fontSize: '0.875rem',
  color: 'var(--color-muted)',
  margin: 0,
  fontStyle: 'italic',
};

interface LandingTileProps {
  testId: string;
  title: string;
  subtitle: ReactNode;
  enabled: boolean;
  onClick?: () => void;
}

function LandingTile({ testId, title, subtitle, enabled, onClick }: LandingTileProps) {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={enabled ? onClick : undefined}
      disabled={!enabled}
      aria-disabled={!enabled}
      style={enabled ? tileBaseStyle : tileDisabledStyle}
    >
      <h2 style={tileTitleStyle}>{title}</h2>
      <p style={tileSubtitleStyle}>{subtitle}</p>
    </button>
  );
}

export function LandingPage() {
  const navigate = useNavigate();
  const { user, isLoaded } = useUser();
  const isAdmin = isLoaded && getClerkUserRole(user?.publicMetadata) === 'admin';

  return (
    <PageShell>
      <div style={containerStyle} data-testid="landing-tiles">
        <LandingTile
          testId="tile-daily"
          title="Today's puzzle"
          subtitle="One 9×9 puzzle a day. UTC midnight resets."
          enabled
          onClick={() => navigate('/play?flow=daily')}
        />
        <LandingTile
          testId="tile-packs"
          title="Puzzle Packs"
          subtitle="Coming soon"
          enabled={false}
        />
        {isAdmin && (
          <LandingTile
            testId="tile-curation"
            title="Curation"
            subtitle="Rate puzzles for the future corpus"
            enabled
            onClick={() => navigate('/curation')}
          />
        )}
      </div>
    </PageShell>
  );
}
