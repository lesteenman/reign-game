import { SignOutButton } from '@clerk/clerk-react';
import type { CSSProperties } from 'react';
import { PageShell } from '../components/common/PageShell';
import { SignInButton } from '../components/auth/SignInButton';

type LandingState = 'anonymous' | 'forbidden';

interface AdminLandingPageProps {
  state: LandingState;
}

// Card styling aligned with BRAND_GUIDELINES §5.5: 2px ink border,
// 10px radius, 3px ink offset shadow, surface background.
const cardStyle: CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  boxShadow: '0 3px 0 var(--color-ink)',
  padding: '32px 24px',
  maxWidth: 480,
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

// Match the SignInButton visual so the forbidden-state button reads as
// a sibling control — same sizing, same ink shadow.
const signOutButtonStyle: CSSProperties = {
  padding: '8px 16px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 700,
  fontSize: '0.875rem',
  backgroundColor: 'var(--color-surface)',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  boxShadow: '0 3px 0 var(--color-ink)',
  minHeight: '44px',
  minWidth: '44px',
};

/**
 * Landing page rendered at `/admin` when the visitor is not authorised
 * to see the admin UI. Two mutually-exclusive states per AS-09:
 *
 *   - 'anonymous' — no session; invite to sign in.
 *   - 'forbidden' — signed in but `publicMetadata.role !== 'admin'`;
 *     offer a sign-out so they can switch accounts.
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
        <SignOutButton>
          <button type="button" style={signOutButtonStyle}>
            Sign out
          </button>
        </SignOutButton>
      </div>
    </PageShell>
  );
}
