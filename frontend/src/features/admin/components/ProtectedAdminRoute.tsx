import { useUser } from "@clerk/react";
import type { ReactNode } from 'react';
import { AdminPage } from '@features/admin/pages/AdminPage';
import { AdminLandingPage } from '@features/admin/pages/AdminLandingPage';
import { PageShell } from '@shared/components/PageShell';
import { Spinner } from '@shared/components/Spinner';
import { getClerkUserRole } from '@shared/auth/role';

/**
 * Admin-route loading indicator — Clerk is still deciding whether the
 * visitor is signed in / has the admin role. Uses the shared `<Spinner>`
 * (added in #176 chrome-cards slice) so this auth-path indicator
 * matches DailyFlow's loading state byte-for-byte.
 */
function AdminLoading() {
  return (
    <PageShell>
      <div data-testid="admin-loading" aria-live="polite">
        <Spinner />
      </div>
    </PageShell>
  );
}

/**
 * Generic admin-route gate. Per auth-surface.md AS-09 the page renders
 * one of three user-facing states, plus a loading state while Clerk
 * is still deciding which one applies:
 *
 *   1. !isLoaded           → loading indicator
 *   2. !isSignedIn         → AdminLandingPage state="anonymous"
 *   3. role !== 'admin'    → AdminLandingPage state="forbidden"
 *   4. role === 'admin'    → render `children` (defaults to <AdminPage />
 *                            for backwards compatibility with the
 *                            original /admin route).
 *
 * The frontend check is cosmetic — the backend RequireAdmin middleware
 * is the source of truth for authorisation (AS-04). This component
 * just prevents non-admins from seeing the UI chrome and exposes a
 * useful next action (sign-in or sign-out) instead.
 *
 * Usage:
 *   <Route path="/admin" element={<ProtectedAdminRoute />} />
 *   <Route path="/curation" element={<ProtectedAdminRoute><CurationPage /></ProtectedAdminRoute>} />
 */
export function ProtectedAdminRoute({ children }: { children?: ReactNode } = {}) {
  const { isLoaded, isSignedIn, user } = useUser();

  if (!isLoaded) {
    return <AdminLoading />;
  }

  if (!isSignedIn || !user) {
    return <AdminLandingPage state="anonymous" />;
  }

  if (getClerkUserRole(user.publicMetadata) !== 'admin') {
    return <AdminLandingPage state="forbidden" />;
  }

  return <>{children ?? <AdminPage />}</>;
}
