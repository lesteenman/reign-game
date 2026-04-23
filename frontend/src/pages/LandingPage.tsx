import { useState, useEffect, useCallback, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGameStorage } from '../hooks/useGameStorage';
import { PageShell } from '../components/common/PageShell';
import { PrimaryButton, SecondaryButton } from '../components/common/Button';
import { PuzzleSelector } from '../components/landing/PuzzleSelector';
import type { PuzzleSelection } from '../components/landing/PuzzleSelector';
import { fetchEnabledModes } from '../services/landingService';
import type { ModeEntry } from '../services/landingService';

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

/** Build a URL search string from puzzle selection. */
function buildPlayUrl(selection: PuzzleSelection): string {
  const params = new URLSearchParams();
  params.set('new', 'true');
  params.set('size', String(selection.size));
  params.set('mode', selection.mode);
  return `/play?${params.toString()}`;
}

/** Landing page with resume/new puzzle flow. */
export function LandingPage() {
  const navigate = useNavigate();
  const { loadState } = useGameStorage();
  const [state, setState] = useState<PageState>({ status: 'loading' });
  // null while the modes fetch is in flight; empty-array once a fetch
  // resolves with no enabled combos or the fetch fails. Either way the
  // PuzzleSelector handles the empty case with a friendly message.
  const [modes, setModes] = useState<ModeEntry[] | null>(null);
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

  useEffect(() => {
    let cancelled = false;
    void fetchEnabledModes()
      .then((list) => {
        if (!cancelled) setModes(list);
      })
      .catch((err) => {
        // Network or backend failure — fall through to the empty-state
        // UI. The offline banner separately covers connectivity loss.
        // Logged at warn level so dev-tools surfaces the root cause
        // instead of a silent "no puzzles" message hiding a 500.
        console.warn('LandingPage: fetchEnabledModes failed', err);
        if (!cancelled) setModes([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleNewPuzzle = useCallback((selection: PuzzleSelection) => {
    if (!navigator.onLine) {
      setState({
        status: 'error',
        message: "You're offline — resume your current puzzle or connect to start a new one",
      });
      return;
    }
    navigate(buildPlayUrl(selection));
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
        <PuzzleSelector modes={modes ?? []} onSelect={handleNewPuzzle} />
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
          <PuzzleSelector modes={modes ?? []} onSelect={handleNewPuzzle} />
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
          <PuzzleSelector modes={modes ?? []} onSelect={handleNewPuzzle} />
        </div>
      )}
    </PageShell>
  );
}

export { buildPlayUrl };
