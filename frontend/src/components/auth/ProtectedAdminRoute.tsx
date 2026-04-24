import { useUser } from "@clerk/react";
import type { CSSProperties } from 'react';
import { AdminPage } from '../../pages/AdminPage';
import { AdminLandingPage } from '../../pages/AdminLandingPage';
import { PageShell } from '../common/PageShell';

const loadingStyle: CSSProperties = {
  textAlign: 'center',
  padding: '24px',
  color: 'var(--color-muted)',
  fontSize: '0.875rem',
};

/**
 * Tiny pulse spinner — the project doesn't ship a shared Spinner yet,
 * so we use a minimal dots animation that degrades gracefully under
 * `prefers-reduced-motion`. Kept inline to avoid a global CSS addition.
 */
function AdminLoading() {
  return (
    <PageShell>
      <div
        data-testid="admin-loading"
        role="status"
        aria-live="polite"
        style={loadingStyle}
      >
        Loading...
      </div>
    </PageShell>
  );
}

/**
 * Route gate for `/admin`. Per auth-surface.md AS-09 the page renders
 * one of three user-facing states, plus a loading state while Clerk
 * is still deciding which one applies:
 *
 *   1. !isLoaded           → loading indicator
 *   2. !isSignedIn         → AdminLandingPage state="anonymous"
 *   3. role !== 'admin'    → AdminLandingPage state="forbidden"
 *   4. role === 'admin'    → AdminPage (the real admin UI)
 *
 * The frontend check is cosmetic — the backend RequireAdmin middleware
 * is the source of truth for authorisation (AS-04). This component
 * just prevents non-admins from seeing the UI chrome and exposes a
 * useful next action (sign-in or sign-out) instead.
 */
export function ProtectedAdminRoute() {
  const { isLoaded, isSignedIn, user } = useUser();

  if (!isLoaded) {
    return <AdminLoading />;
  }

  if (!isSignedIn || !user) {
    return <AdminLandingPage state="anonymous" />;
  }

  const role = resolveRole(user.publicMetadata);
  if (role !== 'admin') {
    return <AdminLandingPage state="forbidden" />;
  }

  return <AdminPage />;
}

function resolveRole(publicMetadata: Record<string, unknown> | undefined): string {
  if (!publicMetadata) return '';
  const role = publicMetadata.role;
  return typeof role === 'string' ? role : '';
}
