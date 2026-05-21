import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '@shared/components/PageShell';
import { PrimaryButton, SecondaryButton } from '@shared/components/Button';
import { ApiError } from '@shared/api';
import { useDailyPuzzle } from '@features/daily/hooks/useDailyPuzzle';
import { useSubmitDaily } from '@features/daily/hooks/useSubmitDaily';
import { DailyGameBoard } from './DailyGameBoard';
import { PostCompletionScreen } from './PostCompletionScreen';

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
        <div data-testid="daily-loading" style={loadingStyle}>
          <Spinner />
          <p style={{ margin: 0 }}>Loading today&apos;s daily…</p>
        </div>
      </PageShell>
    );
  }

  if (query.error) {
    return (
      <PageShell onBack={handleBack}>
        <div data-testid="daily-error" style={errorCardStyle}>
          <p style={errorTextStyle}>{loadErrorCopy(query.error)}</p>
          <SecondaryButton onClick={retryFetch} data-testid="daily-retry">
            Try again
          </SecondaryButton>
        </div>
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
      />
    );
  }

  // We have a playing payload. Now the mutation drives the rest.
  const playingPayload = query.data.payload;
  const subtitle = dailySubtitle(playingPayload.assignedAt);

  if (mutation.isPending) {
    return (
      <PageShell onBack={handleBack} subtitle={subtitle}>
        <div data-testid="daily-submitting" style={loadingStyle}>
          <Spinner />
          <p style={{ margin: 0 }}>Submitting…</p>
        </div>
      </PageShell>
    );
  }

  if (mutation.isSuccess) {
    return (
      <PostCompletionScreen
        serverElapsedMs={mutation.data.result.serverElapsedMs}
        submittedAt={mutation.data.submittedAt}
        leaderboardRank={mutation.data.result.leaderboardRank}
      />
    );
  }

  if (mutation.isError) {
    return (
      <PageShell onBack={handleBack} subtitle={subtitle}>
        <div data-testid="daily-submit-error" style={errorCardStyle}>
          <p style={errorTextStyle}>{submitErrorCopy(mutation.error)}</p>
          <PrimaryButton onClick={retrySubmit} data-testid="daily-submit-retry">
            Try again
          </PrimaryButton>
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell onBack={handleBack} subtitle={subtitle}>
      <DailyGameBoard payload={playingPayload} onSolved={handleSolved} />
    </PageShell>
  );
}

// --- Shared inline style tokens -----------------------------------------
// BRAND_GUIDELINES §5.5 card pattern: surface bg, 2px ink border,
// layered offset shadow, 24px padding. Inline because no shared
// <ErrorCard /> primitive exists yet — falling back to guideline values.

const loadingStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '16px',
  padding: '48px 0',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 600,
  color: 'var(--color-body)',
};

const errorCardStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '16px',
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  boxShadow: '0 3px 0 var(--color-ink)',
  padding: '24px',
  maxWidth: 480,
  width: '100%',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
};

const errorTextStyle: React.CSSProperties = {
  margin: 0,
  color: 'var(--color-destructive)',
  fontWeight: 700,
  fontSize: '1.125rem',
};

/**
 * Lightweight CSS spinner using brand colors. No shared <Spinner />
 * primitive exists in the codebase yet, so we render inline with
 * BRAND_GUIDELINES §1 colors and §6.1 timing tokens.
 */
function Spinner() {
  return (
    <div
      role="status"
      aria-label="Loading"
      style={{
        width: 32,
        height: 32,
        borderRadius: '50%',
        border: '3px solid var(--color-border)',
        borderTopColor: 'var(--color-accent)',
        animation: 'daily-flow-spin 800ms linear infinite',
      }}
    >
      <style>{`
        @keyframes daily-flow-spin {
          to { transform: rotate(360deg); }
        }
        @media (prefers-reduced-motion: reduce) {
          [role="status"][aria-label="Loading"] { animation: none; }
        }
      `}</style>
    </div>
  );
}
