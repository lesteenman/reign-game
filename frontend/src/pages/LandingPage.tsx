import { useState, useEffect, useCallback, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGameStorage } from '../hooks/useGameStorage';
import { generatePuzzle } from '../services/puzzleService';
import type { GameState } from '../storage/types';

type PageState =
  | { status: 'loading' }
  | { status: 'fresh' }
  | { status: 'has-progress' }
  | { status: 'fetching' }
  | { status: 'error'; message: string };

/** Subscribe to online/offline events and return current connectivity status. */
function subscribeOnline(callback: () => void) {
  window.addEventListener('online', callback);
  window.addEventListener('offline', callback);
  return () => {
    window.removeEventListener('online', callback);
    window.removeEventListener('offline', callback);
  };
}

function getOnlineSnapshot(): boolean {
  return navigator.onLine;
}

function getServerOnlineSnapshot(): boolean {
  return true;
}

/** Landing page with resume/new puzzle flow. */
export function LandingPage() {
  const navigate = useNavigate();
  const { loadState, saveState } = useGameStorage();
  const [state, setState] = useState<PageState>({ status: 'loading' });
  const isOnline = useSyncExternalStore(subscribeOnline, getOnlineSnapshot, getServerOnlineSnapshot);

  useEffect(() => {
    let cancelled = false;
    void loadState().then((saved) => {
      if (cancelled) return;
      if (saved && saved.status === 'in-progress') {
        setState({ status: 'has-progress' });
      } else {
        setState({ status: 'fresh' });
      }
    }).catch(() => {
      if (!cancelled) setState({ status: 'fresh' });
    });
    return () => { cancelled = true; };
  }, [loadState]);

  const startNewPuzzle = useCallback(async () => {
    if (!navigator.onLine) {
      setState({
        status: 'error',
        message: "You're offline — resume your current puzzle or connect to start a new one",
      });
      return;
    }
    setState({ status: 'fetching' });
    try {
      const puzzle = await generatePuzzle(5, 'standard');
      const gameState: GameState = {
        id: 'current',
        puzzle,
        cells: Array.from({ length: puzzle.gridSize }, () =>
          Array.from({ length: puzzle.gridSize }, () => 'empty' as const),
        ),
        timer: { elapsedAtLastPause: 0, lastResumedAt: null },
        status: 'in-progress',
        startedAt: Date.now(),
      };
      await saveState(gameState);
      navigate('/play');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setState({ status: 'error', message });
    }
  }, [saveState, navigate]);

  const handleResume = useCallback(() => {
    navigate('/play');
  }, [navigate]);

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '24px',
        padding: '24px 16px',
        minHeight: '100vh',
        backgroundColor: 'var(--color-background)',
        fontFamily: '"Nunito Sans", system-ui, sans-serif',
        color: 'var(--color-ink)',
      }}
    >
      <h1
        style={{
          fontSize: '1.875rem',
          fontWeight: 800,
          letterSpacing: '-0.01em',
          margin: 0,
        }}
      >
        Reign
      </h1>

      {!isOnline && (
        <div
          role="alert"
          data-testid="offline-banner"
          style={{
            padding: '8px 16px',
            borderRadius: 'var(--radius)',
            backgroundColor: 'var(--color-surface)',
            border: '2px solid var(--color-ink)',
            fontWeight: 600,
            fontSize: '0.875rem',
            textAlign: 'center',
          }}
        >
          You&apos;re offline — resume your current puzzle or connect to start a new one
        </div>
      )}

      {state.status === 'loading' && (
        <div data-testid="loading-state" style={{ padding: '48px 0', fontWeight: 600 }}>
          Loading...
        </div>
      )}

      {state.status === 'fresh' && (
        <button
          type="button"
          onClick={() => void startNewPuzzle()}
          style={{
            padding: '12px 32px',
            backgroundColor: 'var(--color-accent)',
            color: 'var(--color-on-accent)',
            border: '2px solid var(--color-ink)',
            borderRadius: 'var(--radius)',
            boxShadow: '0 3px 0 var(--color-accent-shadow)',
            fontFamily: '"Nunito Sans", system-ui, sans-serif',
            fontWeight: 700,
            fontSize: '1rem',
            cursor: 'pointer',
            transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.transform = 'translateY(1px)';
            e.currentTarget.style.boxShadow = '0 2px 0 var(--color-accent-shadow)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.transform = 'translateY(0)';
            e.currentTarget.style.boxShadow = '0 3px 0 var(--color-accent-shadow)';
          }}
        >
          Play
        </button>
      )}

      {state.status === 'has-progress' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', alignItems: 'center' }}>
          <button
            type="button"
            onClick={handleResume}
            style={{
              padding: '12px 32px',
              backgroundColor: 'var(--color-accent)',
              color: 'var(--color-on-accent)',
              border: '2px solid var(--color-ink)',
              borderRadius: 'var(--radius)',
              boxShadow: '0 3px 0 var(--color-accent-shadow)',
              fontFamily: '"Nunito Sans", system-ui, sans-serif',
              fontWeight: 700,
              fontSize: '1rem',
              cursor: 'pointer',
              transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(1px)';
              e.currentTarget.style.boxShadow = '0 2px 0 var(--color-accent-shadow)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = '0 3px 0 var(--color-accent-shadow)';
            }}
          >
            Resume
          </button>
          <button
            type="button"
            onClick={() => void startNewPuzzle()}
            style={{
              padding: '12px 32px',
              backgroundColor: 'var(--color-surface)',
              color: 'var(--color-ink)',
              border: '2px solid var(--color-ink)',
              borderRadius: 'var(--radius)',
              boxShadow: '0 3px 0 var(--color-ink)',
              fontFamily: '"Nunito Sans", system-ui, sans-serif',
              fontWeight: 700,
              fontSize: '1rem',
              cursor: 'pointer',
              transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(1px)';
              e.currentTarget.style.boxShadow = '0 2px 0 var(--color-ink)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = '0 3px 0 var(--color-ink)';
            }}
          >
            New Puzzle
          </button>
        </div>
      )}

      {state.status === 'fetching' && (
        <div data-testid="loading-state" style={{ padding: '48px 0', fontWeight: 600 }}>
          Loading puzzle...
        </div>
      )}

      {state.status === 'error' && (
        <div
          data-testid="error-state"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '16px',
            padding: '48px 0',
          }}
        >
          <p style={{ color: 'var(--color-destructive)', fontWeight: 600 }}>
            Failed to load puzzle: {state.message}
          </p>
          <button
            type="button"
            onClick={() => void startNewPuzzle()}
            style={{
              padding: '12px 32px',
              backgroundColor: 'var(--color-surface)',
              color: 'var(--color-ink)',
              border: '2px solid var(--color-ink)',
              borderRadius: 'var(--radius)',
              boxShadow: '0 3px 0 var(--color-ink)',
              fontFamily: '"Nunito Sans", system-ui, sans-serif',
              fontWeight: 700,
              fontSize: '1rem',
              cursor: 'pointer',
            }}
          >
            Try Again
          </button>
        </div>
      )}
    </div>
  );
}
