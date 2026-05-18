import type { ReactNode } from 'react';
import { Show } from "@clerk/react";
import { useDarkMode } from '../../theme/useDarkMode';
import { SignInButton } from '../auth/SignInButton';
import { UserMenu } from '../auth/UserMenu';
import { useClerkAvailable } from '../auth/ClerkAvailability';
import { OfflineBanner } from './OfflineBanner';

interface PageShellProps {
  children: ReactNode;
  /** When provided, renders a back button that calls this handler. */
  onBack?: () => void;
  /** Accessible label for the back button. Defaults to "Back to home". */
  backLabel?: string;
  /**
   * Optional caption rendered directly under the Reign wordmark.
   * Used by flow-specific pages (e.g. the daily flow renders
   * "Daily Puzzle · May 11"). Omit for generic pages.
   */
  subtitle?: ReactNode;
}

/**
 * Header-right auth slot. Renders Clerk-driven sign-in / user menu
 * when ClerkProvider is mounted; otherwise renders nothing so the
 * anonymous game still works in dev environments without a Clerk
 * key. The backend is still the source of truth for authorisation
 * (auth-surface.md AS-04).
 *
 * `<Show when="signed-in|signed-out">` is the v6 replacement for the
 * removed `<SignedIn>` / `<SignedOut>` components (Clerk core-3
 * upgrade). See @clerk/react 6 upgrade guide.
 */
function HeaderAuthSlot() {
  const clerkAvailable = useClerkAvailable();
  if (!clerkAvailable) {
    return null;
  }
  return (
    <>
      <Show when="signed-out">
        <SignInButton />
      </Show>
      <Show when="signed-in">
        <UserMenu />
      </Show>
    </>
  );
}

/**
 * Standard page layout wrapper. Header pinned to top, body content laid
 * out top-down within the remaining viewport. Keeps the puzzle visible
 * on phone-portrait viewports (e.g. 360x800, 390x844) — vertically
 * centering would push the header down and float the action row below
 * the fold.
 */
export function PageShell({ children, onBack, backLabel = 'Back to home', subtitle }: PageShellProps) {
  const { isDark, toggle } = useDarkMode();

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '16px',
        padding: '16px 16px 24px',
        minHeight: '100vh',
        backgroundColor: 'var(--color-background)',
        fontFamily: '"Nunito Sans", system-ui, sans-serif',
        color: 'var(--color-ink)',
      }}
    >
      {/* Header row: back button (left) | title (center) | auth + dark-mode toggle (right) */}
      <div
        data-testid="page-header"
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          width: '100%',
          maxWidth: 600,
        }}
      >
        {/* Left: back button or spacer */}
        {onBack ? (
          <button
            type="button"
            onClick={onBack}
            aria-label={backLabel}
            data-testid="back-button"
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: '1.25rem',
              padding: '8px',
              color: 'var(--color-ink)',
              lineHeight: 1,
              minWidth: 44,
              minHeight: 44,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {'←'}
          </button>
        ) : (
          <div style={{ minWidth: 44 }} />
        )}

        {/* Center: title */}
        <h1
          style={{
            fontSize: '1.875rem',
            fontWeight: 800,
            letterSpacing: '-0.01em',
            margin: 0,
          }}
        >
          Reign
        </h1>

        {/* Right: auth slot + dark mode toggle. The admin link no
            longer lives in the header directly — it's inside the user
            menu, role-gated per AS-10. */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <HeaderAuthSlot />
          <button
            type="button"
            onClick={toggle}
            aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            data-testid="dark-mode-toggle"
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: '1.25rem',
              padding: '8px',
              color: 'var(--color-muted)',
              lineHeight: 1,
              minWidth: 44,
              minHeight: 44,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {isDark ? '☀' : '☾'}
          </button>
        </div>
      </div>
      <OfflineBanner />
      {subtitle && (
        <p
          data-testid="page-subtitle"
          style={{
            margin: 0,
            color: 'var(--color-muted)',
            fontWeight: 600,
            fontSize: '0.875rem',
            letterSpacing: '0.01em',
            textAlign: 'center',
          }}
        >
          {subtitle}
        </p>
      )}
      {children}
    </div>
  );
}
