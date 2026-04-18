import { render, screen, fireEvent, waitFor, cleanup } from '../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ThemeProvider } from '../theme/ThemeContext';
import { AdminPage } from './AdminPage';

// Mock admin service
const mockFetchPoolStatus = vi.fn();
const mockUpdateConfig = vi.fn();
const mockCreateConfig = vi.fn();
const mockTriggerReplenish = vi.fn();

vi.mock('../services/adminService', () => ({
  fetchPoolStatus: (...args: unknown[]) => mockFetchPoolStatus(...args),
  updateConfig: (...args: unknown[]) => mockUpdateConfig(...args),
  createConfig: (...args: unknown[]) => mockCreateConfig(...args),
  triggerReplenish: (...args: unknown[]) => mockTriggerReplenish(...args),
}));

// Track navigation
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => (_path: string) => { /* noop for tests */ },
  };
});

const MOCK_POOL = {
  combos: [
    {
      size: 5,
      mode: 'standard',
      config: {
        pipeline: 'region-first',
        solver: 'backtrack',
        regions: 'bfs',
        regionVariance: 0.3,
        deducible: true,
        concurrency: 2,
        threshold: 3,
        enabled: true,
      },
      readyCount: 2,
    },
    {
      size: 7,
      mode: 'double',
      config: {
        pipeline: 'iterative',
        solver: 'propagation',
        regions: 'wfc',
        regionVariance: 0.5,
        deducible: false,
        concurrency: 4,
        threshold: 5,
        enabled: false,
      },
      readyCount: 0,
    },
  ],
};

beforeEach(() => {
  mockFetchPoolStatus.mockResolvedValue(MOCK_POOL);
  mockTriggerReplenish.mockResolvedValue({ triggered: [{ size: 5, mode: 'standard', count: 1 }], skipped: [] });
  mockUpdateConfig.mockResolvedValue(MOCK_POOL.combos[0]!.config);
  mockCreateConfig.mockResolvedValue(MOCK_POOL.combos[0]!.config);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderAdmin() {
  return render(
    <ThemeProvider>
      <MemoryRouter initialEntries={['/admin']}>
        <AdminPage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('AdminPage', () => {
  it('shows loading state initially', () => {
    // Arrange
    mockFetchPoolStatus.mockReturnValue(new Promise(() => {})); // never resolves

    // Act
    renderAdmin();

    // Assert
    expect(screen.getByTestId('loading')).toBeInTheDocument();
    expect(screen.getByText('Loading pool status...')).toBeInTheDocument();
  });

  it('renders pool table with combo data after fetch', async () => {
    // Act (arrange is the default mocks from beforeEach)
    renderAdmin();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('pool-table')).toBeInTheDocument();
    });
    expect(screen.getByText('5x5 Standard')).toBeInTheDocument();
    expect(screen.getByText('7x7 Double Queens')).toBeInTheDocument();
    expect(screen.getByText('2 / 3')).toBeInTheDocument();
    expect(screen.getByText('0 / 5')).toBeInTheDocument();
  });

  it('Replenish All button triggers API call', async () => {
    // Arrange
    renderAdmin();
    await waitFor(() => {
      expect(screen.getByTestId('pool-table')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('replenish-all'));

    // Assert
    await waitFor(() => {
      expect(mockTriggerReplenish).toHaveBeenCalledWith();
    });
  });

  it('Edit button opens config form', async () => {
    // Arrange
    renderAdmin();
    await waitFor(() => {
      expect(screen.getByTestId('pool-table')).toBeInTheDocument();
    });

    // Act
    fireEvent.click(screen.getByTestId('edit-5-standard'));

    // Assert
    expect(screen.getByTestId('config-form')).toBeInTheDocument();
    expect(screen.getByText('Edit Config')).toBeInTheDocument();
  });

  it('shows error state when fetch fails', async () => {
    // Arrange
    mockFetchPoolStatus.mockRejectedValue(new Error('Network error'));

    // Act
    renderAdmin();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });
    expect(screen.getByText('Network error')).toBeInTheDocument();
  });

  it('shows Pool Management heading', async () => {
    // Act (arrange is the default mocks from beforeEach)
    renderAdmin();

    // Assert
    await waitFor(() => {
      expect(screen.getByText('Pool Management')).toBeInTheDocument();
    });
  });

  it('shows Add Combo button after loading', async () => {
    // Act (arrange is the default mocks from beforeEach)
    renderAdmin();

    // Assert
    await waitFor(() => {
      expect(screen.getByTestId('add-combo')).toBeInTheDocument();
    });
  });
});
