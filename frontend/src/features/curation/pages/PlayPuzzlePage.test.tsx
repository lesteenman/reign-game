import 'fake-indexeddb/auto';
import { render, screen, fireEvent, cleanup, waitFor } from '@shared/test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { PlayPuzzlePage } from './PlayPuzzlePage';
import { FALLBACK_PUZZLE } from '@shared/test-fixtures';
import type { PuzzleData } from '@engine/types';

const MOCK_PUZZLE_WITH_METADATA: PuzzleData = {
  ...FALLBACK_PUZZLE,
  puzzleId: 'pool-001',
  metadata: {
    difficulty: 2,
    maxTier: 2,
    tierCounts: [0, 3, 1, 0, 0],
    traceLen: 4,
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
const mockSubmitVerdict = vi.fn().mockResolvedValue(undefined);

vi.mock('@features/curation/services/fetch-next-puzzle-service', () => ({
  fetchNextPuzzle: (size: number, mode: string) => {
    lastFetchArgs = { size, mode };
    fetchCallCount++;
    return mockFetchResult();
  },
  NoPuzzlesAvailableError: class NoPuzzlesAvailableError extends Error {
    constructor(message = 'No puzzles available') {
      super(message);
      this.name = 'NoPuzzlesAvailableError';
    }
  },
}));

vi.mock('@shared/game/services/puzzleService', () => ({
  updatePuzzleStatus: (...args: unknown[]) => mockUpdateStatus(...args),
}));

vi.mock('@features/curation/services/verdictService', () => ({
  submitVerdict: (...args: unknown[]) => mockSubmitVerdict(...args),
}));

// Clerk hook mock — controls the role-gated UI (verdict surface, Skip
// button) under three states: signedOut, role=user, role=admin.
type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};
const useUserMock = vi.fn<() => UseUserReturn>();
vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
}));

// Mock useGameStorage with stable references
const mockLoadState = vi.fn().mockResolvedValue(null);
const mockSaveState = vi.fn().mockResolvedValue(undefined);
const mockClearState = vi.fn().mockResolvedValue(undefined);
const mockAddCompletion = vi.fn().mockResolvedValue(undefined);
vi.mock('@shared/game/hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    loadState: mockLoadState,
    saveState: mockSaveState,
    clearState: mockClearState,
    addCompletion: mockAddCompletion,
  }),
}));

const originalFetch = globalThis.fetch;

beforeEach(() => {
  mockNavigate.mockClear();
  mockLoadState.mockClear();
  mockLoadState.mockResolvedValue(null);
  mockSaveState.mockClear();
  mockClearState.mockClear();
  mockAddCompletion.mockClear();
  mockUpdateStatus.mockClear();
  mockSubmitVerdict.mockClear();
  mockSubmitVerdict.mockResolvedValue(undefined);
  mockUpdateStatus.mockResolvedValue(undefined);
  // Default to signed-out state. Each test that needs a different
  // Clerk state overrides via useUserMock.mockReturnValue(...) before
  // calling renderPage.
  useUserMock.mockReset();
  useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
  sessionStorage.clear();
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

function renderPage(initialEntry = '/play?flow=curation&size=5&mode=standard') {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <PlayPuzzlePage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

/** Wait for the page header to appear (ensures PageShell is rendered). */
async function waitForHeader() {
  await screen.findByTestId('page-header');
}

describe('PlayPuzzlePage header', () => {
  it('renders back button', async () => {
    // Arrange
    renderPage();

    // Act
    await waitForHeader();

    // Assert
    const backButton = screen.getByTestId('back-button');
    expect(backButton).toBeInTheDocument();
    expect(backButton).toHaveAttribute('aria-label', 'Back to home');
  });

  it('back button navigates to / on click', async () => {
    // Arrange
    renderPage();
    await waitForHeader();

    // Act
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('renders dark mode toggle in header', async () => {
    // Arrange
    renderPage();

    // Act
    await waitForHeader();

    // Assert
    const toggle = screen.getByTestId('dark-mode-toggle');
    expect(toggle).toBeInTheDocument();
    expect(toggle).toHaveAttribute('aria-label', expect.stringMatching(/switch to (light|dark) mode/i));
  });

  it('dark mode toggle changes mode on click', async () => {
    // Arrange
    renderPage();
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

describe('PlayPuzzlePage back navigation preserves state', () => {
  it('does not clear IndexedDB when navigating back', async () => {
    // Arrange
    renderPage();
    await waitForHeader();

    // Act — navigate back
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert — navigate was called with '/' (back-to-home, not a /play
    // URL that would discard state).
    expect(mockNavigate).toHaveBeenCalledWith('/');
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('/play'));
  });
});

describe('PlayPuzzlePage fetchNextPuzzle integration', () => {
  it('calls fetchNextPuzzle with size and mode from URL params', async () => {
    // Arrange & Act
    renderPage('/play?flow=curation&size=7&mode=double');
    await waitForHeader();

    // Assert
    expect(lastFetchArgs).toEqual({ size: 7, mode: 'double' });
  });

  it('uses default size=5 and mode=standard when params absent', async () => {
    // Arrange & Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert
    expect(lastFetchArgs).toEqual({ size: 5, mode: 'standard' });
  });

  it('does not read pipeline, solver, regions, or regionVariance from URL', async () => {
    // Arrange & Act
    renderPage('/play?flow=curation&size=9&mode=standard&pipeline=iterative&solver=propagation');
    await waitForHeader();

    // Assert — only size and mode matter now
    expect(lastFetchArgs).toEqual({ size: 9, mode: 'standard' });
  });
});

describe('PlayPuzzlePage error states', () => {
  it('shows generic error on fetch failure', async () => {
    // Arrange
    mockFetchResult = () => Promise.reject(new Error('network error'));
    renderPage('/play?flow=curation&size=7&mode=standard');

    // Act
    const errorState = await screen.findByTestId('error-state');

    // Assert
    expect(errorState).toBeInTheDocument();
  });

  it('Try Again on error retries fetch without navigating', async () => {
    // Arrange
    mockFetchResult = () => Promise.reject(new Error('generation failed'));
    renderPage('/play?flow=curation&size=9&mode=double');

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

describe('PlayPuzzlePage no-puzzles state (FE-04)', () => {
  it('shows no-puzzles message when fetchNextPuzzle throws NoPuzzlesAvailableError', async () => {
    // Arrange — import the mock error class
    const { NoPuzzlesAvailableError } = await import('@features/curation/services/fetch-next-puzzle-service');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderPage('/play?flow=curation&size=7&mode=standard');

    // Act
    const noPuzzlesState = await screen.findByTestId('no-puzzles-state');

    // Assert
    expect(noPuzzlesState).toBeInTheDocument();
    expect(noPuzzlesState).toHaveTextContent('No puzzles available for this size and mode');
  });

  it('shows retry button in no-puzzles state', async () => {
    // Arrange
    const { NoPuzzlesAvailableError } = await import('@features/curation/services/fetch-next-puzzle-service');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderPage('/play?flow=curation&size=7&mode=standard');

    // Act
    await screen.findByTestId('no-puzzles-state');

    // Assert
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('retry button re-fetches without navigating', async () => {
    // Arrange
    const { NoPuzzlesAvailableError } = await import('@features/curation/services/fetch-next-puzzle-service');
    mockFetchResult = () => Promise.reject(new NoPuzzlesAvailableError());
    renderPage('/play?flow=curation&size=9&mode=double');

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

describe('PlayPuzzlePage — saveState failure is tolerated (#176 review-finding)', () => {
  // The pre-#176 useEffect-based loader swallowed `saveState` errors
  // and the user could still play the in-memory puzzle (degraded mode:
  // no resume on refresh). The TanStack migration must preserve that
  // tolerance — surfacing the persistence error as a user-visible
  // "Try Again" UI would block play and burn the fetched puzzle on
  // every retry. Regression test: when saveState rejects, the game
  // must still render.

  it('renders the GameBoard when fetchNextPuzzle succeeds but saveState rejects', async () => {
    // Arrange — fetch ok, IDB write blows up (quota, private mode, etc.)
    mockLoadState.mockResolvedValue(null);
    mockSaveState.mockRejectedValueOnce(new Error('QuotaExceededError'));
    // Suppress the console.warn the hook emits in this branch — its
    // presence is the contract; the test asserts the warn fired below.
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    const timer = await screen.findByTestId('timer-display');

    // Assert — GameBoard mounted, error UI did NOT mount
    expect(timer).toBeInTheDocument();
    expect(screen.queryByTestId('error-state')).not.toBeInTheDocument();
    expect(screen.queryByTestId('no-puzzles-state')).not.toBeInTheDocument();
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('saveState failed'),
      expect.any(Error),
    );

    warnSpy.mockRestore();
  });
});

describe('PlayPuzzlePage metadata display (FE-05)', () => {
  // After the Services→TanStack migration (#176), the loading state's
  // PageShell renders `page-header` BEFORE `useQuery` resolves the
  // load+fetch cascade; `waitForHeader()` no longer implies the
  // GameBoard has mounted. Tests here wait on `timer-display` (only
  // present in the loaded GameBoard) before asserting against
  // `puzzle-metadata`'s presence or absence.

  it('shows metadata when puzzle has metadata', async () => {
    // Arrange & Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await screen.findByTestId('timer-display');

    // Assert
    const metadata = screen.getByTestId('puzzle-metadata');
    expect(metadata).toBeInTheDocument();
    expect(metadata).toHaveTextContent('difficulty 2');
    expect(metadata).toHaveTextContent('4.2s');
  });

  it('formats duration in ms when under 1000ms', async () => {
    // Arrange
    mockFetchResult = () => Promise.resolve({
      ...MOCK_PUZZLE_WITH_METADATA,
      metadata: { ...MOCK_PUZZLE_WITH_METADATA.metadata!, generationDurationMs: 450 },
    });
    renderPage('/play?flow=curation&size=5&mode=standard');
    await screen.findByTestId('timer-display');

    // Assert
    const metadata = screen.getByTestId('puzzle-metadata');
    expect(metadata).toHaveTextContent('450ms');
  });

  it('does not show metadata when puzzle has no metadata', async () => {
    // Arrange
    mockFetchResult = () => Promise.resolve(FALLBACK_PUZZLE);
    renderPage('/play?flow=curation&size=5&mode=standard');
    await screen.findByTestId('timer-display');

    // Assert
    expect(screen.queryByTestId('puzzle-metadata')).not.toBeInTheDocument();
  });

  it('does not show metadata for resumed game without metadata', async () => {
    // Arrange
    mockLoadState.mockResolvedValue({
      id: 'curation:5x5-standard',
      flowType: 'curation',
      flowId: '5x5-standard',
      puzzle: FALLBACK_PUZZLE,
      cells: Array.from({ length: 5 }, () => Array(5).fill('empty')),
      timer: { elapsedAtLastPause: 10, lastResumedAt: null },
      status: 'in-progress',
      startedAt: Date.now(),
    });
    renderPage('/play?flow=curation&size=5&mode=standard');
    await screen.findByTestId('timer-display');

    // Assert
    expect(screen.queryByTestId('puzzle-metadata')).not.toBeInTheDocument();
  });
});

describe('PlayPuzzlePage undo/redo (R-060)', () => {
  const emptyGrid5 = () => Array.from({ length: 5 }, () => Array(5).fill('empty'));

  function mockStateWithUndoable() {
    const empty = emptyGrid5();
    const afterTap: ('empty' | 'excluded' | 'marked')[][] = emptyGrid5();
    afterTap[0]![0] = 'excluded';
    mockLoadState.mockResolvedValue({
      id: 'curation:5x5-standard',
      flowType: 'curation',
      flowId: '5x5-standard',
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert
    const undoBtn = await screen.findByTestId('undo-button');
    expect(undoBtn).not.toBeDisabled();
    expect(screen.getByTestId('redo-button')).toBeDisabled();
  });

  it('clicking undo reverts state and flips button enabled states', async () => {
    // Arrange
    mockStateWithUndoable();
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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
    renderPage('/play?flow=curation&size=5&mode=standard');
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

describe('PlayPuzzlePage — Skip button visibility (FB-11)', () => {
  it('hides the Skip button when signed out', async () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });

    // Act
    renderPage();
    await waitForHeader();

    // Assert
    expect(screen.queryByTestId('skip-button')).not.toBeInTheDocument();
  });

  it('hides the Skip button when signed in as a user-role player', async () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'user' } },
    });

    // Act
    renderPage();
    await waitForHeader();

    // Assert
    expect(screen.queryByTestId('skip-button')).not.toBeInTheDocument();
  });

  it('shows the Skip button when signed in as admin', async () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });

    // Act
    renderPage();
    await waitForHeader();

    // Assert
    const skip = await screen.findByTestId('skip-button');
    expect(skip).toBeInTheDocument();
    expect(skip).toHaveTextContent('Skip puzzle');
  });

  it('hides the Skip button if Clerk has not loaded yet (no flicker)', async () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });

    // Act
    renderPage();
    await waitForHeader();

    // Assert
    expect(screen.queryByTestId('skip-button')).not.toBeInTheDocument();
  });
});

describe('PlayPuzzlePage — Skip modal flow (FB-02 §2)', () => {
  beforeEach(() => {
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });
  });

  it('opens the modal with three buttons when admin clicks Skip', async () => {
    // Arrange
    renderPage();
    await waitForHeader();
    const skip = await screen.findByTestId('skip-button');

    // Act
    fireEvent.click(skip);

    // Assert — three buttons inside the modal
    await screen.findByTestId('skip-modal');
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'I hate this' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Just skip' })).toBeInTheDocument();
    // Status PUT not called by the button click itself (FB-11).
    expect(mockUpdateStatus).not.toHaveBeenCalled();
    expect(mockSubmitVerdict).not.toHaveBeenCalled();
  });

  it('Cancel closes the modal without calling any service', async () => {
    // Arrange
    renderPage();
    await waitForHeader();
    const skip = await screen.findByTestId('skip-button');
    fireEvent.click(skip);
    await screen.findByTestId('skip-modal');

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    // Assert
    await waitFor(() => {
      expect(screen.queryByTestId('skip-modal')).not.toBeInTheDocument();
    });
    expect(mockUpdateStatus).not.toHaveBeenCalled();
    expect(mockSubmitVerdict).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalledWith('/curation');
  });

  it('"I hate this" calls both updatePuzzleStatus and submitVerdict, then navigates to /curation', async () => {
    // Arrange
    renderPage();
    await waitForHeader();
    const skip = await screen.findByTestId('skip-button');
    fireEvent.click(skip);
    await screen.findByTestId('skip-modal');

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'I hate this' }));

    // Assert
    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledTimes(1);
      expect(mockSubmitVerdict).toHaveBeenCalledTimes(1);
    });
    expect(mockSubmitVerdict).toHaveBeenCalledWith(
      expect.objectContaining({
        value: 'down',
        outcome: 'skipped',
      }),
    );
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/curation');
    });
  });

  it('"Just skip" calls only updatePuzzleStatus, then navigates to /curation', async () => {
    // Arrange
    renderPage();
    await waitForHeader();
    const skip = await screen.findByTestId('skip-button');
    fireEvent.click(skip);
    await screen.findByTestId('skip-modal');

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Just skip' }));

    // Assert
    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledTimes(1);
      expect(mockNavigate).toHaveBeenCalledWith('/curation');
    });
    expect(mockSubmitVerdict).not.toHaveBeenCalled();
  });
});

describe('PlayPuzzlePage — Flow Slot URL contract', () => {
  const emptyGrid5 = (): ('empty' | 'excluded' | 'marked')[][] =>
    Array.from({ length: 5 }, () => Array(5).fill('empty'));

  it('redirects home when flow query param is unrecognized (ST-11)', async () => {
    // Arrange
    renderPage('/play?flow=junk&size=5&mode=standard');

    // Act — wait for the redirect effect to fire
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true });
    });

    // Assert — no fetch/load happened for an unknown flow
    expect(fetchCallCount).toBe(0);
    expect(mockLoadState).not.toHaveBeenCalled();
  });

  it('redirects home when flow query param is missing', async () => {
    // Arrange — no flow param at all
    renderPage('/play?size=5&mode=standard');

    // Act
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true });
    });

    // Assert
    expect(fetchCallCount).toBe(0);
    expect(mockLoadState).not.toHaveBeenCalled();
  });

  it('mount with valid flow + no slot → fetch path runs and saves a fresh slot', async () => {
    // Arrange — explicit miss
    mockLoadState.mockResolvedValue(null);

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert — fetched the right (size, mode); saveState wrote a slot
    // tagged with the derived (flowType, flowId).
    expect(lastFetchArgs).toEqual({ size: 5, mode: 'standard' });
    await waitFor(() => {
      const matched = mockSaveState.mock.calls.find((c: unknown[]) => {
        const state = c[0] as { flowType?: string; flowId?: string };
        return state.flowType === 'curation' && state.flowId === '5x5-standard';
      });
      expect(matched).toBeDefined();
    });
  });

  it('mount with pre-seeded matching in-progress slot → resume (no fetch)', async () => {
    // Arrange — slot already present, status in-progress
    mockLoadState.mockResolvedValue({
      id: 'curation:5x5-standard',
      flowType: 'curation',
      flowId: '5x5-standard',
      puzzle: FALLBACK_PUZZLE,
      cells: emptyGrid5(),
      timer: { elapsedAtLastPause: 12, lastResumedAt: null },
      status: 'in-progress',
      startedAt: Date.now(),
    });

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert — no fetch happened; the resume path took over.
    expect(fetchCallCount).toBe(0);
  });

  it("mount with pre-seeded slot whose status is 'solved' → defensive fetch (ST-06)", async () => {
    // Arrange — solved slot is treated as a miss; clear-on-solve
    // should normally have removed it, but a crashed completion
    // could leave it behind.
    mockLoadState.mockResolvedValue({
      id: 'curation:5x5-standard',
      flowType: 'curation',
      flowId: '5x5-standard',
      puzzle: FALLBACK_PUZZLE,
      cells: emptyGrid5(),
      timer: { elapsedAtLastPause: 30, lastResumedAt: null },
      status: 'solved',
      startedAt: Date.now(),
    });

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert — fetch ran exactly once (the defensive fallback).
    expect(fetchCallCount).toBe(1);
  });

  it('clears the right Flow Slot on solve (ST-07)', async () => {
    // Arrange — pre-seed an in-progress slot with cells that already
    // satisfy FALLBACK_PUZZLE's constraints (1-marker-per-row/col/region,
    // no adjacency). When useGame computes `isSolved`, the completion
    // effect fires on mount and calls clearState.
    //
    // Solved layout for FALLBACK_PUZZLE region map:
    //   r0c3, r1c1, r2c4, r3c2, r4c0
    const solvedCells: ('empty' | 'excluded' | 'marked')[][] = emptyGrid5();
    solvedCells[0]![3] = 'marked';
    solvedCells[1]![1] = 'marked';
    solvedCells[2]![4] = 'marked';
    solvedCells[3]![2] = 'marked';
    solvedCells[4]![0] = 'marked';

    mockLoadState.mockResolvedValue({
      id: 'curation:5x5-standard',
      flowType: 'curation',
      flowId: '5x5-standard',
      puzzle: FALLBACK_PUZZLE,
      cells: solvedCells,
      timer: { elapsedAtLastPause: 5, lastResumedAt: null },
      status: 'in-progress',
      startedAt: Date.now(),
    });

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert — clearState was called with the matching slot.
    await waitFor(() => {
      expect(mockClearState).toHaveBeenCalledWith('curation', '5x5-standard');
    });
  });

  it('switching pools preserves the prior slot — slot A is never cleared by mounting B', async () => {
    // Arrange — first mount slot A (5x5 standard), let it settle, then
    // unmount and mount slot B (7x7 double).
    mockLoadState.mockResolvedValue(null);
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Sanity: slot A's saveState ran with the right tag.
    await waitFor(() => {
      const matched = mockSaveState.mock.calls.find((c: unknown[]) => {
        const state = c[0] as { flowType?: string; flowId?: string };
        return state.flowType === 'curation' && state.flowId === '5x5-standard';
      });
      expect(matched).toBeDefined();
    });

    // Act — unmount A, mount B with a different (size, mode).
    cleanup();
    mockLoadState.mockResolvedValue(null);
    renderPage('/play?flow=curation&size=7&mode=double');
    await waitForHeader();

    // Assert — no clearState call ever targeted slot A. Slot B may
    // call its own clear on solve (it never solves here), but slot A
    // must not be touched.
    const aClearCalls = mockClearState.mock.calls.filter(
      (args: unknown[]) =>
        args[0] === 'curation' && args[1] === '5x5-standard',
    );
    expect(aClearCalls).toHaveLength(0);
  });
});

// `flow=daily` delegation moved to the router (`src/app/router.tsx`)
// in #176; this page no longer reads the daily flow. The delegation
// behaviour is covered structurally by `src/app/router.tsx`'s shape:
// `?flow=daily` returns `<DailyFlow />` directly without touching this
// page. No equivalent test lives here anymore.

describe('PlayPuzzlePage — timer starts on mount (not on first tap)', () => {
  it('timer is running by the time the grid first renders, before any user input', async () => {
    // Arrange — fresh puzzle, no restored state. The timer must begin
    // ticking on mount so a user opening the daily/curation page sees
    // it count from 0 without needing to tap a cell first.
    mockLoadState.mockResolvedValue(null);
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert — the timer-display is mounted (with elapsed=0 initially)
    // and a saveState payload includes a non-null `lastResumedAt`
    // within the debounce window. That `lastResumedAt` is the proof
    // the timer has been .start()ed without a pointer event.
    const timer = await screen.findByTestId('timer-display');
    expect(timer).toBeInTheDocument();
    await waitFor(() => {
      const startedSave = mockSaveState.mock.calls.find((c: unknown[]) => {
        const state = c[0] as { timer?: { lastResumedAt: number | null } };
        return state.timer?.lastResumedAt !== null && state.timer?.lastResumedAt !== undefined;
      });
      expect(startedSave).toBeDefined();
    }, { timeout: 1500 });
  });
});

describe('PlayPuzzlePage — admin sees Skip button (curation flow)', () => {
  // Skip puzzle is the admin verdict surface and belongs to the
  // curation flow. The daily-flow negative case (admin does NOT see
  // Skip on daily) is verified at the GameBoard level — see
  // GameBoard.test.tsx's "daily flow hides Skip button (admin)" test
  // — because the daily flow no longer routes through this page after
  // #176's GamePage split.

  it('curation flow: admin sees Skip button', async () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });

    // Act
    renderPage('/play?flow=curation&size=5&mode=standard');
    await waitForHeader();

    // Assert
    expect(await screen.findByTestId('skip-button')).toBeInTheDocument();
  });
});
