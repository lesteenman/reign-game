import { render, screen, cleanup, waitFor, fireEvent } from '../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../theme/ThemeContext';
import { DailyFlow } from './DailyFlow';
import { DailyApiError, type DailyPuzzlePayload } from '../services/dailyService';

// Mock the daily service so the component's data-fetch effect is
// observable. The default `mockResolvedValue` is replaced per test
// to drive each branch of DP-31's state machine.
const mockGetDaily = vi.fn();
vi.mock('../services/dailyService', async () => {
  const actual = await vi.importActual<typeof import('../services/dailyService')>(
    '../services/dailyService',
  );
  return {
    ...actual,
    getDaily: (date?: string) => mockGetDaily(date),
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
  mockGetDaily.mockReset();
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
      expect(screen.getByTestId('daily-loaded')).toBeInTheDocument();
    });
    expect(screen.getByTestId('daily-loaded')).toHaveTextContent(MOCK_PAYLOAD.puzzleId);
  });

  it('renders 404 error UI when getDaily throws DailyApiError with status 404', async () => {
    // Arrange
    mockGetDaily.mockRejectedValue(new DailyApiError('not found', 404));

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });
    expect(screen.getByText(/no daily available right now/i)).toBeInTheDocument();
    expect(screen.getByTestId('daily-retry')).toBeInTheDocument();
  });

  it('renders 500 error UI when getDaily throws DailyApiError with status 500', async () => {
    // Arrange
    mockGetDaily.mockRejectedValue(new DailyApiError('server boom', 500));

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
    mockGetDaily.mockRejectedValueOnce(new DailyApiError('boom', 500));
    mockGetDaily.mockResolvedValueOnce(MOCK_PAYLOAD);
    renderDailyFlow();
    await waitFor(() => {
      expect(screen.getByTestId('daily-error')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('daily-retry'));

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-loaded')).toBeInTheDocument();
    });
    expect(mockGetDaily).toHaveBeenCalledTimes(2);
  });

  it('renders the chunk-3 placeholder when outcome is solved', async () => {
    // Arrange — chunk 5 will replace this placeholder with the
    // post-completion screen. The chunk-3 stub just proves the
    // short-circuit branch renders.
    mockGetDaily.mockResolvedValue({ ...MOCK_PAYLOAD, outcome: 'solved' });

    // Act
    renderDailyFlow();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('daily-solved-placeholder')).toBeInTheDocument();
    });
  });
});
