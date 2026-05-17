import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '../../../components/common/PageShell';
import { PrimaryButton, SecondaryButton } from '../../../components/common/Button';
import { ApiError } from '../../../services/api';
import {
  getDaily,
  submitDailyResult,
  type DailyPuzzlePayload,
  type DailySubmitResponse,
} from '../../../services/dailyService';
import { DailyGameBoard } from './DailyGameBoard';
import { PostCompletionScreen } from './PostCompletionScreen';
import { useGameStorage } from '../../../hooks/useGameStorage';
import type { GameState } from '../../../storage/types';
import type { CellState } from '../../../engine/types';

/**
 * Daily Puzzle flow chrome (R-8-02 chunks 3 + 6).
 *
 * Owns the daily-flow state machine per DP-31:
 *   loading      → playing                 on getDaily() success
 *   loading      → solved (chunk-3 stub)   on getDaily() returning outcome=solved
 *   loading      → error                   on getDaily() failure (DP-30)
 *   playing      → submitting              on the GameBoard's onSolved event
 *   submitting   → solved                  on POST 200 (DailySubmitResponse)
 *   submitting   → solved                  on POST 409 (server has canonical state)
 *   submitting   → submit-error            on other POST failure
 *   submit-error → playing                 on retry (player solution preserved)
 *
 * The `solved` state renders <PostCompletionScreen /> with the server
 * response's serverElapsedMs, submittedAt, and leaderboardRank.
 */

type FlowState =
  | { kind: 'loading' }
  | {
      kind: 'error';
      httpStatus: number | null;
      message: string;
    }
  | { kind: 'playing'; payload: DailyPuzzlePayload }
  | { kind: 'submitting'; payload: DailyPuzzlePayload }
  | {
      kind: 'solved';
      payload: DailyPuzzlePayload;
      result: DailySubmitResponse;
      submittedAt: string;
    }
  | {
      kind: 'submit-error';
      payload: DailyPuzzlePayload;
      httpStatus: number | null;
      message: string;
    };

/** Maps a thrown error to user-facing copy per DP-30. */
function loadErrorCopy(state: { httpStatus: number | null }): string {
  if (state.httpStatus === 404) return 'No daily available right now';
  if (state.httpStatus === 500) return 'Something went wrong, try again';
  return "Could not load today's daily";
}

/** Maps a thrown submit error to user-facing copy. */
function submitErrorCopy(state: { httpStatus: number | null }): string {
  if (state.httpStatus === 500) return 'Something went wrong, try again';
  if (state.httpStatus === 400) return "That doesn't match — keep trying";
  return 'Could not submit your result';
}

/** Extracts YYYY-MM-DD from an RFC3339 timestamp's UTC date component. */
function dateFromAssignedAt(assignedAt: string): string {
  return new Date(assignedAt).toISOString().slice(0, 10);
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

/** Empty cells grid sized to the payload's grid dimension — minimal
 *  placeholder so the persisted GameState row carries a coherent
 *  shape for the daily flow. The follow-up chunk that wires the real
 *  GameBoard replaces this with the live cells reference. */
function makeEmptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from({ length: size }, () => 'empty' as const),
  );
}

/** Today's date as YYYY-MM-DD in UTC. The player's daily flowId
 *  is derived from server-provided `assignedAt`, but at mount time
 *  (before the GET) we don't have the server's view yet. Using
 *  today's UTC date here is the standard short-circuit path: a
 *  player who solved today's daily and revisits the same UTC day
 *  hits the persisted row directly without re-fetching. Cross-
 *  midnight edge cases (player solved 23:55 UTC, revisits at
 *  00:01 UTC) fall through to `getDaily()` and pick up the new
 *  day's puzzle naturally. */
function todayUtcDate(): string {
  return new Date().toISOString().slice(0, 10);
}

export function DailyFlow() {
  const navigate = useNavigate();
  const { saveState, loadState } = useGameStorage();
  const [state, setState] = useState<FlowState>({ kind: 'loading' });
  const [fetchKey, setFetchKey] = useState(0);

  // `handleSolved` reads the latest state via a ref so its identity
  // stays stable across renders. Without this, `handleSolved` re-binds
  // every state transition, which churns `DailyGameBoard`'s memoized
  // `onSolveDetectedRef` and triggers redundant re-renders of the
  // grid. Lesson: identity stability matters for downstream memos.
  //
  // The ref is assigned at render time (not in useEffect) so the value
  // is current the moment React commits. A useEffect-based sync lags one
  // commit behind, which races event handlers reading the ref between
  // commit and effect — manifested as an intermittent test flake where
  // the post-click state transition didn't fire.
  const stateRef = useRef<FlowState>(state);
  stateRef.current = state;

  /** Re-runs the fetch effect (DP-30 retry affordance). */
  const retryFetch = useCallback(() => {
    setState({ kind: 'loading' });
    setFetchKey((k) => k + 1);
  }, []);

  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      // DP-27 short-circuit. Read the per-flow IDB row for today's
      // UTC date BEFORE the GET. If we have a solved row with a
      // canonical `serverElapsedMs`, render PostCompletionScreen
      // straight from persisted state — no network round-trip,
      // observable as a missing /api/daily/* request in Playwright.
      // A solved row missing `serverElapsedMs` (older partial write
      // shape) falls through to `getDaily()` so the server's solved
      // payload drives the post-completion render.
      try {
        const persisted = await loadState('daily', todayUtcDate());
        if (cancelled) return;
        if (
          persisted &&
          persisted.status === 'solved' &&
          typeof persisted.serverElapsedMs === 'number' &&
          persisted.serverElapsedMs > 0
        ) {
          // Synthesize a minimal payload + result so the existing
          // 'solved' render branch works without a special case.
          const payload: DailyPuzzlePayload = {
            puzzleId: persisted.puzzle.puzzleId,
            grid: persisted.puzzle.gridSize,
            regions: persisted.puzzle.regionMap,
            assignedAt: `${persisted.flowId}T00:00:00Z`,
            outcome: 'solved',
          };
          setState({
            kind: 'solved',
            payload,
            result: {
              serverElapsedMs: persisted.serverElapsedMs,
              leaderboardRank: persisted.leaderboardRank,
            },
            submittedAt:
              persisted.submittedAt ?? new Date().toISOString(),
          });
          return;
        }
      } catch {
        // Storage read failed — fall through to the network path.
        // Better to fetch (and retry on next visit) than to surface
        // a phantom error UI for a transient IDB hiccup.
      }

      try {
        const payload = await getDaily();
        if (cancelled) return;
        if (payload.outcome === 'solved') {
          // GET-said-solved short-circuit. Server returned a payload
          // with the canonical solve fields; render PostCompletionScreen
          // straight away. The POST response shape (DailySubmitResponse)
          // is reconstructed from the GET payload's solved fields —
          // leaderboardRank is intentionally undefined here because GET
          // /api/daily/* does not surface rank (only POST does).
          if (
            typeof payload.serverElapsedMs === 'number' &&
            payload.submittedAt
          ) {
            setState({
              kind: 'solved',
              payload,
              result: {
                serverElapsedMs: payload.serverElapsedMs,
                leaderboardRank: undefined,
              },
              submittedAt: payload.submittedAt,
            });
            return;
          }
          // Defensive: GET returned outcome=solved but the canonical
          // fields are missing (older partial-response shape). Log and
          // fall through to the playing branch so the gap is observable
          // rather than silently rendering a broken post-completion
          // screen.
          console.warn(
            'DailyFlow: getDaily returned outcome=solved without serverElapsedMs/submittedAt; falling back to playing',
          );
        }
        setState({ kind: 'playing', payload });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError) {
          setState({
            kind: 'error',
            httpStatus: err.status,
            message: err.message,
          });
          return;
        }
        const message = err instanceof Error ? err.message : 'Unknown error';
        setState({ kind: 'error', httpStatus: null, message });
      }
    })();
    return () => {
      cancelled = true;
    };
    // loadState is stable from useGameStorage's useMemo, but list it
    // explicitly so exhaustive-deps stays clean.
  }, [fetchKey, loadState]);

  /**
   * Submit handler — fired by `<DailyGameBoard onSolved>`. Drives the
   * playing → submitting → (solved | submit-error) transition. On
   * success persists the solved row to the per-flow IDB slot
   * `(daily, YYYY-MM-DD)` so a subsequent visit short-circuits via
   * DP-27 (chunk 7).
   */
  const handleSolved = useCallback(
    async (solution: number[][], elapsedMs: number) => {
      // Read the current state via the ref so this callback's identity
      // is stable across renders (no `state` in the dep list). The ref
      // is assigned synchronously at render time above, so it always
      // reflects the latest committed state when this handler fires.
      const current = stateRef.current;
      if (current.kind !== 'playing') return;
      const activePayload = current.payload;
      setState({ kind: 'submitting', payload: activePayload });

      try {
        const result = await submitDailyResult({
          assignedAt: activePayload.assignedAt,
          outcome: 'solved',
          playTimeMs: elapsedMs,
          solution,
        });
        const submittedAt = new Date().toISOString();

        // Persist the solved row to the per-flow store. Lesson 16:
        // the persisted shape lives in storage/, so we route through
        // useGameStorage's saveState rather than redeclaring a daily
        // row layout here. The minimal GameState shape is enough to
        // satisfy DP-27's short-circuit read on the next visit.
        const flowId = dateFromAssignedAt(activePayload.assignedAt);
        const persistedState: GameState = {
          id: `daily:${flowId}`,
          flowType: 'daily',
          flowId,
          puzzle: {
            puzzleId: activePayload.puzzleId,
            gridSize: activePayload.grid,
            mode: 'standard',
            regionMap: activePayload.regions,
          },
          cells: makeEmptyCells(activePayload.grid),
          timer: {
            elapsedAtLastPause: result.serverElapsedMs,
            lastResumedAt: null,
          },
          status: 'solved',
          startedAt: Date.now() - result.serverElapsedMs,
          // R-8-02 chunk 7: persist the canonical solve fields so a
          // subsequent visit short-circuits via DP-27 with the same
          // PostCompletionScreen content (no second GET, no second
          // POST).
          serverElapsedMs: result.serverElapsedMs,
          submittedAt,
          leaderboardRank: result.leaderboardRank,
        };
        await saveState(persistedState);

        setState({
          kind: 'solved',
          payload: activePayload,
          result,
          submittedAt,
        });
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          // 409 = server says already solved. Treat as terminal solved
          // state per DP-30. Server didn't return a fresh body shape,
          // so use sensible defaults; chunk 7 will read the canonical
          // solved row back from the GET endpoint on the next visit.
          setState({
            kind: 'solved',
            payload: activePayload,
            result: { serverElapsedMs: 0 },
            submittedAt: new Date().toISOString(),
          });
          return;
        }
        if (err instanceof ApiError) {
          setState({
            kind: 'submit-error',
            payload: activePayload,
            httpStatus: err.status,
            message: err.message,
          });
          return;
        }
        const message = err instanceof Error ? err.message : 'Unknown error';
        setState({
          kind: 'submit-error',
          payload: activePayload,
          httpStatus: null,
          message,
        });
      }
    },
    [saveState],
  );

  const retrySubmit = useCallback(() => {
    setState((current) => {
      if (current.kind !== 'submit-error') return current;
      return { kind: 'playing', payload: current.payload };
    });
  }, []);

  // --- Render branches --------------------------------------------------

  if (state.kind === 'loading') {
    return (
      <PageShell onBack={handleBack}>
        <div
          data-testid="daily-loading"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '16px',
            padding: '48px 0',
            fontFamily: '"Nunito Sans", system-ui, sans-serif',
            fontWeight: 600,
            color: 'var(--color-body)',
          }}
        >
          <Spinner />
          <p style={{ margin: 0 }}>Loading today&apos;s daily…</p>
        </div>
      </PageShell>
    );
  }

  if (state.kind === 'error') {
    return (
      <PageShell onBack={handleBack}>
        <div
          data-testid="daily-error"
          style={errorCardStyle}
        >
          <p style={errorTextStyle}>{loadErrorCopy(state)}</p>
          <SecondaryButton onClick={retryFetch} data-testid="daily-retry">
            Try again
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  if (state.kind === 'playing') {
    return (
      <PageShell
        onBack={handleBack}
        subtitle={dailySubtitle(state.payload.assignedAt)}
      >
        <DailyGameBoard payload={state.payload} onSolved={handleSolved} />
      </PageShell>
    );
  }

  if (state.kind === 'submitting') {
    return (
      <PageShell
        onBack={handleBack}
        subtitle={dailySubtitle(state.payload.assignedAt)}
      >
        <div
          data-testid="daily-submitting"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '16px',
            padding: '48px 0',
            fontFamily: '"Nunito Sans", system-ui, sans-serif',
            fontWeight: 600,
            color: 'var(--color-body)',
          }}
        >
          <Spinner />
          <p style={{ margin: 0 }}>Submitting…</p>
        </div>
      </PageShell>
    );
  }

  if (state.kind === 'submit-error') {
    return (
      <PageShell
        onBack={handleBack}
        subtitle={dailySubtitle(state.payload.assignedAt)}
      >
        <div data-testid="daily-submit-error" style={errorCardStyle}>
          <p style={errorTextStyle}>{submitErrorCopy(state)}</p>
          <PrimaryButton onClick={retrySubmit} data-testid="daily-submit-retry">
            Try again
          </PrimaryButton>
        </div>
      </PageShell>
    );
  }

  // state.kind === 'solved'
  return (
    <PostCompletionScreen
      serverElapsedMs={state.result.serverElapsedMs}
      submittedAt={state.submittedAt}
      leaderboardRank={state.result.leaderboardRank}
    />
  );
}

// --- Shared inline style tokens -----------------------------------------
// BRAND_GUIDELINES §5.5 card pattern: surface bg, 2px ink border,
// layered offset shadow, 24px padding. Inline because no shared
// <ErrorCard /> primitive exists yet — falling back to guideline values.

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
