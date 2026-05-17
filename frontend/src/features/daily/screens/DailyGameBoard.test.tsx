import { render, screen, cleanup, waitFor } from '../../../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../../../theme/ThemeContext';
import { DailyGameBoard } from './DailyGameBoard';
import type { DailyPuzzlePayload } from '../../../shared/types/daily';
import type { GameBoardProps } from '../../../shared/game/components/GameBoard';
import type { GameState } from '../../../storage/types';
import type { CellState } from '../../../engine/types';

// Storage stub — DailyGameBoard reads `loadState('daily', flowId)` on
// mount. Keeping the mock here means the test asserts the call shape
// without standing up fake-indexeddb.
const mockSaveState = vi.fn(async () => {});
const mockLoadState = vi.fn<(...args: [unknown, unknown]) => Promise<GameState | null>>(async () => null);
const mockClearState = vi.fn(async () => {});
const mockAddCompletion = vi.fn(async () => {});
vi.mock('../../../hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    saveState: mockSaveState,
    loadState: mockLoadState,
    clearState: mockClearState,
    addCompletion: mockAddCompletion,
  }),
}));

// GameBoard stub — DailyGameBoard's contract with GameBoard is the
// `onSolveDetected` opt-in delegate. The stub renders the props it
// received (so the adapter mapping is observable) and exposes a
// button that fires the solve callback with a fixed payload.
const mockGameBoardProps = vi.fn();
const STUB_SOLUTION = [
  [1, 0, 0],
  [0, 1, 0],
];
const STUB_ELAPSED_MS = 8_000;
vi.mock('../../../shared/game/components/GameBoard', async () => {
  const actual = await vi.importActual<typeof import('../../../shared/game/components/GameBoard')>('../../../shared/game/components/GameBoard');
  return {
    ...actual,
    GameBoard: (props: GameBoardProps) => {
      mockGameBoardProps(props);
      return (
        <div data-testid="gameboard-stub">
          <span data-testid="gameboard-puzzle-id">{props.puzzle.puzzleId}</span>
          <span data-testid="gameboard-grid-size">{props.puzzle.gridSize}</span>
          <span data-testid="gameboard-mode">{props.puzzle.mode}</span>
          <span data-testid="gameboard-flow-type">{props.flowType}</span>
          <span data-testid="gameboard-flow-id">{props.flowId}</span>
          <button
            data-testid="gameboard-fire-solve"
            onClick={() =>
              props.onSolveDetected?.(STUB_SOLUTION, STUB_ELAPSED_MS)
            }
          >
            Fire solve
          </button>
        </div>
      );
    },
  };
});

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

beforeEach(() => {
  mockSaveState.mockReset();
  mockSaveState.mockResolvedValue(undefined);
  mockLoadState.mockReset();
  mockLoadState.mockResolvedValue(null);
  mockClearState.mockReset();
  mockClearState.mockResolvedValue(undefined);
  mockAddCompletion.mockReset();
  mockAddCompletion.mockResolvedValue(undefined);
  mockGameBoardProps.mockReset();
});

afterEach(() => {
  cleanup();
});

function renderBoard(onSolved = vi.fn()) {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <DailyGameBoard payload={MOCK_PAYLOAD} onSolved={onSolved} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('DailyGameBoard', () => {
  it('renders GameBoard with PuzzleData adapted from DailyPuzzlePayload', async () => {
    // Arrange — default storage mock returns null (no restore).

    // Act
    renderBoard();

    // Assert — restore lookup resolves first, then GameBoard renders
    // with the adapted PuzzleData.
    await waitFor(() => {
      expect(screen.getByTestId('gameboard-stub')).toBeInTheDocument();
    });
    expect(screen.getByTestId('gameboard-puzzle-id')).toHaveTextContent(
      'daily-2026-05-02',
    );
    expect(screen.getByTestId('gameboard-grid-size')).toHaveTextContent('9');
    expect(screen.getByTestId('gameboard-mode')).toHaveTextContent('standard');
    expect(screen.getByTestId('gameboard-flow-type')).toHaveTextContent('daily');
    expect(screen.getByTestId('gameboard-flow-id')).toHaveTextContent(
      '2026-05-02',
    );
  });

  it('reads the per-flow IDB row on mount with (daily, YYYY-MM-DD)', async () => {
    // Arrange

    // Act
    renderBoard();

    // Assert
    await waitFor(() => {
      expect(mockLoadState).toHaveBeenCalledWith('daily', '2026-05-02');
    });
  });

  it('restores in-progress cells/timer from persisted state on mount', async () => {
    // Arrange — seed an in-progress row; DailyGameBoard should pass
    // the saved cells / timer into GameBoard rather than the empty
    // defaults.
    const restoredCells: CellState[][] = Array.from({ length: 9 }, () =>
      Array.from<CellState>({ length: 9 }).fill('empty'),
    );
    restoredCells[0]![0] = 'marked';
    mockLoadState.mockResolvedValue({
      id: 'daily:2026-05-02',
      flowType: 'daily',
      flowId: '2026-05-02',
      puzzle: {
        puzzleId: 'daily-2026-05-02',
        gridSize: 9,
        mode: 'standard',
        regionMap: MOCK_PAYLOAD.regions,
      },
      cells: restoredCells,
      timer: { elapsedAtLastPause: 4_321, lastResumedAt: null },
      status: 'in-progress',
      startedAt: 1_700_000_000_000,
      history: { past: [], future: [] },
    });

    // Act
    renderBoard();

    // Assert
    await waitFor(() => {
      expect(mockGameBoardProps).toHaveBeenCalled();
    });
    const calls = mockGameBoardProps.mock.calls;
    const props = calls[calls.length - 1]![0] as GameBoardProps;
    expect(props.initialCells[0]![0]).toBe('marked');
    expect(props.timerElapsed).toBe(4_321);
    expect(props.startedAt).toBe(1_700_000_000_000);
  });

  it('forwards payload.assignedAt to GameBoard as the wall-clock anchor', async () => {
    // Arrange — default storage mock returns null (no restore).

    // Act
    renderBoard();
    await waitFor(() => {
      expect(mockGameBoardProps).toHaveBeenCalled();
    });

    // Assert
    const calls = mockGameBoardProps.mock.calls;
    const props = calls[calls.length - 1]![0] as GameBoardProps;
    expect(props.assignedAt).toBe('2026-05-02T00:00:00Z');
  });

  it('forwards GameBoard onSolveDetected to props.onSolved with (solution, elapsedMs)', async () => {
    // Arrange
    const onSolved = vi.fn();
    renderBoard(onSolved);
    await waitFor(() => {
      expect(screen.getByTestId('gameboard-stub')).toBeInTheDocument();
    });

    // Act — synthesize the GameBoard solve event.
    screen.getByTestId('gameboard-fire-solve').click();

    // Assert
    expect(onSolved).toHaveBeenCalledTimes(1);
    expect(onSolved).toHaveBeenCalledWith(STUB_SOLUTION, STUB_ELAPSED_MS);
  });
});
