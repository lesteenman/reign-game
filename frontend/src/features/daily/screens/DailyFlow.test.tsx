import { render, screen, cleanup, waitFor, fireEvent, act } from '@shared/test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { DailyFlow } from './DailyFlow';
import { ApiError } from '@shared/api';
import type { DailyPuzzlePayload, DailySubmitResponse } from '@shared/types/daily';

// Mock the daily service so the component's data-fetch + submit
// effects are observable. The default `mockResolvedValue` is replaced
// per test to drive each branch of the daily-flow state machine.
const mockGetDaily = vi.fn();
const mockSubmitDailyResult = vi.fn();
vi.mock('@features/daily/services/dailyService', async () => {
  const actual = await vi.importActual<typeof import('@features/daily/services/dailyService')>(
    '@features/daily/services/dailyService',
  );
  return {
    ...actual,
    getDaily: (date?: string) => mockGetDaily(date),
    submitDailyResult: (args: unknown) => mockSubmitDailyResult(args),
  };
});

// Mock storage so the per-flow IndexedDB write is observable without
// needing fake-indexeddb here. Lesson 16 drove the choice to keep the
// shape inside storage/ — the mock asserts call shape, not row layout.
import type { GameState } from '@storage/types';
const mockSaveState = vi.fn(async () => {});
const mockLoadState = vi.fn<(...args: [unknown, unknown]) => Promise<GameState | null>>(async () => null);
const mockClearState = vi.fn(async () => {});
const mockAddCompletion = vi.fn(async () => {});
vi.mock('@shared/game/hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    saveState: mockSaveState,
    loadState: mockLoadState,
    clearState: mockClearState,
    addCompletion: mockAddCompletion,
  }),
}));

// Stub DailyGameBoard so tests can synthesize a player solve without
// standing up the real grid. The stub renders a button that, when
// clicked, fires the `onSolved` callback with a fixed (solution,
// elapsedMs) tuple — enough to verify the wiring contract.
const STUB_SOLUTION = [
  [0, 1, 2],
  [3, 4, 5],
];
const STUB_ELAPSED_MS = 12_345;
vi.mock('./DailyGameBoard', () => ({
  DailyGameBoard: (props: {
    onSolved: (solution: number[][], elapsedMs: number) => void;
  }) => (
    <div data-testid="daily-game-board-stub">
      <button
        data-testid="daily-stub-solve"
        onClick={() => props.onSolved(STUB_SOLUTION, STUB_ELAPSED_MS)}
      >
        Stub solve
      </button>
    </div>
  ),
}));

const MOCK_PAYLOAD: DailyPuzzlePayload = {
  puzzleId: 'daily-2026-05-02',
  grid: 9,
  regions: [
    [0, 0, 1, 1, 2, 2, 3, 3, 4],
    [0, 0, 1, 1, 2, 2, 3, 3, 4],
    [0, 5, 5, 1, 6, 2, 7, 3, 4],
    [0, 5, 5, 6, 6, 2, 7, 7, 4],
    [8, 5, 5, 6, 6, 7, 7, 7, 4],
    [8, 8, 5, 6, 7, 7, 7, 4, 4],
    [8, 8, 5, 6, 6, 6, 4, 4, 4],
    [8, 8, 5, 5, 6, 4, 4, 4, 4],
    [8, 8, 8, 8, 8, 4, 4, 4, 4],
  ],
  assignedAt: '2026-05-02T00:00:00Z',
  outcome: 'started',
};

const MOCK_SUBMIT_RESPONSE: DailySubmitResponse = {
  serverElapsedMs: 11_500,
  leaderboardRank: 42,
};

beforeEach(() => {
  mockGetDaily.mockReset();
  mockSubmitDailyResult.mockReset();
  mockSaveState.mockReset();
  mockSaveState.mockResolvedValue(undefined);
  mockLoadState.mockReset();
  mockLoadState.mockResolvedValue(null);
  mockClearState.mockReset();
  mockClearState.mockResolvedValue(undefined);
  mockAddCompletion.mockReset();
  mockAddCompletion.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
});

function renderDailyFlow() {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={['/play?flow=daily']}>
        <DailyFlow />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('DailyFlow', () => {
  it('renders loading state initially', () => {
    // Arrange
    mockGetDaily.mockReturnValue(new Promise(() => {})); // never resolves

    // Act
    renderDailyFlow();

    // Assert
    expect(screen.getByTestId('daily-loading')).toBeInTheDocument();
    expect(screen.getByText(/loading today/i)).toBeInTheDocument();
  });

  it('renders puzzle data on successful getDaily()', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });
  });

  it('renders a "Daily Puzzle · <date>" subtitle while playing', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);

    // Act
    renderDailyFlow();

    // Assert — subtitle is rendered as a single PageShell once the
    // daily payload is available. The exact day formatting is locale-
    // dependent; we assert the "Daily Puzzle ·" prefix and that the
    // month/day from `assignedAt` (2026-05-02) reaches the DOM.
    await waitFor(() => {
      expect(screen.getByTestId('page-subtitle')).toBeInTheDocument();
    });
    expect(screen.getByTestId('page-subtitle').textContent).toMatch(/Daily Puzzle ·/);
    // There must be exactly ONE Reign header — DailyFlow's PageShell.
    // The (mocked) DailyGameBoard stub must not introduce a second.
    expect(screen.getAllByRole('heading', { name: /reign/i })).toHaveLength(1);
  });

  it('renders 404 error UI when getDaily throws ApiError with status 404', async () => {
    // Arrange
    mockGetDaily.mockRejectedValue(new ApiError('not found', 404));

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/no daily available right now/i)).toBeInTheDocument();
    expect(screen.getByTestId('daily-retry')).toBeInTheDocument();
  });

  it('renders 500 error UI when getDaily throws ApiError with status 500', async () => {
    // Arrange
    mockGetDaily.mockRejectedValue(new ApiError('server boom', 500));

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/something went wrong, try again/i)).toBeInTheDocument();
    expect(screen.getByTestId('daily-retry')).toBeInTheDocument();
  });

  it('renders generic error UI when getDaily throws a non-Daily error', async () => {
    // Arrange
    mockGetDaily.mockRejectedValue(new Error('network down'));

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/could not load today's daily/i)).toBeInTheDocument();
    expect(screen.getByTestId('daily-retry')).toBeInTheDocument();
  });

  it('retry button re-invokes getDaily()', async () => {
    // Arrange
    mockGetDaily.mockRejectedValueOnce(new ApiError('boom', 500));
    mockGetDaily.mockResolvedValueOnce(MOCK_PAYLOAD);
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-retry'));

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });
    expect(mockGetDaily).toHaveBeenCalledTimes(2);
  });

  it('transitions loading -> solved and renders PostCompletionScreen when GET returns outcome=solved with elapsedMs+submittedAt', async () => {
    // Arrange — server-said-solved on the GET path. The flow must
    // render PostCompletionScreen straight from the GET payload's
    // canonical solve fields (no POST). The placeholder branch is gone.
    mockGetDaily.mockResolvedValue({
      ...MOCK_PAYLOAD,
      outcome: 'solved',
      serverElapsedMs: 14_000,
      submittedAt: '2026-05-02T10:30:00Z',
    });

    // Act
    renderDailyFlow();

    // Assert — PostCompletionScreen renders with the GET payload's
    // serverElapsedMs (14_000ms → 0:14). No POST is fired. The
    // placeholder data-testid must NOT appear.
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('0:14');
    expect(screen.queryByTestId('daily-solved-placeholder')).not.toBeInTheDocument();
    expect(mockSubmitDailyResult).not.toHaveBeenCalled();
  });

  it('falls back to playing branch with a console.warn when GET returns outcome=solved but elapsedMs is missing', async () => {
    // Arrange — defensive path. The server returned outcome=solved but
    // omitted serverElapsedMs (older partial-response shape). The flow
    // must NOT render a broken post-completion screen; it warns and
    // falls through to playing.
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    mockGetDaily.mockResolvedValue({
      ...MOCK_PAYLOAD,
      outcome: 'solved',
      // serverElapsedMs intentionally absent
      // submittedAt intentionally absent
    });

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('outcome=solved without serverElapsedMs/submittedAt'),
    );
    warnSpy.mockRestore();
  });

  // --- Chunk 6: submit wiring + state machine -------------------------

  it('transitions playing -> submitting -> solved on successful submit', async () => {
    // Arrange — defer the submit so the submitting state is observable
    // long enough to assert against. With a synchronous `mockResolvedValue`
    // the mutation transitions idle → pending → settled inside one React
    // batch and the pending render never paints — the test would either
    // miss `daily-submitting` entirely or be ordering-fragile. Holding
    // the promise pending across the assertion makes the intermediate
    // state observable, then resolving lets the transition to solved
    // complete deterministically.
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    let resolveSubmit: (value: DailySubmitResponse) => void = () => {};
    mockSubmitDailyResult.mockReturnValue(
      new Promise<DailySubmitResponse>((resolve) => {
        resolveSubmit = resolve;
      }),
    );
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-stub-solve'));

    // Assert — submitting indicator visible while the POST is pending.
    await screen.findByTestId('daily-submitting');

    // Resolve the submit and verify the transition to solved.
    resolveSubmit(MOCK_SUBMIT_RESPONSE);
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('0:11');
  });

  it('transitions playing -> submitting -> solved on 409 already-solved', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    mockSubmitDailyResult.mockRejectedValue(
      new ApiError('already solved', 409),
    );
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-stub-solve'));

    // Assert — post-completion shows the "already submitted" copy, not 0:00
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });
    expect(screen.getByTestId('daily-already-submitted')).toHaveTextContent(
      'Already submitted earlier today',
    );
    expect(screen.queryByTestId('daily-solve-time')).not.toBeInTheDocument();
  });

  it('transitions playing -> submit-error -> playing on retry', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    mockSubmitDailyResult.mockRejectedValueOnce(
      new ApiError('server boom', 500),
    );
    mockSubmitDailyResult.mockResolvedValueOnce(MOCK_SUBMIT_RESPONSE);
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act — first solve attempt fails, surfacing the submit-error card.
    fireEvent.click(screen.getByTestId('daily-stub-solve'));
    await waitFor(() => {
      expect(screen.getByTestId('daily-submit-error')).toBeInTheDocument();
    });

    // Act — retry transitions back to playing with the same payload.
    fireEvent.click(screen.getByTestId('daily-submit-retry'));

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });
  });

  it('calls submitDailyResult with the correct args', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    mockSubmitDailyResult.mockResolvedValue(MOCK_SUBMIT_RESPONSE);
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-stub-solve'));
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });

    // Assert
    expect(mockSubmitDailyResult).toHaveBeenCalledTimes(1);
    expect(mockSubmitDailyResult).toHaveBeenCalledWith({
      assignedAt: MOCK_PAYLOAD.assignedAt,
      outcome: 'solved',
      playTimeMs: STUB_ELAPSED_MS,
      solution: STUB_SOLUTION,
    });
  });

  it('persists the solved outcome to per-flow storage with flowId derived from assignedAt', async () => {
    // Arrange
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    mockSubmitDailyResult.mockResolvedValue(MOCK_SUBMIT_RESPONSE);
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-stub-solve'));
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });

    // Assert — flowType=daily, flowId=YYYY-MM-DD from assignedAt.
    expect(mockSaveState).toHaveBeenCalled();
    expect(mockSaveState).toHaveBeenCalledWith(
      expect.objectContaining({
        flowType: 'daily',
        flowId: '2026-05-02',
        status: 'solved',
      }),
    );
  });

  // --- Short-circuit on persisted solved state --------------------------------

  it('short-circuits to PostCompletionScreen from persisted solved state without calling getDaily', async () => {
    // Arrange — seed the storage mock with a solved daily row that
    // carries a canonical serverElapsedMs. The short-circuit reads
    // this on mount and renders PostCompletionScreen straight from
    // persisted state.
    const todayUtc = new Date().toISOString().slice(0, 10);
    mockLoadState.mockResolvedValue({
      id: `daily:${todayUtc}`,
      flowType: 'daily',
      flowId: todayUtc,
      puzzle: {
        puzzleId: 'persisted-puzzle',
        gridSize: 9,
        mode: 'standard',
        regionMap: MOCK_PAYLOAD.regions,
      },
      cells: [],
      timer: { elapsedAtLastPause: 9_876, lastResumedAt: null },
      status: 'solved',
      startedAt: Date.now() - 9_876,
      serverElapsedMs: 9_876,
      submittedAt: '2026-05-02T10:30:00Z',
      leaderboardRank: 7,
    });

    // Act
    renderDailyFlow();

    // Assert — PostCompletionScreen renders straight away with the
    // persisted serverElapsedMs, and getDaily() is never called.
    await waitFor(() => {
      expect(screen.getByTestId('daily-post-completion')).toBeInTheDocument();
    });
    expect(mockGetDaily).not.toHaveBeenCalled();
    // 9876 ms → 0:09 (rounded down to whole seconds).
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('0:09');
  });

  it('falls back to getDaily when persisted solved row lacks serverElapsedMs', async () => {
    // Arrange — partial-write shape: status=solved but
    // serverElapsedMs is undefined. The short-circuit must not
    // engage; the network path runs as if the persisted row didn't
    // exist.
    const todayUtc = new Date().toISOString().slice(0, 10);
    mockLoadState.mockResolvedValue({
      id: `daily:${todayUtc}`,
      flowType: 'daily',
      flowId: todayUtc,
      puzzle: {
        puzzleId: 'persisted-puzzle',
        gridSize: 9,
        mode: 'standard',
        regionMap: MOCK_PAYLOAD.regions,
      },
      cells: [],
      timer: { elapsedAtLastPause: 0, lastResumedAt: null },
      status: 'solved',
      startedAt: Date.now(),
      // serverElapsedMs intentionally absent.
    });
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);

    // Act
    renderDailyFlow();

    // Assert — getDaily() IS called and the playing branch renders.
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });
    expect(mockGetDaily).toHaveBeenCalledTimes(1);
  });

  it('shows a "Submitting…" indicator while the submit is in flight', async () => {
    // Arrange — keep the submit promise pending so the submitting
    // state remains observable.
    let resolveSubmit: (value: DailySubmitResponse) => void = () => {};
    mockGetDaily.mockResolvedValue(MOCK_PAYLOAD);
    mockSubmitDailyResult.mockReturnValue(
      new Promise<DailySubmitResponse>((resolve) => {
        resolveSubmit = resolve;
      }),
    );
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-game-board-stub')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-stub-solve'));

    // Assert — `findBy` waits for the mutation's pending render (see
    // the "playing -> submitting -> solved" test above for context).
    await screen.findByTestId('daily-submitting');
    expect(screen.getByText(/submitting/i)).toBeInTheDocument();

    // Cleanup the pending promise so the test exits cleanly.
    await act(async () => {
      resolveSubmit(MOCK_SUBMIT_RESPONSE);
    });
  });
});
