import { render, screen, cleanup, waitFor } from '@shared/test-utils';
import { fireEvent } from '@testing-library/react';
import { describe, it, test, expect, vi, beforeEach, afterEach } from 'vitest';

const submitVerdictMock = vi.fn();
const updatePuzzleStatusMock = vi.fn();

// Mock the hooks rather than the services: the hooks wrap services in
// useMutation, so mocking at the service layer requires waiting for
// TanStack's async mutation pipeline. Mocking the hooks gives direct
// control over mutateAsync's resolved/rejected value.
vi.mock('../hooks/useSubmitVerdict', () => ({
  useSubmitVerdict: () => ({ mutateAsync: (...args: unknown[]) => submitVerdictMock(...args) }),
}));

vi.mock('../hooks/useUpdatePuzzleStatus', () => ({
  useUpdatePuzzleStatus: () => ({ mutateAsync: (...args: unknown[]) => updatePuzzleStatusMock(...args) }),
}));

import { VerdictSurface } from './VerdictSurface';
import { ApiError } from '@services/api';

const baseProps = {
  puzzleId: 'puz-test-123',
  size: 7,
  mode: 'standard' as const,
  playTimeMs: 42_000,
};

beforeEach(() => {
  submitVerdictMock.mockReset();
  updatePuzzleStatusMock.mockReset();
  submitVerdictMock.mockResolvedValue(undefined);
  updatePuzzleStatusMock.mockResolvedValue(undefined);
  sessionStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe('VerdictSurface — completion variant', () => {
  test('renders "Good puzzle" + "Bad puzzle" buttons; no Cancel', () => {
    // Arrange + Act
    render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Assert
    expect(screen.getByRole('button', { name: 'Good puzzle' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Bad puzzle' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'I hate this' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Just skip' })).not.toBeInTheDocument();
  });

  test('click "Good puzzle" → submitVerdict({ value: "up", outcome: "solved" })', async () => {
    // Arrange
render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Good puzzle' }));

    // Assert
    await waitFor(() => {
      expect(submitVerdictMock).toHaveBeenCalledTimes(1);
    });
    expect(submitVerdictMock).toHaveBeenCalledWith(
      expect.objectContaining({
        puzzleId: 'puz-test-123',
        size: 7,
        mode: 'standard',
        value: 'up',
        playTimeMs: 42_000,
        outcome: 'solved',
      }),
    );
    expect(updatePuzzleStatusMock).not.toHaveBeenCalled();
  });

  test('click "Bad puzzle" → submitVerdict({ value: "down", outcome: "solved" })', async () => {
    // Arrange
render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Bad puzzle' }));

    // Assert
    await waitFor(() => {
      expect(submitVerdictMock).toHaveBeenCalledWith(
        expect.objectContaining({ value: 'down', outcome: 'solved' }),
      );
    });
  });

  test('success → done state shows "Thanks — recorded.", buttons gone', async () => {
    // Arrange
render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Good puzzle' }));

    // Assert
    await waitFor(() => {
      expect(screen.getByText(/thanks/i)).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: 'Good puzzle' })).not.toBeInTheDocument();
  });

  test('500 → error state with Retry; Retry re-invokes service with the previously-clicked value', async () => {
    // Arrange
submitVerdictMock.mockRejectedValueOnce(new ApiError('boom', 500));
    render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Bad puzzle' }));

    // Assert
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    });
    expect(screen.getByText(/couldn't save/i)).toBeInTheDocument();

    // Retry succeeds (default mock)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => {
      expect(submitVerdictMock).toHaveBeenCalledTimes(2);
    });
    expect(submitVerdictMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ value: 'down', outcome: 'solved' }),
    );
    await waitFor(() => {
      expect(screen.getByText(/thanks/i)).toBeInTheDocument();
    });
  });

  test('submitting state disables both verdict buttons', async () => {
    // Arrange — never-resolving submit so we can observe the submitting state
    submitVerdictMock.mockImplementation(() => new Promise(() => {}));
render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Good puzzle' }));

    // Assert
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Good puzzle' })).toBeDisabled();
    });
    expect(screen.getByRole('button', { name: 'Bad puzzle' })).toBeDisabled();
  });

  test('sessionStorage cache: pre-seed → component renders null (FB-07)', () => {
    // Arrange
    sessionStorage.setItem(
      'reign:verdict:submitted',
      JSON.stringify(['puz-test-123']),
    );

    // Act
    const { container } = render(
      <VerdictSurface variant="completion" outcome="solved" {...baseProps} />,
    );

    // Assert
    expect(container.firstChild).toBeNull();
  });
});

describe('VerdictSurface — skip variant', () => {
  test('renders Cancel + I hate this + Just skip; no Good/Bad', () => {
    // Arrange + Act
    render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={() => {}}
      />,
    );

    // Assert
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'I hate this' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Just skip' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Good puzzle' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Bad puzzle' })).not.toBeInTheDocument();
  });

  test('Cancel → calls onDismiss; no API calls', async () => {
    // Arrange
const onDismiss = vi.fn();
    render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={onDismiss}
        onAfterVerdict={() => {}}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    // Assert
    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(submitVerdictMock).not.toHaveBeenCalled();
    expect(updatePuzzleStatusMock).not.toHaveBeenCalled();
  });

  test('"I hate this" → calls updatePuzzleStatus AND submitVerdict (parallel) AND onAfterVerdict', async () => {
    // Arrange
const onAfterVerdict = vi.fn();
    render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={onAfterVerdict}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'I hate this' }));

    // Assert — both services called
    await waitFor(() => {
      expect(updatePuzzleStatusMock).toHaveBeenCalledTimes(1);
      expect(submitVerdictMock).toHaveBeenCalledTimes(1);
    });
    expect(updatePuzzleStatusMock).toHaveBeenCalledWith(
      expect.objectContaining({
        puzzleId: 'puz-test-123',
        size: 7,
        mode: 'standard',
        status: 'skipped',
      }),
    );
    expect(submitVerdictMock).toHaveBeenCalledWith(
      expect.objectContaining({
        puzzleId: 'puz-test-123',
        value: 'down',
        outcome: 'skipped',
      }),
    );
    await waitFor(() => {
      expect(onAfterVerdict).toHaveBeenCalledTimes(1);
    });
  });

  test('"Just skip" → calls only updatePuzzleStatus AND onAfterVerdict; no verdict written', async () => {
    // Arrange
const onAfterVerdict = vi.fn();
    render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={onAfterVerdict}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Just skip' }));

    // Assert
    await waitFor(() => {
      expect(updatePuzzleStatusMock).toHaveBeenCalledTimes(1);
    });
    expect(submitVerdictMock).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(onAfterVerdict).toHaveBeenCalledTimes(1);
    });
  });

  test('500 on "I hate this" → error state with Retry; Retry re-invokes both calls', async () => {
    // Arrange
updatePuzzleStatusMock.mockRejectedValueOnce(new ApiError('boom', 500));
    const onAfterVerdict = vi.fn();
    render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={onAfterVerdict}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'I hate this' }));

    // Assert
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    });
    expect(onAfterVerdict).not.toHaveBeenCalled();

    // Retry — both mocks now succeed
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => {
      expect(onAfterVerdict).toHaveBeenCalledTimes(1);
    });
  });

  test('submitting state disables ALL three skip buttons (incl. Cancel)', async () => {
    // Arrange
    updatePuzzleStatusMock.mockImplementation(() => new Promise(() => {}));
render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={() => {}}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Just skip' }));

    // Assert
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    });
    expect(screen.getByRole('button', { name: 'I hate this' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Just skip' })).toBeDisabled();
  });

  test('sessionStorage cache: pre-seed → skip variant renders null', () => {
    // Arrange
    sessionStorage.setItem(
      'reign:verdict:submitted',
      JSON.stringify(['puz-test-123']),
    );

    // Act
    const { container } = render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={() => {}}
      />,
    );

    // Assert
    expect(container.firstChild).toBeNull();
  });
});

describe('VerdictSurface — successful submission marks puzzleId in sessionStorage cache', () => {
  it('completion variant adds the puzzleId to reign:verdict:submitted on Good click', async () => {
    // Arrange
render(<VerdictSurface variant="completion" outcome="solved" {...baseProps} />);

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Good puzzle' }));
    await waitFor(() => {
      expect(screen.getByText(/thanks/i)).toBeInTheDocument();
    });

    // Assert
    const stored = JSON.parse(
      sessionStorage.getItem('reign:verdict:submitted') ?? '[]',
    );
    expect(stored).toContain('puz-test-123');
  });

  it('skip variant adds the puzzleId on "Just skip" success', async () => {
    // Arrange
render(
      <VerdictSurface
        variant="skip"
        outcome="skipped"
        {...baseProps}
        onDismiss={() => {}}
        onAfterVerdict={() => {}}
      />,
    );

    // Act
    fireEvent.click(screen.getByRole('button', { name: 'Just skip' }));
    await waitFor(() => {
      const stored = JSON.parse(
        sessionStorage.getItem('reign:verdict:submitted') ?? '[]',
      );
      expect(stored).toContain('puz-test-123');
    });
  });
});
