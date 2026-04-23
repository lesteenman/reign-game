import { render, screen, fireEvent, waitFor, cleanup } from '../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../theme/ThemeContext';
import { LandingPage, buildPlayUrl } from './LandingPage';

// Mock useGameStorage
const mockLoadState = vi.fn();
vi.mock('../hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    loadState: mockLoadState,
    saveState: vi.fn(),
    addCompletion: vi.fn(),
  }),
}));

// Mock the enabled-modes fetch so the page's mount effect doesn't hit
// a real backend. Returns the pre-Phase-5 preset list by default.
const mockFetchEnabledModes = vi.fn();
vi.mock('../services/landingService', () => ({
  fetchEnabledModes: (...args: unknown[]) => mockFetchEnabledModes(...args),
}));

// Track navigation
let navigatedTo: string | undefined;
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => (path: string) => { navigatedTo = path; },
  };
});

beforeEach(() => {
  navigatedTo = undefined;
  mockLoadState.mockResolvedValue(null); // fresh state by default
  mockFetchEnabledModes.mockResolvedValue([
    { size: 5, mode: 'standard' },
    { size: 7, mode: 'standard' },
    { size: 9, mode: 'standard' },
    { size: 9, mode: 'double' },
  ]);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
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

describe('LandingPage', () => {
  describe('fresh state (no active puzzle)', () => {
    it('shows puzzle selector with preset buttons', async () => {
      // Arrange & Act
      renderLanding();

      // Assert
      await waitFor(() => {
        expect(screen.getByTestId('puzzle-selector')).toBeInTheDocument();
      });
      expect(screen.getByTestId('preset-0')).toBeInTheDocument();
      expect(screen.getByTestId('preset-1')).toBeInTheDocument();
      expect(screen.getByTestId('preset-2')).toBeInTheDocument();
    });

    it('Play navigates with default preset params', async () => {
      // Arrange
      renderLanding();
      await waitFor(() => {
        expect(screen.getByTestId('play-button')).toBeInTheDocument();
      });

      // Act
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(navigatedTo).toBe('/play?new=true&size=5&mode=standard');
    });

    it('Play navigates with selected preset params', async () => {
      // Arrange
      renderLanding();
      await waitFor(() => {
        expect(screen.getByTestId('preset-2')).toBeInTheDocument();
      });

      // Act
      fireEvent.click(screen.getByTestId('preset-2'));
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(navigatedTo).toBe('/play?new=true&size=9&mode=standard');
    });
  });

  describe('has-progress state (active puzzle)', () => {
    beforeEach(() => {
      mockLoadState.mockResolvedValue({ status: 'in-progress' });
    });

    it('shows Resume and New Puzzle buttons', async () => {
      // Arrange & Act
      renderLanding();

      // Assert
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
      });
      expect(screen.getByRole('button', { name: /new puzzle/i })).toBeInTheDocument();
    });

    it('clicking New Puzzle reveals the selector', async () => {
      // Arrange
      renderLanding();
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /new puzzle/i })).toBeInTheDocument();
      });

      // Act
      fireEvent.click(screen.getByRole('button', { name: /new puzzle/i }));

      // Assert
      expect(screen.getByTestId('puzzle-selector')).toBeInTheDocument();
    });

    it('Resume navigates to /play', async () => {
      // Arrange
      renderLanding();
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
      });

      // Act
      fireEvent.click(screen.getByRole('button', { name: /resume/i }));

      // Assert
      expect(navigatedTo).toBe('/play');
    });
  });

  describe('buildPlayUrl', () => {
    it('builds URL with size and mode', () => {
      // Arrange & Act
      const url = buildPlayUrl({ size: 7, mode: 'standard' });

      // Assert
      expect(url).toBe('/play?new=true&size=7&mode=standard');
    });

    it('builds URL with double mode', () => {
      // Arrange & Act
      const url = buildPlayUrl({ size: 9, mode: 'double' });

      // Assert
      expect(url).toBe('/play?new=true&size=9&mode=double');
    });

    it('does not include advanced params', () => {
      // Arrange & Act
      const url = buildPlayUrl({ size: 5, mode: 'standard' });

      // Assert
      expect(url).not.toContain('pipeline');
      expect(url).not.toContain('solver');
      expect(url).not.toContain('regions');
      expect(url).not.toContain('regionVariance');
    });
  });
});
