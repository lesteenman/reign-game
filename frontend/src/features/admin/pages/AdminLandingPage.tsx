import type { CSSProperties } from 'react';
import { Link } from 'react-router-dom';
import { PageShell } from '../../../components/common/PageShell';
import { SignInButton } from '../../../components/auth/SignInButton';
import { compactSecondaryButtonStyle } from '../../../components/common/buttonStyles';
import { pressIn, pressOut } from '../../../components/common/press';

type LandingState = 'anonymous' | 'forbidden';

interface AdminLandingPageProps {
  state: LandingState;
}

// Card width cap: beyond this, the heading + body + single CTA stop
// feeling like a focused landing and start looking like a sparse dialog
// on desktop. 480px keeps the card comfortably readable at any breakpoint.
const CARD_MAX_WIDTH_PX = 480;

// Card styling aligned with BRAND_GUIDELINES §5.5: 2px ink border,
// 10px radius, 3px ink offset shadow, surface background.
const cardStyle: CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  boxShadow: '0 3px 0 var(--color-ink)',
  padding: '32px 24px',
  maxWidth: CARD_MAX_WIDTH_PX,
  width: '100%',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '16px',
  textAlign: 'center',
};

const headingStyle: CSSProperties = {
  margin: 0,
  fontSize: '1.5rem',
  fontWeight: 800,
  letterSpacing: '-0.01em',
  color: 'var(--color-ink)',
};

const bodyStyle: CSSProperties = {
  margin: 0,
  fontSize: '1rem',
  color: 'var(--color-body)',
  lineHeight: 1.5,
};

/**
 * Landing page rendered at `/admin` when the visitor is not authorised
 * to see the admin UI. Two mutually-exclusive states per AS-09:
 *
 *   - 'anonymous' — no session; invite to sign in.
 *   - 'forbidden' — signed in but `publicMetadata.role !== 'admin'`;
 *     offer a way back to the game. Sign-out is reachable from the
 *     header avatar; duplicating it here would be redundant.
 */
export function AdminLandingPage({ state }: AdminLandingPageProps) {
  if (state === 'anonymous') {
    return (
      <PageShell>
        <div style={cardStyle} data-testid="admin-landing-anonymous">
          <h2 style={headingStyle}>Admin Access</h2>
          <p style={bodyStyle}>Sign in to access the admin panel.</p>
          <SignInButton />
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell>
      <div style={cardStyle} data-testid="admin-landing-forbidden">
        <h2 style={headingStyle}>No Admin Access</h2>
        <p style={bodyStyle}>This account doesn&apos;t have admin access.</p>
        <Link
          to="/"
          style={{ ...compactSecondaryButtonStyle, textDecoration: 'none' }}
          onMouseDown={(e) => pressIn(e, 'var(--color-ink)')}
          onMouseUp={(e) => pressOut(e, 'var(--color-ink)')}
          onMouseLeave={(e) => pressOut(e, 'var(--color-ink)')}
        >
          Home
        </Link>
      </div>
    </PageShell>
  );
}
