import 'fake-indexeddb/auto';
import { render, screen, cleanup, waitFor } from '@shared/test-utils';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { GameBoard } from './GameBoard';
import { FALLBACK_PUZZLE } from '@shared/test-fixtures';
import { EMPTY_HISTORY } from '@storage/types';
import type { CellState } from '@engine/types';

// Mock the Clerk hook so admin/non-admin branches are exercisable
// without booting Clerk. Default to signed-out; tests override per case.
type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};
const useUserMock = vi.fn<() => UseUserReturn>();
vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
  Show: () => null,
  SignInButton: () => null,
  UserButton: () => null,
}));

const mockUpdateStatus = vi.fn().mockResolvedValue(undefined);
vi.mock('@services/puzzleService', () => ({
  updatePuzzleStatus: (...args: unknown[]) => mockUpdateStatus(...args),
}));

// We don't care about VerdictSurface internals here — stub it.
vi.mock('./VerdictSurface', () => ({
  VerdictSurface: () => null,
}));

const emptyGrid5 = (): CellState[][] =>
  Array.from({ length: 5 }, () => Array<CellState>(5).fill('empty'));

beforeEach(() => {
  useUserMock.mockReset();
  // Default admin so the Skip-gating test starts from the admin-shows
  // baseline. The curation case in GamePage.test.tsx covers the
  // non-admin path.
  useUserMock.mockReturnValue({
    isLoaded: true,
    isSignedIn: true,
    user: { publicMetadata: { role: 'admin' } },
  });
  mockUpdateStatus.mockReset();
  mockUpdateStatus.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
});

function Harness({
  flowType,
  onSolveDetected,
}: {
  flowType: 'curation' | 'daily';
  onSolveDetected?: (s: number[][], ms: number) => void;
}) {
  const navigate = useNavigate();
  return (
    <GameBoard
      puzzle={FALLBACK_PUZZLE}
      flowType={flowType}
      flowId={flowType === 'daily' ? '2026-05-11' : '5x5-standard'}
      initialCells={emptyGrid5()}
      initialHistory={EMPTY_HISTORY}
      timerElapsed={0}
      timerResumedAt={null}
      startedAt={Date.now()}
      navigate={navigate}
      saveState={async () => {}}
      clearState={async () => {}}
      addCompletion={async () => {}}
      onBack={() => {}}
      onPlayAgain={() => {}}
      onSolveDetected={onSolveDetected}
    />
  );
}

function renderHarness(
  flowType: 'curation' | 'daily',
  onSolveDetected?: (s: number[][], ms: number) => void,
) {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <Harness flowType={flowType} onSolveDetected={onSolveDetected} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('GameBoard — daily flow hides Skip button (admin)', () => {
  it('does NOT render the Skip button when flowType is daily, even for an admin', async () => {
    // Arrange — admin Clerk state is set in beforeEach. flowType=daily
    // gates the admin verdict surface OFF: Skip belongs to curation.

    // Act
    renderHarness('daily', () => {});

    // Assert
    await screen.findByTestId('reset-button');
    expect(screen.queryByTestId('skip-button')).not.toBeInTheDocument();
  });

  it('renders the Skip button when flowType is curation and user is admin', async () => {
    // Arrange — admin already set in beforeEach.

    // Act
    renderHarness('curation');

    // Assert
    expect(await screen.findByTestId('skip-button')).toBeInTheDocument();
  });
});

describe('GameBoard — delegated mode does not render its own PageShell', () => {
  it('does NOT render a PageShell header when onSolveDetected is provided', async () => {
    // Arrange — delegated mode (e.g. DailyFlow caller). The caller
    // owns the page chrome; rendering a second PageShell would stack
    // two Reign headers.

    // Act
    renderHarness('daily', () => {});

    // Assert — wait for the grid to render, then probe for the page
    // header. There must be no page-header rendered by GameBoard in
    // delegated mode.
    await screen.findByTestId('reset-button');
    expect(screen.queryByTestId('page-header')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /reign/i })).not.toBeInTheDocument();
  });

  it('DOES render a PageShell when onSolveDetected is undefined (standalone curation/practice)', async () => {
    // Arrange — standalone mode; GameBoard is the page.

    // Act
    renderHarness('curation', undefined);

    // Assert
    await screen.findByTestId('reset-button');
    expect(screen.getByTestId('page-header')).toBeInTheDocument();
  });
});

describe('GameBoard — timer starts on mount, not on first tap', () => {
  it('starts the timer as soon as the grid renders (lastResumedAt is non-null)', async () => {
    // Arrange — fresh puzzle, no pointer events fired.
    let lastSavedTimer: { elapsedAtLastPause: number; lastResumedAt: number | null } | null = null;
    function CaptureHarness() {
      const navigate = useNavigate();
      return (
        <GameBoard
          puzzle={FALLBACK_PUZZLE}
          flowType="curation"
          flowId="5x5-standard"
          initialCells={emptyGrid5()}
          initialHistory={EMPTY_HISTORY}
          timerElapsed={0}
          timerResumedAt={null}
          startedAt={Date.now()}
          navigate={navigate}
          saveState={async (s) => {
            lastSavedTimer = s.timer;
          }}
          clearState={async () => {}}
          addCompletion={async () => {}}
          onBack={() => {}}
          onPlayAgain={() => {}}
        />
      );
    }
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });

    // Act
    render(
      <ThemeProvider>
        <MemoryRouter>
          <CaptureHarness />
        </MemoryRouter>
      </ThemeProvider>,
    );

    // Assert — a debounced save fires; the captured timer state must
    // show the timer is running (lastResumedAt !== null).
    await waitFor(() => {
      expect(lastSavedTimer).not.toBeNull();
      expect(lastSavedTimer!.lastResumedAt).not.toBeNull();
    }, { timeout: 1500 });
  });
});
