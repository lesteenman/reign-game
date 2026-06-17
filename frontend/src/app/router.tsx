import { lazy, Suspense } from 'react';
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useSearchParams,
} from 'react-router-dom';
import { LandingPage } from '@features/landing/pages/LandingPage';
import { DailyFlow } from '@features/daily/screens/DailyFlow';
import { PageShell } from '@shared/components/PageShell';
import { Spinner } from '@shared/components/Spinner';

// Admin + curation are admin-gated routes most users never reach, so
// their code is lazily loaded and split out of the initial bundle.
// Landing and the daily flow stay eager — daily is the primary user
// flow and is kept free of a Suspense navigation waterfall.
const CurationPage = lazy(() =>
  import('@features/curation/pages/CurationPage').then((m) => ({
    default: m.CurationPage,
  })),
);
const PlayPuzzlePage = lazy(() =>
  import('@features/curation/pages/PlayPuzzlePage').then((m) => ({
    default: m.PlayPuzzlePage,
  })),
);
const ProtectedAdminRoute = lazy(() =>
  import('@features/admin/components/ProtectedAdminRoute').then((m) => ({
    default: m.ProtectedAdminRoute,
  })),
);
const AdminPacksPage = lazy(() =>
  import('@features/admin/pages/AdminPacksPage').then((m) => ({
    default: m.AdminPacksPage,
  })),
);
// The player pack flow (browse + play) is a secondary surface, so it is
// lazily split out of the eager landing bundle — same pattern as the
// admin/curation routes above.
const PacksBrowsePage = lazy(() =>
  import('@features/packs/pages/PacksBrowsePage').then((m) => ({
    default: m.PacksBrowsePage,
  })),
);
const PackFlow = lazy(() =>
  import('@features/packs/screens/PackFlow').then((m) => ({
    default: m.PackFlow,
  })),
);

/**
 * Suspense fallback for lazy routes — reuses the existing
 * `<Spinner>` inside `<PageShell>` so it matches `AdminLoading` /
 * `DailyFlow`'s loading state byte-for-byte (no layout shift; CLS
 * stays 0).
 */
function RouteFallback() {
  return (
    <PageShell>
      <div data-testid="route-loading" aria-live="polite">
        <Spinner />
      </div>
    </PageShell>
  );
}

/**
 * `/play` is shared between two features, dispatched on the `flow=`
 * query param:
 *
 *   /play?flow=curation        → features/curation/pages/PlayPuzzlePage
 *   /play?flow=daily           → features/daily/screens/DailyFlow
 *   /play?flow=pack&pack={slug} → features/packs/screens/PackFlow
 *   /play (missing/unknown flow) → redirect home
 *
 * The dispatch lives in the router (here) rather than inside either
 * feature page so that features/curation/ doesn't import
 * features/daily/ — a cross-feature import that BR's
 * `import/no-restricted-paths` rule forbids.
 */
function PlayRoute() {
  const [searchParams] = useSearchParams();
  const flow = searchParams.get('flow');
  if (flow === 'daily') {
    return (
      <div data-testid="daily-flow">
        <DailyFlow />
      </div>
    );
  }
  if (flow === 'curation') {
    return (
      <Suspense fallback={<RouteFallback />}>
        <PlayPuzzlePage />
      </Suspense>
    );
  }
  if (flow === 'pack') {
    // `?pack=` is the published pack's slug. A missing/unknown slug
    // surfaces a graceful not-found inside PackFlow (slug=null → its
    // not-found branch; a fetched-but-404 pack → its error branch).
    const slug = searchParams.get('pack');
    return (
      <Suspense fallback={<RouteFallback />}>
        <PackFlow slug={slug} />
      </Suspense>
    );
  }
  // Unknown or missing flow → home (preserves the legacy ST-11
  // no-state redirect behaviour).
  return <Navigate to="/" replace />;
}

/**
 * Top-level app router. Composed by `main.tsx` inside `<Providers>`.
 *
 * Extracted from `src/App.tsx` in #176 — the previous `App.tsx`'s only
 * job was to mount `<BrowserRouter>` + `<Routes>`. After this slice,
 * `src/App.tsx` is gone and `main.tsx` mounts `<Router />` directly.
 */
export function Router() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route
          path="/packs"
          element={
            <Suspense fallback={<RouteFallback />}>
              <PacksBrowsePage />
            </Suspense>
          }
        />
        <Route path="/play" element={<PlayRoute />} />
        <Route
          path="/curation"
          element={
            <Suspense fallback={<RouteFallback />}>
              <ProtectedAdminRoute>
                <CurationPage />
              </ProtectedAdminRoute>
            </Suspense>
          }
        />
        <Route
          path="/admin"
          element={
            <Suspense fallback={<RouteFallback />}>
              <ProtectedAdminRoute />
            </Suspense>
          }
        />
        <Route
          path="/admin/packs"
          element={
            <Suspense fallback={<RouteFallback />}>
              <ProtectedAdminRoute>
                <AdminPacksPage />
              </ProtectedAdminRoute>
            </Suspense>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
