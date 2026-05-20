import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { LandingPage } from '@pages/LandingPage';
import { GamePage } from '@pages/GamePage';
import { CurationPage } from '@pages/CurationPage';
import { ProtectedAdminRoute } from '@features/admin/components/ProtectedAdminRoute';

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
