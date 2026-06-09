import { render, screen, cleanup, fireEvent, waitFor } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { ApiError } from '@shared/api';

// The hook reloads the Clerk user on success; stub useUser so the
// reload() call is a no-op and the component renders without a real
// Clerk provider.
const reloadMock = vi.fn().mockResolvedValue(undefined);
vi.mock('@clerk/react', () => ({
  useUser: () => ({ user: { reload: reloadMock }, isLoaded: true }),
}));

// Mock the service so the mutation resolves/rejects deterministically.
const setUsernameMock = vi.fn();
vi.mock('@shared/profile/usernameService', () => ({
  setUsername: (name: string) => setUsernameMock(name),
}));

import { ClaimUsernamePrompt } from './ClaimUsernamePrompt';

beforeEach(() => {
  setUsernameMock.mockReset();
  reloadMock.mockClear();
});

afterEach(() => {
  cleanup();
});

describe('ClaimUsernamePrompt', () => {
  it('renders the claim prompt with input and submit', () => {
    // Arrange + Act
    render(<ClaimUsernamePrompt />);

    // Assert
    expect(screen.getByTestId('claim-username-prompt')).toBeInTheDocument();
    expect(screen.getByTestId('claim-username-input')).toBeInTheDocument();
  });

  it('shows success copy with the saved name on a successful claim', async () => {
    // Arrange
    setUsernameMock.mockResolvedValue('happyplayer');
    render(<ClaimUsernamePrompt />);

    // Act
    fireEvent.change(screen.getByTestId('claim-username-input'), {
      target: { value: 'happyplayer' },
    });
    fireEvent.click(screen.getByTestId('claim-username-submit'));

    // Assert
    await waitFor(() =>
      expect(screen.getByTestId('claim-username-success')).toHaveTextContent(
        'happyplayer',
      ),
    );
    expect(setUsernameMock).toHaveBeenCalledWith('happyplayer');
  });

  it.each([
    [400, /3-20 letters/i],
    [409, /already taken/i],
    [422, /isn't allowed/i],
  ])('renders error copy for status %i', async (status, pattern) => {
    // Arrange
    setUsernameMock.mockRejectedValue(new ApiError('nope', status));
    render(<ClaimUsernamePrompt />);

    // Act
    fireEvent.change(screen.getByTestId('claim-username-input'), {
      target: { value: 'somename' },
    });
    fireEvent.click(screen.getByTestId('claim-username-submit'));

    // Assert
    await waitFor(() =>
      expect(screen.getByTestId('claim-username-error')).toHaveTextContent(
        pattern,
      ),
    );
  });
});
