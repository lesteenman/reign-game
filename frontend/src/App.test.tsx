import { render, screen, cleanup } from './test-utils';
import { MemoryRouter } from 'react-router-dom';
import { expect, test, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from './theme/ThemeContext';
import { LandingPage } from './pages/LandingPage';
import { FALLBACK_PUZZLE } from './App';

const originalFetch = globalThis.fetch;

beforeEach(() => {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(FALLBACK_PUZZLE),
  });
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

test('renders Reign heading on landing page', async () => {
  renderLanding();
  const heading = screen.getByRole('heading', { name: /reign/i });
  expect(heading).toBeInTheDocument();
});

test('shows Play button when no saved state', async () => {
  renderLanding();
  const playButton = await screen.findByRole('button', { name: /play/i });
  expect(playButton).toBeInTheDocument();
});

test('shows loading state initially', () => {
  renderLanding();
  expect(screen.getByTestId('loading-state')).toBeInTheDocument();
});
