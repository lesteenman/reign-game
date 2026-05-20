import { render, screen, cleanup, act } from '@shared/test-utils';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { GameBoard, type GameBoardProps } from './GameBoard';
import type { PuzzleData, CellState } from '@engine/types';
import { EMPTY_HISTORY } from '@storage/types';

// Clerk hook stub — GameBoard reads `useUser()` for role-gated UI; the
// wall-clock anchor logic is independent of role.
vi.mock('@clerk/react', () => ({
  useUser: () => ({
    isLoaded: true,
    isSignedIn: false,
    user: null,
  }),
}));

// puzzleService stub — GameBoard's solve effect calls updatePuzzleStatus
// in non-delegated mode. The tests below stay in delegated mode (onSolve-
// Detected wired), so this stub is defensive only.
vi.mock('@services/puzzleService', () => ({
  updatePuzzleStatus: vi.fn().mockResolvedValue(undefined),
  fetchNextPuzzle: vi.fn(),
  NoPuzzlesAvailableError: class extends Error {},
}));

// Verdict service stub — only invoked when the admin verdict surface is
// rendered; not exercised here.
vi.mock('@services/verdictService', () => ({
  submitVerdict: vi.fn().mockResolvedValue(undefined),
}));

const SIMPLE_5X5: PuzzleData = {
  puzzleId: 'wallclock-test',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [0, 3, 3, 2, 4],
    [3, 3, 2, 2, 4],
    [3, 3, 4, 4, 4],
  ],
};

function emptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from<CellState>({ length: size }).fill('empty'),
  );
}

function NavigateConsumer({ render: r }: { render: (nav: ReturnType<typeof useNavigate>) => React.ReactNode }) {
  const nav = useNavigate();
  return <>{r(nav)}</>;
}

function renderBoard(overrides: Partial<GameBoardProps> = {}) {
  // useNavigate must be called inside a Router subtree, so the inner
  // component grabs it and passes it back as a prop.
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <NavigateConsumer
          render={(navigate) => {
            const props: GameBoardProps = {
              puzzle: SIMPLE_5X5,
              flowType: 'daily',
              flowId: '2026-05-11',
              initialCells: emptyCells(5),
              initialHistory: EMPTY_HISTORY,
              timerElapsed: 0,
              timerResumedAt: null,
              startedAt: Date.now(),
              navigate,
              saveState: vi.fn().mockResolvedValue(undefined),
              clearState: vi.fn().mockResolvedValue(undefined),
              addCompletion: vi.fn().mockResolvedValue(undefined),
              onBack: vi.fn(),
              onPlayAgain: vi.fn(),
              onSolveDetected: vi.fn(),
              ...overrides,
            };
            return <GameBoard {...props} />;
          }}
        />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('GameBoard wall-clock-anchored timer', () => {
  beforeEach(() => {
    // Fake only the wall-clock and the interval timer used by the
    // wall-clock-tick effect. Leaving setTimeout/microtasks on real
    // timers keeps React's scheduler and the testing-library async
    // helpers (`findByTestId`, `act` flushes) functional.
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    vi.setSystemTime(new Date('2026-05-11T12:00:45Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('reads displayed elapsed from (Date.now() − assignedAt) when assignedAt is set', async () => {
    // Arrange — assignedAt is 45s in the past.
    const assignedAt = '2026-05-11T12:00:00Z';

    // Act
    renderBoard({ assignedAt });

    // Assert — display reflects the 45-second wall-clock delta.
    const timer = await screen.findByTestId('timer-display');
    expect(timer).toHaveTextContent('00:45');
  });

  it('does not reset to 00:00 when the board unmounts and remounts', async () => {
    // Arrange — assignedAt 30s in the past on first mount.
    const assignedAt = '2026-05-11T12:00:15Z'; // 30s before fake now

    // Act 1 — first mount
    const first = renderBoard({ assignedAt });
    const initial = await screen.findByTestId('timer-display');
    expect(initial).toHaveTextContent('00:30');

    // Act 2 — simulate navigate-away and back: unmount, advance wall
    // clock by 20s, mount again with the same assignedAt.
    first.unmount();
    vi.setSystemTime(new Date('2026-05-11T12:01:05Z')); // 50s after assignedAt
    renderBoard({ assignedAt });

    // Assert — elapsed reflects total wall-clock since assignedAt, not
    // a fresh 00:00.
    const remounted = await screen.findByTestId('timer-display');
    expect(remounted).toHaveTextContent('00:50');
  });

  it('ticks forward when the 1-second interval fires', async () => {
    // Arrange — assignedAt 3s in the past on initial mount.
    const assignedAt = '2026-05-11T12:00:42Z';

    // Act 1
    renderBoard({ assignedAt });
    const timer = await screen.findByTestId('timer-display');
    expect(timer).toHaveTextContent('00:03');

    // Act 2 — advance fake timers by 1s. `toFake: ['Date', 'setInterval', ...]`
    // advances Date in lockstep, so wall-clock elapsed becomes 4s and the
    // interval handler fires once, re-rendering with the new value.
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    // Assert
    expect(screen.getByTestId('timer-display')).toHaveTextContent('00:04');
  });
});
