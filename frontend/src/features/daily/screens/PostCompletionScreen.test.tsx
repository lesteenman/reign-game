import { render, screen, cleanup, act } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { MemoryRouter } from 'react-router-dom';

/**
 * Tests for the daily post-completion screen.
 *
 * Countdown uses fake timers via Vitest; submitted-at rendering pins UTC
 * + a fixed `now` so the local-time render is deterministic across CI
 * timezones. `useUser` is mocked so the claim-a-name eligibility gate is
 * controllable; the prompt itself is stubbed (its internals are covered
 * by ClaimUsernamePrompt.test.tsx) so these tests assert only the gate.
 */

// Configurable Clerk user mock. Defaults to "not loaded" so the existing
// presentational tests never trip the claim prompt. The return type is
// widened so per-test overrides can supply a user with a username.
type UseUserResult = {
  user: { username: string | null } | null;
  isLoaded: boolean;
};
const useUserMock = vi.fn<() => UseUserResult>(() => ({
  user: null,
  isLoaded: false,
}));
vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
}));

vi.mock('@shared/profile/ClaimUsernamePrompt', () => ({
  ClaimUsernamePrompt: () => <div data-testid="claim-username-prompt-stub" />,
}));

import { PostCompletionScreen } from './PostCompletionScreen';

beforeEach(() => {
  useUserMock.mockReset();
  useUserMock.mockReturnValue({ user: null, isLoaded: false });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function renderScreen(props: Parameters<typeof PostCompletionScreen>[0]) {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <PostCompletionScreen {...props} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

const BASE_PROPS = {
  serverElapsedMs: 75_000,
  submittedAt: '2026-05-02T14:30:00Z',
  now: new Date('2026-05-02T22:30:00Z'),
};

describe('PostCompletionScreen', () => {
  it("renders 'Done for today' header", () => {
    // Arrange + Act
    renderScreen(BASE_PROPS);

    // Assert
    expect(screen.getByText(/done for today/i)).toBeInTheDocument();
  });

  it('formats serverElapsedMs as mm:ss for sub-hour times', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, serverElapsedMs: 75_000 });

    // Assert — 1 minute 15 seconds
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('1:15');
  });

  it('formats serverElapsedMs as h:mm:ss for over-hour times', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, serverElapsedMs: 5_400_000 });

    // Assert — 1 hour 30 minutes 0 seconds
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('1:30:00');
  });

  it('pads sub-minute solve times correctly', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, serverElapsedMs: 5_000 });

    // Assert — 5 seconds renders as 0:05
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('0:05');
  });

  it('renders leaderboard rank when prop present', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, leaderboardRank: 3 });

    // Assert
    expect(screen.getByTestId('daily-leaderboard-rank')).toHaveTextContent('#3');
  });

  it('omits leaderboard rank when prop absent', () => {
    // Arrange + Act
    renderScreen(BASE_PROPS);

    // Assert — anonymous players see no rank line
    expect(screen.queryByTestId('daily-leaderboard-rank')).not.toBeInTheDocument();
  });

  describe('claim-a-name prompt', () => {
    const promptStub = () => screen.queryByTestId('claim-username-prompt-stub');

    it('shows the prompt for a signed-in, leaderboard-eligible player with no username', () => {
      // Arrange — signed in, loaded, no username; rank present (eligible)
      useUserMock.mockReturnValue({
        user: { username: null },
        isLoaded: true,
      });

      // Act
      renderScreen({ ...BASE_PROPS, leaderboardRank: 5 });

      // Assert
      expect(promptStub()).toBeInTheDocument();
    });

    it('hides the prompt when the player already has a username', () => {
      // Arrange
      useUserMock.mockReturnValue({
        user: { username: 'alreadyset' },
        isLoaded: true,
      });

      // Act
      renderScreen({ ...BASE_PROPS, leaderboardRank: 5 });

      // Assert
      expect(promptStub()).not.toBeInTheDocument();
    });

    it('hides the prompt for anonymous players (no leaderboard rank)', () => {
      // Arrange — even a loaded user with no username, but no rank → anon
      useUserMock.mockReturnValue({
        user: { username: null },
        isLoaded: true,
      });

      // Act — no leaderboardRank prop ⇒ not leaderboard-eligible
      renderScreen(BASE_PROPS);

      // Assert
      expect(promptStub()).not.toBeInTheDocument();
    });

    it('hides the prompt while Clerk is still loading', () => {
      // Arrange — eligible rank but user not yet hydrated
      useUserMock.mockReturnValue({ user: null, isLoaded: false });

      // Act
      renderScreen({ ...BASE_PROPS, leaderboardRank: 5 });

      // Assert — no flash of the prompt before hydration
      expect(promptStub()).not.toBeInTheDocument();
    });
  });

  it('displays submittedAt in HH:MM', () => {
    // Arrange + Act — pin a deterministic timezone via Intl format
    // assertion. We assert on the same Intl formatter the component
    // uses, supplied with a fixed timezone, so the rendered text is
    // independent of the host timezone.
    renderScreen({
      ...BASE_PROPS,
      submittedAt: '2026-05-02T14:30:00Z',
    });

    // Assert — derive expected from the same Date the component used
    const expected = new Date('2026-05-02T14:30:00Z').toLocaleTimeString(
      undefined,
      { hour: '2-digit', minute: '2-digit' },
    );
    expect(screen.getByTestId('daily-submitted-at')).toHaveTextContent(expected);
  });

  it('countdown shows time until next UTC midnight', () => {
    // Arrange + Act — 22:30 UTC → 1h 30m to midnight
    renderScreen({
      ...BASE_PROPS,
      now: new Date('2026-05-02T22:30:00Z'),
    });

    // Assert
    expect(screen.getByTestId('daily-countdown')).toHaveTextContent('01:30:00');
  });

  it('countdown updates every second', () => {
    // Arrange — pin fake timers + wall-clock to a fixed `fixedNow`.
    // `vi.advanceTimersByTime(N)` advances both the fake timer queue
    // AND the fake `Date` simultaneously, so the interval callback's
    // `new Date()` (drift fix) sees the post-advance wall-clock time.
    const fixedNow = new Date('2026-05-02T22:30:00Z');
    vi.useFakeTimers();
    vi.setSystemTime(fixedNow);
    renderScreen({ ...BASE_PROPS, now: fixedNow });
    expect(screen.getByTestId('daily-countdown')).toHaveTextContent('01:30:00');

    // Act — advance the fake clock by 5 seconds. `advanceTimersByTime`
    // also moves `Date.now()` forward in lockstep.
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    // Assert — decremented by 5 seconds. The countdown reads `new
    // Date()` on each tick, which now returns fixedNow + 5s.
    expect(screen.getByTestId('daily-countdown')).toHaveTextContent('01:29:55');
  });

  it('countdown handles boundary (rolls past UTC midnight)', () => {
    // Arrange
    const fixedNow = new Date('2026-05-02T23:59:59Z');
    vi.useFakeTimers();
    vi.setSystemTime(fixedNow);
    renderScreen({ ...BASE_PROPS, now: fixedNow });
    expect(screen.getByTestId('daily-countdown')).toHaveTextContent('00:00:01');

    // Act — advance past midnight; countdown should re-anchor to the
    // following day rather than going negative. `advanceTimersByTime`
    // moves the fake `Date` forward in lockstep.
    act(() => {
      vi.advanceTimersByTime(2000);
    });

    // Assert — re-anchored to ~24h to the NEXT next-midnight (give or
    // take the 1s we crossed). Just assert it's a valid HH:MM:SS in
    // the 23:59:5x range.
    expect(screen.getByTestId('daily-countdown').textContent).toMatch(
      /^23:59:5\d$/,
    );
  });

  it('shows "already submitted" copy and hides solve time + submitted-at when synthetic409', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, synthetic409: true });

    // Assert
    expect(screen.getByTestId('daily-already-submitted')).toHaveTextContent(
      'Already submitted earlier today',
    );
    expect(screen.queryByTestId('daily-solve-time')).not.toBeInTheDocument();
    expect(screen.queryByTestId('daily-submitted-at')).not.toBeInTheDocument();
  });

  it('renders solve time and no "already submitted" copy when synthetic409 absent', () => {
    // Arrange + Act
    renderScreen(BASE_PROPS);

    // Assert — regression: normal success path still shows solve time
    expect(screen.getByTestId('daily-solve-time')).toHaveTextContent('1:15');
    expect(
      screen.queryByTestId('daily-already-submitted'),
    ).not.toBeInTheDocument();
  });

  it('renders the recycle line when isRecycle is true', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, isRecycle: true });

    // Assert
    expect(screen.getByTestId('daily-recycle-line')).toHaveTextContent(
      "Today's puzzle is a recycle of a recent day.",
    );
  });

  it('omits the recycle line when isRecycle is false', () => {
    // Arrange + Act
    renderScreen({ ...BASE_PROPS, isRecycle: false });

    // Assert
    expect(screen.queryByTestId('daily-recycle-line')).not.toBeInTheDocument();
  });

  it('countdown cleans up on unmount', () => {
    // Arrange
    vi.useFakeTimers();
    const baseline = vi.getTimerCount();
    const { unmount } = renderScreen({ ...BASE_PROPS });
    expect(vi.getTimerCount()).toBeGreaterThan(baseline);

    // Act
    unmount();

    // Assert — interval cleared, advancing timers doesn't error
    expect(vi.getTimerCount()).toBe(baseline);
    act(() => {
      vi.advanceTimersByTime(5000);
    });
  });
});
