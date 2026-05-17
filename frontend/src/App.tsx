import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from './theme/ThemeContext';
import { LandingPage } from './pages/LandingPage';
import { GamePage } from './pages/GamePage';
import { CurationPage } from './pages/CurationPage';
import { ProtectedAdminRoute } from './features/admin/components/ProtectedAdminRoute';
import type { PuzzleData } from './engine/types';

// TanStack Query client (Track 2 foundation; first useQuery/useMutation
// adoptions land in Track 3 alongside the LoadState migration). Single
// instance for the whole app — recreating per render breaks caching.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Conservative defaults; tune in Track 3 once we have real
      // usage to measure against.
      staleTime: 30 * 1000,
      refetchOnWindowFocus: false,
    },
  },
});

/**
 * Hardcoded 5x5 puzzle used as fallback for development/testing
 * when the backend is unavailable.
 */
export const FALLBACK_PUZZLE: PuzzleData = {
  puzzleId: 'playtest-001',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<LandingPage />} />
            <Route path="/play" element={<GamePage />} />
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
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default App;
