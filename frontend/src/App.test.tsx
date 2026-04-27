import { render, screen, cleanup } from './test-utils';
import { MemoryRouter } from 'react-router-dom';
import { expect, test, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from './theme/ThemeContext';
import { LandingPage } from './pages/LandingPage';
import { FALLBACK_PUZZLE } from './App';

// Mock the Clerk hook the new tile-based LandingPage reads — same
// pattern used in LandingPage.test.tsx / GamePage.test.tsx.
type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};
const useUserMock = vi.fn<() => UseUserReturn>();
vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
}));

const originalFetch = globalThis.fetch;

beforeEach(() => {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(FALLBACK_PUZZLE),
  });
  useUserMock.mockReset();
  useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
});

afterEach(() => {
  cleanup();
  globalThis.fetch = originalFetch;
});

function renderLanding() {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={['/']}>
        <LandingPage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

test('renders Reign heading on landing page (from PageShell)', () => {
  renderLanding();
  const heading = screen.getByRole('heading', { name: /reign/i });
  expect(heading).toBeInTheDocument();
});

test('renders the three top-level tiles container', () => {
  renderLanding();
  expect(screen.getByTestId('landing-tiles')).toBeInTheDocument();
  expect(screen.getByTestId('tile-daily')).toBeInTheDocument();
  expect(screen.getByTestId('tile-packs')).toBeInTheDocument();
});

test('Daily and Packs tiles are disabled placeholders', () => {
  renderLanding();
  expect(screen.getByTestId('tile-daily')).toBeDisabled();
  expect(screen.getByTestId('tile-packs')).toBeDisabled();
});
