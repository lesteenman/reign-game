import type { CSSProperties } from 'react';
import { useOnlineStatus } from '../../shared/hooks/useOnlineStatus';

// Style: inline CSSProperties using theme tokens (no Tamagui).
// Tamagui migration tracked in #176.

const bannerStyle: CSSProperties = {
  width: '100%',
  maxWidth: 600,
  padding: '8px 16px',
  borderRadius: 'var(--radius)',
  backgroundColor: 'var(--color-destructive-bg)',
  color: 'var(--color-destructive)',
  fontSize: '0.875rem',
  fontWeight: 600,
  textAlign: 'center',
};

/**
 * Global offline indicator. Renders nothing while online; renders a
 * polite-priority aria-live banner when navigator.onLine === false.
 * Mounted by PageShell so every route gets the same signal.
 */
export function OfflineBanner() {
  const online = useOnlineStatus();
  if (online) return null;
  return (
    <div
      data-testid="offline-banner"
      role="alert"
      aria-live="polite"
      style={bannerStyle}
    >
      {"You're offline — connect to start a new puzzle or sync results."}
    </div>
  );
}
