import { render, screen, cleanup, waitFor } from '../test-utils';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockFetchEnabledModes = vi
  .fn<
    () => Promise<
      Array<{ size: number; mode: 'standard' | 'double'; threshold: number; enabled: true }>
    >
  >()
  .mockResolvedValue([
    { size: 7, mode: 'standard', threshold: 3, enabled: true },
    { size: 9, mode: 'double', threshold: 3, enabled: true },
  ]);

vi.mock('../services/landingService', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../services/landingService')>();
  return {
    ...actual,
    fetchEnabledModes: () => mockFetchEnabledModes(),
  };
});

import { CurationPage } from './CurationPage';

beforeEach(() => {
  mockNavigate.mockClear();
  mockFetchEnabledModes.mockClear();
  mockFetchEnabledModes.mockResolvedValue([
    { size: 7, mode: 'standard', threshold: 3, enabled: true },
    { size: 9, mode: 'double', threshold: 3, enabled: true },
  ]);
});

afterEach(() => {
  cleanup();
});

function renderCurationPage() {
  return render(
    <MemoryRouter>
      <CurationPage />
    </MemoryRouter>,
  );
}

describe('CurationPage', () => {
  it('renders a Curation heading and a Settings button', async () => {
    // Arrange + Act
    renderCurationPage();

    // Assert
    expect(await screen.findByText('Curation')).toBeInTheDocument();
    expect(screen.getByTestId('curation-settings-button')).toBeInTheDocument();
  });

  it('Settings button navigates to /admin', async () => {
    // Arrange
    renderCurationPage();
    const settings = await screen.findByTestId('curation-settings-button');

    // Act
    fireEvent.click(settings);

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/admin');
  });

  it('renders the PuzzleSelector with the enabled modes from the backend', async () => {
    // Arrange + Act
    renderCurationPage();

    // Assert — PuzzleSelector renders one button per combo.
    await waitFor(() => {
      expect(screen.getByText(/7×7 Standard/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/9×9 Double Queens/i)).toBeInTheDocument();
  });

  it('selecting a pool navigates to /play with size + mode params', async () => {
    // Arrange
    renderCurationPage();
    await waitFor(() => {
      expect(screen.getByText(/7×7 Standard/i)).toBeInTheDocument();
    });

    // Act — click on the second combo (9x9 Double) so we're not just
    // confirming the default selection.
    fireEvent.click(screen.getByText(/9×9 Double Queens/i));
    const playButton = screen.getByRole('button', { name: /play/i });
    fireEvent.click(playButton);

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith(
      expect.stringContaining('/play?'),
    );
    const playUrl = mockNavigate.mock.calls.find(
      (c) => typeof c[0] === 'string' && (c[0] as string).startsWith('/play?'),
    )?.[0] as string;
    expect(playUrl).toContain('flow=curation');
    expect(playUrl).toContain('size=9');
    expect(playUrl).toContain('mode=double');
    expect(playUrl).not.toContain('new=true');
  });

  it('falls through to an empty state when fetchEnabledModes rejects', async () => {
    // Arrange
    mockFetchEnabledModes.mockRejectedValueOnce(new Error('network fail'));

    // Act
    renderCurationPage();

    // Assert — rather than throwing, the page should render its
    // empty-modes state. The PuzzleSelector handles the empty-array
    // case internally; we just verify the page didn't crash.
    await waitFor(() => {
      expect(screen.getByText('Curation')).toBeInTheDocument();
    });
  });
});
