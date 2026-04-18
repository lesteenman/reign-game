import 'fake-indexeddb/auto';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../theme/ThemeContext';
import { GamePage } from './GamePage';
import { FALLBACK_PUZZLE } from '../App';
import type { PuzzleData } from '../engine/types';

const MOCK_PUZZLE_WITH_METADATA: PuzzleData = {
  ...FALLBACK_PUZZLE,
  puzzleId: 'pool-001',
  metadata: {
    pipeline: 'iterative',
    solver: 'propagation',
    regions: 'bfs',
    regionVariance: 0.0,
    generationDurationMs: 4200,
    createdAt: '2026-04-15T10:30:00Z',
  },
};

// Track navigate calls
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Track fetchNextPuzzle and updatePuzzleStatus calls
let lastFetchArgs: { size: number; mode: string } | undefined;
let fetchCallCount = 0;
let mockFetchResult: () => Promise<PuzzleData> = () => Promise.resolve(MOCK_PUZZLE_WITH_METADATA);
const mockUpdateStatus = vi.fn().mockResolvedValue(undefined);

vi.mock('../services/puzzleService', () => ({
  fetchNextPuzzle: (size: number, mode: string) => {
    lastFetchArgs = { size, mode };
    fetchCallCount++;
    return mockFetchResult();
  },
  updatePuzzleStatus: (...args: unknown[]) => mockUpdateStatus(...args),
  NoPuzzlesAvailableError: class NoPuzzlesAvailableError extends Error {
    constructor(message = 'No puzzles available') {
      super(message);
      this.name = 'NoPuzzlesAvailableError';
    }
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
  mockUpdateStatus.mockClear();
  fetchCallCount = 0;
  lastFetchArgs = undefined;
  mockFetchResult = () => Promise.resolve(MOCK_PUZZLE_WITH_METADATA);
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
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('new=true'));
  });
});

describe('GamePage fetchNextPuzzle integration', () => {
  it('calls fetchNextPuzzle with size and mode from URL params', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=7&mode=double');
    await waitForHeader();

    // Assert
    expect(lastFetchArgs).toEqual({ size: 7, mode: 'double' });
  });

  it('uses default size=5 and mode=standard when params absent', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true');
    await waitForHeader();

    // Assert
    expect(lastFetchArgs).toEqual({ size: 5, mode: 'standard' });
  });

  it('does not read pipeline, solver, regions, or regionVariance from URL', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=9&mode=standard&pipeline=iterative&solver=propagation');
    await waitForHeader();

    // Assert — only size and mode matter now
    expect(lastFetchArgs).toEqual({ size: 9, mode: 'standard' });
  });
});

describe('GamePage error states', () => {
  it('shows generic error on fetch failure', async () => {
    // Arrange
    mockFetchResult = () => Promise.reject(new Error('network error'));
    renderGamePage('/play?new=true&size=7&mode=standard');

    // Act
    const errorState = await screen.findByTestId('error-state');

    // Assert
    expect(errorState).toBeInTheDocument();
  });

  it('Try Again on error retries fetch without navigating', async () => {
    // Arrange
    mockFetchResult = () => Promise.reject(new Error('generation failed'));
    renderGamePage('/play?new=true&size=9&mode=double');

    // Act
    await screen.findByTestId('error-state');
    const initialCalls = fetchCallCount;
    mockFetchResult = () => Promise.resolve(MOCK_PUZZLE_WITH_METADATA);
    fireEvent.click(screen.getByRole('button', { name: /try again/i }));

    // Assert — re-fetches without navigating
    await waitFor(() => {
      expect(fetchCallCount).toBeGreaterThan(initialCalls);
    });
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});

describe('GamePage no-puzzles state (FE-04)', () => {
  it('shows no-puzzles message when fetchNextPuzzle throws NoPuzzlesAvailableError', async () => {
    // Arrange — import the mock error class
    const { NoPuzzlesAvailableError } = await import('../services/puzzleService');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderGamePage('/play?new=true&size=7&mode=standard');

    // Act
    const noPuzzlesState = await screen.findByTestId('no-puzzles-state');

    // Assert
    expect(noPuzzlesState).toBeInTheDocument();
    expect(noPuzzlesState).toHaveTextContent('No puzzles available for this size and mode');
  });

  it('shows retry button in no-puzzles state', async () => {
    // Arrange
    const { NoPuzzlesAvailableError } = await import('../services/puzzleService');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderGamePage('/play?new=true&size=7&mode=standard');

    // Act
    await screen.findByTestId('no-puzzles-state');

    // Assert
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('retry button re-fetches without navigating', async () => {
    // Arrange
    const { NoPuzzlesAvailableError } = await import('../services/puzzleService');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderGamePage('/play?new=true&size=9&mode=double');

    // Act
    await screen.findByTestId('no-puzzles-state');
    const initialCalls = fetchCallCount;
    mockFetchResult = () => Promise.resolve(MOCK_PUZZLE_WITH_METADATA);
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));

    // Assert — re-fetches without navigating
    await waitFor(() => {
      expect(fetchCallCount).toBeGreaterThan(initialCalls);
    });
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});

describe('GamePage metadata display (FE-05)', () => {
  it('shows metadata when puzzle has metadata', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true&size=5&mode=standard');
    await waitForHeader();

    // Assert
    const metadata = screen.getByTestId('puzzle-metadata');
    expect(metadata).toBeInTheDocument();
    expect(metadata).toHaveTextContent('iterative');
    expect(metadata).toHaveTextContent('propagation');
    expect(metadata).toHaveTextContent('4.2s');
  });

  it('formats duration in ms when under 1000ms', async () => {
    // Arrange
    mockFetchResult = () => Promise.resolve({
      ...MOCK_PUZZLE_WITH_METADATA,
      metadata: { ...MOCK_PUZZLE_WITH_METADATA.metadata!, generationDurationMs: 450 },
    });
    renderGamePage('/play?new=true&size=5&mode=standard');
    await waitForHeader();

    // Assert
    const metadata = screen.getByTestId('puzzle-metadata');
    expect(metadata).toHaveTextContent('450ms');
  });

  it('does not show metadata when puzzle has no metadata', async () => {
    // Arrange
    mockFetchResult = () => Promise.resolve(FALLBACK_PUZZLE);
    renderGamePage('/play?new=true&size=5&mode=standard');
    await waitForHeader();

    // Assert
    expect(screen.queryByTestId('puzzle-metadata')).not.toBeInTheDocument();
  });

  it('does not show metadata for resumed game without metadata', async () => {
    // Arrange
    mockLoadState.mockResolvedValue({
      id: 'current',
      puzzle: FALLBACK_PUZZLE,
      cells: Array.from({ length: 5 }, () => Array(5).fill('empty')),
      timer: { elapsedAtLastPause: 10, lastResumedAt: null },
      status: 'in-progress',
      startedAt: Date.now(),
    });
    renderGamePage('/play');
    await waitForHeader();

    // Assert
    expect(screen.queryByTestId('puzzle-metadata')).not.toBeInTheDocument();
  });
});

describe('GamePage undo/redo (R-060)', () => {
  const emptyGrid5 = () => Array.from({ length: 5 }, () => Array(5).fill('empty'));

  function mockStateWithUndoable() {
    const empty = emptyGrid5();
    const afterTap: ('empty' | 'excluded' | 'marked')[][] = emptyGrid5();
    afterTap[0]![0] = 'excluded';
    mockLoadState.mockResolvedValue({
      id: 'current',
      puzzle: FALLBACK_PUZZLE,
      cells: afterTap,
      timer: { elapsedAtLastPause: 10, lastResumedAt: null },
      status: 'in-progress',
      startedAt: Date.now(),
      history: { past: [empty], future: [] },
    });
  }

  it('renders undo and redo buttons, initially disabled for fresh puzzle', async () => {
    // Arrange & Act
    renderGamePage('/play?new=true');
    await waitForHeader();

    // Assert
    const undoBtn = await screen.findByTestId('undo-button');
    expect(undoBtn).toBeDisabled();
    expect(screen.getByTestId('redo-button')).toBeDisabled();
  });

  it('undo button is enabled when loaded state has past history', async () => {
    // Arrange
    mockStateWithUndoable();

    // Act
    renderGamePage('/play');
    await waitForHeader();

    // Assert
    const undoBtn = await screen.findByTestId('undo-button');
    expect(undoBtn).not.toBeDisabled();
    expect(screen.getByTestId('redo-button')).toBeDisabled();
  });

  it('clicking undo reverts state and flips button enabled states', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    const undoBtn = await screen.findByTestId('undo-button');
    expect(undoBtn).not.toBeDisabled();

    // Act
    fireEvent.click(undoBtn);

    // Assert
    await waitFor(() => expect(undoBtn).toBeDisabled());
    expect(screen.getByTestId('redo-button')).not.toBeDisabled();
  });

  it('Ctrl+Z triggers undo when history is available', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    await screen.findByTestId('undo-button');

    // Act
    fireEvent.keyDown(window, { key: 'z', ctrlKey: true });

    // Assert
    await waitFor(() => expect(screen.getByTestId('undo-button')).toBeDisabled());
    expect(screen.getByTestId('redo-button')).not.toBeDisabled();
  });

  it('Ctrl+Shift+Z triggers redo after an undo', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    await screen.findByTestId('undo-button');
    fireEvent.keyDown(window, { key: 'z', ctrlKey: true });
    await waitFor(() => expect(screen.getByTestId('redo-button')).not.toBeDisabled());

    // Act
    fireEvent.keyDown(window, { key: 'z', ctrlKey: true, shiftKey: true });

    // Assert
    await waitFor(() => expect(screen.getByTestId('redo-button')).toBeDisabled());
    expect(screen.getByTestId('undo-button')).not.toBeDisabled();
  });

  it('Meta+Z also triggers undo (Mac chord)', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    await screen.findByTestId('undo-button');

    // Act
    fireEvent.keyDown(window, { key: 'z', metaKey: true });

    // Assert
    await waitFor(() => expect(screen.getByTestId('undo-button')).toBeDisabled());
  });

  it('plain Z (no modifier) does not trigger undo', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    const undoBtn = await screen.findByTestId('undo-button');

    // Act
    fireEvent.keyDown(window, { key: 'z' });

    // Assert — still enabled, nothing happened
    expect(undoBtn).not.toBeDisabled();
  });

  it('saveState payload includes history after an undo', async () => {
    // Arrange
    mockStateWithUndoable();
    renderGamePage('/play');
    await waitForHeader();
    const undoBtn = await screen.findByTestId('undo-button');
    mockSaveState.mockClear();

    // Act
    fireEvent.click(undoBtn);

    // Assert — debounced save eventually fires with future populated (undone state)
    await waitFor(() => {
      const calls = mockSaveState.mock.calls;
      const withFuture = calls.find((c: unknown[]) => {
        const state = c[0] as { history?: { past: unknown[]; future: unknown[] } };
        return state.history !== undefined && state.history.future.length > 0;
      });
      expect(withFuture).toBeDefined();
    }, { timeout: 1000 });
  });
});
