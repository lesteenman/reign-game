import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useSearchParams,
} from 'react-router-dom';
import { LandingPage } from '@features/landing/pages/LandingPage';
import { CurationPage } from '@features/curation/pages/CurationPage';
import { PlayPuzzlePage } from '@features/curation/pages/PlayPuzzlePage';
import { ProtectedAdminRoute } from '@features/admin/components/ProtectedAdminRoute';
import { DailyFlow } from '@features/daily/screens/DailyFlow';

/**
 * `/play` is shared between two features, dispatched on the `flow=`
 * query param:
 *
 *   /play?flow=curation → features/curation/pages/PlayPuzzlePage
 *   /play?flow=daily    → features/daily/screens/DailyFlow
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
  if (flow === 'curation') return <PlayPuzzlePage />;
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
        <Route path="/play" element={<PlayRoute />} />
        <Route
          path="/curation"
          element={
            <ProtectedAdminRoute>
              <CurationPage />
            </ProtectedAdminRoute>
          }
        />
        <Route path="/admin" element={<ProtectedAdminRoute />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
