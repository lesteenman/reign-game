import { useState, useEffect, useCallback, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGameStorage } from '../hooks/useGameStorage';
import { PageShell } from '../components/common/PageShell';
import { PrimaryButton, SecondaryButton } from '../components/common/Button';

type PageState =
  | { status: 'loading' }
  | { status: 'fresh' }
  | { status: 'has-progress' }
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

  const handleNewPuzzle = useCallback(() => {
    if (!navigator.onLine) {
      setState({
        status: 'error',
        message: "You're offline — resume your current puzzle or connect to start a new one",
      });
      return;
    }
    navigate('/play?new=true');
  }, [navigate]);

  const handleResume = useCallback(() => {
    navigate('/play');
  }, [navigate]);

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
        <PrimaryButton onClick={handleNewPuzzle}>Play</PrimaryButton>
      )}

      {state.status === 'has-progress' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', alignItems: 'center' }}>
          <PrimaryButton onClick={handleResume}>Resume</PrimaryButton>
          <SecondaryButton onClick={handleNewPuzzle}>New Puzzle</SecondaryButton>
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
          <SecondaryButton onClick={handleNewPuzzle}>Try Again</SecondaryButton>
        </div>
      )}
    </PageShell>
  );
}
