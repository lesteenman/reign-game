import 'fake-indexeddb/auto';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../theme/ThemeContext';
import { GamePage } from './GamePage';
import { FALLBACK_PUZZLE } from '../App';
import type { GenerateOptions } from '../services/puzzleService';

// Track navigate calls
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Track generatePuzzle calls
let lastGenerateOptions: GenerateOptions | undefined;
vi.mock('../services/puzzleService', () => ({
  generatePuzzle: (options: GenerateOptions) => {
    lastGenerateOptions = options;
    return Promise.resolve(FALLBACK_PUZZLE);
  },
}));

// Mock useGameStorage with stable references
const mockLoadState = vi.fn().mockResolvedValue(null);
const mockSaveState = vi.fn().mockResolvedValue(undefined);
const mockAddCompletion = vi.fn().mockResolvedValue(undefined);
vi.mock('../hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    loadState: mockLoadState,
    saveState: mockSaveState,
    addCompletion: mockAddCompletion,
  }),
}));

const originalFetch = globalThis.fetch;

beforeEach(() => {
  mockNavigate.mockClear();
  mockLoadState.mockClear();
  mockSaveState.mockClear();
  mockAddCompletion.mockClear();
  lastGenerateOptions = undefined;
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(FALLBACK_PUZZLE),
  });
});

afterEach(() => {
  cleanup();
  globalThis.fetch = originalFetch;
});

function renderGamePage(initialEntry = '/play?new=true') {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <GamePage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

/** Wait for the page header to appear (ensures PageShell is rendered). */
async function waitForHeader() {
  await screen.findByTestId('page-header');
}

describe('GamePage header', () => {
  it('renders back button', async () => {
    // Arrange
    renderGamePage();

    // Act
    await waitForHeader();

    // Assert
    const backButton = screen.getByTestId('back-button');
    expect(backButton).toBeInTheDocument();
    expect(backButton).toHaveAttribute('aria-label', 'Back to home');
  });

  it('back button navigates to / on click', async () => {
    // Arrange
    renderGamePage();
    await waitForHeader();

    // Act
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('renders dark mode toggle in header', async () => {
    // Arrange
    renderGamePage();

    // Act
    await waitForHeader();

    // Assert
    const toggle = screen.getByTestId('dark-mode-toggle');
    expect(toggle).toBeInTheDocument();
    expect(toggle).toHaveAttribute('aria-label', expect.stringMatching(/switch to (light|dark) mode/i));
  });

  it('dark mode toggle changes mode on click', async () => {
    // Arrange
    renderGamePage();
    await waitForHeader();
    const toggle = screen.getByTestId('dark-mode-toggle');
    const initialLabel = toggle.getAttribute('aria-label');

    // Act
    fireEvent.click(toggle);

    // Assert
    await waitFor(() => {
      expect(toggle.getAttribute('aria-label')).not.toEqual(initialLabel);
    });
  });
});

describe('GamePage back navigation preserves state', () => {
  it('does not clear IndexedDB when navigating back', async () => {
    // Arrange
    renderGamePage();
    await waitForHeader();

    // Act — navigate back
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert — navigate was called with '/' (not '/play?new=true' which would discard state)
    expect(mockNavigate).toHaveBeenCalledWith('/');
    // The navigate call does NOT include any state-clearing logic;
    // only navigating to /play?new=true triggers a new puzzle generation
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('new=true'));
  });
});

describe('GamePage URL params for puzzle generation', () => {
  it('reads size and mode from URL params', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=7&mode=double');

    // Wait for generation to complete
    await waitForHeader();

    // Assert
    expect(lastGenerateOptions).toBeDefined();
    expect(lastGenerateOptions?.size).toBe(7);
    expect(lastGenerateOptions?.mode).toBe('double');
  });

  it('uses default size=5 and mode=standard when params absent', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true');
    await waitForHeader();

    // Assert
    expect(lastGenerateOptions).toBeDefined();
    expect(lastGenerateOptions?.size).toBe(5);
    expect(lastGenerateOptions?.mode).toBe('standard');
  });

  it('passes optional generator params when present', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=9&mode=standard&pipeline=iterative&solver=propagation&regions=wfc&regionVariance=0.3');
    await waitForHeader();

    // Assert
    expect(lastGenerateOptions).toBeDefined();
    expect(lastGenerateOptions?.pipeline).toBe('iterative');
    expect(lastGenerateOptions?.solver).toBe('propagation');
    expect(lastGenerateOptions?.regions).toBe('wfc');
    expect(lastGenerateOptions?.regionVariance).toBe(0.3);
  });

  it('omits optional params when not in URL', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=6');
    await waitForHeader();

    // Assert
    expect(lastGenerateOptions).toBeDefined();
    expect(lastGenerateOptions?.pipeline).toBeUndefined();
    expect(lastGenerateOptions?.solver).toBeUndefined();
    expect(lastGenerateOptions?.regions).toBeUndefined();
    expect(lastGenerateOptions?.regionVariance).toBeUndefined();
  });
});
