import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { LandingPage } from './pages/LandingPage';
import { GamePage } from './pages/GamePage';
import { CurationPage } from './pages/CurationPage';
import { ProtectedAdminRoute } from './features/admin/components/ProtectedAdminRoute';
import type { PuzzleData } from './engine/types';

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
  );
}

export default App;
