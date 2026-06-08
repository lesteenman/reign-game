import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { styled, Text, View } from 'tamagui';
import { PageShell } from '@shared/components/PageShell';
import { PrimaryButton, SecondaryButton } from '@shared/components/Button';
import { Card } from '@shared/components/Card';
import { Spinner } from '@shared/components/Spinner';
import { ApiError } from '@shared/api';
import { useDailyPuzzle } from '@features/daily/hooks/useDailyPuzzle';
import { useSubmitDaily } from '@features/daily/hooks/useSubmitDaily';
import { DailyGameBoard } from './DailyGameBoard';
import { PostCompletionScreen } from './PostCompletionScreen';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

// Centered column for the loading + submitting states. NOT a card (no
// border/shadow — these are transient indicators while a network
// round-trip is in flight). Layout-only — font props live on the
// `<LoadingCaption>` Text child so Tamagui can apply them via its
// text-style channel rather than relying on CSS cascade from a
// View-derived `<div>` wrapper.
const LoadingPanel = styledAny(View, {
  name: 'DailyFlowLoadingPanel',
  alignItems: 'center',
  gap: 16,
  paddingVertical: 48,
});

// Error message paragraph inside an error Card. `<p>` element + zero
// margin (browser UA stylesheet on `<p>` adds vertical margin we don't
// want inside the card's gap-16 layout). Color is destructive (full
// brand red) to signal the failure clearly.
const ErrorText = styledAny(Text, {
  name: 'DailyFlowErrorText',
  fontSize: '$5',
  fontWeight: '700',
  color: '$destructive',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

// Caption line under the spinner ("Loading…", "Submitting…"). Carries
// the font props because Tamagui's text-style channel applies them
// reliably on `<Text>`-derived components; cascading from
// `LoadingPanel`'s View wrapper was unreliable (Tamagui doesn't
// propagate font props through View-`<div>` to Text children).
// Extracted as a named styled component (not inline `<Text margin={0}>`)
// because v2-RC's `Partial<TextStyle>` inference rejects raw numeric
// props on inline JSX — same workaround pattern as PageShell's
// HeaderSpacer in #216.
const LoadingCaption = styledAny(Text, {
  name: 'DailyFlowLoadingCaption',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: '600',
  fontSize: '$4',
  color: '$body',
  margin: 0,
});

/**
 * Daily Puzzle flow chrome.
 *
 * Renders one of six visual states driven by the read query
 * (`useDailyPuzzle`) and the submit mutation (`useSubmitDaily`):
 *
 *   query.isPending                     → loading
 *   query.error                         → error
 *   query.data.kind === 'solved'        → solved (GET / IDB short-circuit)
 *   mutation.isPending                  → submitting
 *   mutation.isSuccess                  → solved (POST or 409)
 *   mutation.isError                    → submit-error
 *   query.data.kind === 'playing'       → playing (default)
 *
 * The pre-#176 6-arm `useState<FlowState>` discriminated union + 76
 * lines of `useEffect` cascade + `stateRef`-stabilised `handleSolved`
 * collapse into the two-hook composition above. Retry affordances:
 * the load-error's "Try again" calls `query.refetch()`; the
 * submit-error's "Try again" calls `mutation.reset()` (returns the UI
 * to playing — the user re-triggers submit by re-solving).
 */

/** Maps a thrown load error to user-facing copy. */
function loadErrorCopy(err: ApiError | Error): string {
  const status = err instanceof ApiError ? err.status : null;
  if (status === 404) return 'No daily available right now';
  if (status === 500) return 'Something went wrong, try again';
  return "Could not load today's daily";
}

/** Maps a thrown submit error to user-facing copy. */
function submitErrorCopy(err: ApiError | Error): string {
  const status = err instanceof ApiError ? err.status : null;
  if (status === 500) return 'Something went wrong, try again';
  if (status === 400) return "That doesn't match — keep trying";
  return 'Could not submit your result';
}

/**
 * Format the daily's assignedAt as a locale-aware "Mon DD" string for
 * the page subtitle (e.g. "May 11"). Use the UTC date components from
 * the RFC3339 timestamp — the daily belongs to a UTC calendar day, so
 * a player in PST playing at 23:30 local on May 10 still sees the May
 * 11 puzzle's date correctly rather than the local-time month/day.
 *
 * Returns an empty string for an unparseable assignedAt; callers fall
 * back to a date-less subtitle rather than crashing.
 */
function formatDailyDate(assignedAt: string): string {
  const d = new Date(assignedAt);
  if (Number.isNaN(d.getTime())) return '';
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  }).format(d);
}

/** Subtitle copy for the daily-flow PageShell, e.g. "Daily Puzzle · May 11". */
function dailySubtitle(assignedAt: string): string {
  const date = formatDailyDate(assignedAt);
  return date ? `Daily Puzzle · ${date}` : 'Daily Puzzle';
}

export function DailyFlow() {
  const navigate = useNavigate();
  const query = useDailyPuzzle();
  const mutation = useSubmitDaily();

  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  /** Re-runs the load query (load-error "Try again"). */
  const retryFetch = useCallback(() => {
    void query.refetch();
  }, [query]);

  /**
   * Return to the playing state (submit-error "Try again"). The user
   * re-triggers submit by re-solving — matches pre-#176 semantics
   * where retry went back to `{ kind: 'playing' }` without
   * automatically re-POSTing.
   */
  const retrySubmit = useCallback(() => {
    mutation.reset();
  }, [mutation]);

  /**
   * Fired by `<DailyGameBoard onSolved>`. Hands the player's solution
   * + elapsed-time to the submit mutation along with the current
   * GET payload (needed for the persistence step's puzzleId / grid /
   * regions / assignedAt).
   */
  const handleSolved = useCallback(
    (solution: number[][], elapsedMs: number) => {
      // Defensive guard: `DailyGameBoard` is only mounted when
      // `query.data.kind === 'playing'`, so this branch shouldn't be
      // reachable today. Warn (not silently drop) so any future code
      // path that re-mounts the board against a non-playing data kind
      // surfaces in the console rather than the click vanishing.
      if (query.data?.kind !== 'playing') {
        console.warn(
          '[DailyFlow] onSolved fired while query.data.kind=%s; dropping submit',
          query.data?.kind ?? 'pending',
        );
        return;
      }
      mutation.mutate({
        payload: query.data.payload,
        solution,
        elapsedMs,
      });
    },
    [query.data, mutation],
  );

  // --- Render branches --------------------------------------------------

  if (query.isPending) {
    return (
      <PageShell onBack={handleBack}>
        <LoadingPanel data-testid="daily-loading">
          <Spinner />
          <LoadingCaption>Loading today&apos;s daily…</LoadingCaption>
        </LoadingPanel>
      </PageShell>
    );
  }

  if (query.error) {
    return (
      <PageShell onBack={handleBack}>
        <Card size="compact" data-testid="daily-error">
          <ErrorText>{loadErrorCopy(query.error)}</ErrorText>
          <SecondaryButton onClick={retryFetch} data-testid="daily-retry">
            Try again
          </SecondaryButton>
        </Card>
      </PageShell>
    );
  }

  // GET or IDB short-circuit said solved — render PostCompletionScreen
  // directly without ever mounting the GameBoard.
  if (query.data.kind === 'solved') {
    return (
      <PostCompletionScreen
        serverElapsedMs={query.data.result.serverElapsedMs}
        submittedAt={query.data.submittedAt}
        leaderboardRank={query.data.result.leaderboardRank}
        isRecycle={query.data.payload.isRecycle}
      />
    );
  }

  // We have a playing payload. Now the mutation drives the rest.
  const playingPayload = query.data.payload;
  const subtitle = dailySubtitle(playingPayload.assignedAt);

  if (mutation.isPending) {
    return (
      <PageShell onBack={handleBack} subtitle={subtitle}>
        <LoadingPanel data-testid="daily-submitting">
          <Spinner />
          <LoadingCaption>Submitting…</LoadingCaption>
        </LoadingPanel>
      </PageShell>
    );
  }

  if (mutation.isSuccess) {
    return (
      <PostCompletionScreen
        serverElapsedMs={mutation.data.result.serverElapsedMs}
        submittedAt={mutation.data.submittedAt}
        leaderboardRank={mutation.data.result.leaderboardRank}
        synthetic409={mutation.data.synthetic409}
        isRecycle={mutation.data.payload.isRecycle}
      />
    );
  }

  if (mutation.isError) {
    return (
      <PageShell onBack={handleBack} subtitle={subtitle}>
        <Card size="compact" data-testid="daily-submit-error">
          <ErrorText>{submitErrorCopy(mutation.error)}</ErrorText>
          <PrimaryButton onClick={retrySubmit} data-testid="daily-submit-retry">
            Try again
          </PrimaryButton>
        </Card>
      </PageShell>
    );
  }

  return (
    <PageShell onBack={handleBack} subtitle={subtitle}>
      <DailyGameBoard payload={playingPayload} onSolved={handleSolved} />
    </PageShell>
  );
}

