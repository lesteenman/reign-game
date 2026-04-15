import { useState, useEffect, useCallback, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGameStorage } from '../hooks/useGameStorage';
import { PageShell } from '../components/common/PageShell';
import { PrimaryButton, SecondaryButton } from '../components/common/Button';
import { PuzzleSelector } from '../components/landing/PuzzleSelector';
import type { GenerateOptions } from '../services/puzzleService';

type PageState =
  | { status: 'loading' }
  | { status: 'fresh' }
  | { status: 'has-progress' }
  | { status: 'has-progress-selecting' }
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

/** Build a URL search string from GenerateOptions. */
function buildPlayUrl(options: GenerateOptions): string {
  const params = new URLSearchParams();
  params.set('new', 'true');
  params.set('size', String(options.size));
  params.set('mode', options.mode);
  if (options.pipeline !== undefined) {
    params.set('pipeline', options.pipeline);
  }
  if (options.solver !== undefined) {
    params.set('solver', options.solver);
  }
  if (options.regions !== undefined) {
    params.set('regions', options.regions);
  }
  if (options.regionVariance !== undefined) {
    params.set('regionVariance', String(options.regionVariance));
  }
  return `/play?${params.toString()}`;
}

/** Landing page with resume/new puzzle flow. */
export function LandingPage() {
  const navigate = useNavigate();
  const { loadState } = useGameStorage();
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

  const handleNewPuzzle = useCallback((options: GenerateOptions) => {
    if (!navigator.onLine) {
      setState({
        status: 'error',
        message: "You're offline — resume your current puzzle or connect to start a new one",
      });
      return;
    }
    navigate(buildPlayUrl(options));
  }, [navigate]);

  const handleResume = useCallback(() => {
    navigate('/play');
  }, [navigate]);

  const handleShowSelector = useCallback(() => {
    setState({ status: 'has-progress-selecting' });
  }, []);

  return (
    <PageShell>
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
        <PuzzleSelector onSelect={handleNewPuzzle} />
      )}

      {state.status === 'has-progress' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', alignItems: 'center' }}>
          <PrimaryButton onClick={handleResume}>Resume</PrimaryButton>
          <SecondaryButton onClick={handleShowSelector}>New Puzzle</SecondaryButton>
        </div>
      )}

      {state.status === 'has-progress-selecting' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', alignItems: 'center', width: '100%' }}>
          <SecondaryButton onClick={handleResume}>Resume</SecondaryButton>
          <PuzzleSelector onSelect={handleNewPuzzle} />
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
            {state.message}
          </p>
          <PuzzleSelector onSelect={handleNewPuzzle} />
        </div>
      )}
    </PageShell>
  );
}

export { buildPlayUrl };
