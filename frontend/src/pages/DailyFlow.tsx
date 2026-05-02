import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '../components/common/PageShell';
import { SecondaryButton } from '../components/common/Button';
import {
  DailyApiError,
  getDaily,
  type DailyPuzzlePayload,
} from '../services/dailyService';

/**
 * Daily Puzzle flow chrome (R-8-02 chunk 3).
 *
 * Owns the GET-side state machine per DP-31:
 *   loading → data (started)   → renders the chunk-3 daily-loaded stub
 *   loading → data (solved)    → renders the chunk-5 placeholder
 *   loading → error            → renders status-aware error UI per DP-30
 *
 * Submission wiring (chunk 6) and the real post-completion screen
 * (chunk 5) replace the placeholders without changing this component's
 * outer state machine.
 */

type State =
  | { status: 'loading' }
  | { status: 'data'; payload: DailyPuzzlePayload }
  | { status: 'error'; httpStatus: number | null; message: string };

/** Maps a thrown error to user-facing copy per DP-30. */
function errorCopy(state: { httpStatus: number | null }): string {
  if (state.httpStatus === 404) return 'No daily available right now';
  if (state.httpStatus === 500) return 'Something went wrong, try again';
  return "Could not load today's daily";
}

export function DailyFlow() {
  const navigate = useNavigate();
  const [state, setState] = useState<State>({ status: 'loading' });
  const [fetchKey, setFetchKey] = useState(0);

  /** Re-runs the fetch effect (DP-30 retry affordance). */
  const retry = useCallback(() => {
    setState({ status: 'loading' });
    setFetchKey((k) => k + 1);
  }, []);

  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const payload = await getDaily();
        if (cancelled) return;
        setState({ status: 'data', payload });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof DailyApiError) {
          setState({
            status: 'error',
            httpStatus: err.status,
            message: err.message,
          });
          return;
        }
        const message = err instanceof Error ? err.message : 'Unknown error';
        setState({ status: 'error', httpStatus: null, message });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchKey]);

  if (state.status === 'loading') {
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

  if (state.status === 'error') {
    return (
      <PageShell onBack={handleBack}>
        <div
          data-testid="daily-error"
          // BRAND_GUIDELINES §5.5 card pattern: surface bg, 2px ink
          // border, layered offset shadow, 24px padding. Inline because
          // no shared <ErrorCard /> primitive exists yet — falling back
          // to guideline values directly.
          style={{
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
          }}
        >
          <p
            style={{
              margin: 0,
              color: 'var(--color-destructive)',
              fontWeight: 700,
              fontSize: '1.125rem',
            }}
          >
            {errorCopy(state)}
          </p>
          <SecondaryButton onClick={retry} data-testid="daily-retry">
            Try again
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  // status === 'data'
  if (state.payload.outcome === 'solved') {
    return (
      <PageShell onBack={handleBack}>
        <div
          data-testid="daily-solved-placeholder"
          style={{
            padding: '48px 0',
            fontFamily: '"Nunito Sans", system-ui, sans-serif',
            color: 'var(--color-body)',
            textAlign: 'center',
          }}
        >
          Already solved (post-completion screen lands chunk 5)
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell onBack={handleBack}>
      <div
        data-testid="daily-loaded"
        style={{
          padding: '48px 0',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          color: 'var(--color-body)',
          textAlign: 'center',
        }}
      >
        Daily loaded: {state.payload.puzzleId} (grid renderer wires in chunk 6)
      </div>
    </PageShell>
  );
}

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
