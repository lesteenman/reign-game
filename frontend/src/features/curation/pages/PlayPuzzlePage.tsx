import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import {
  fetchNextPuzzle,
  NoPuzzlesAvailableError,
} from '@features/curation/services/fetch-next-puzzle-service';
import { createFreshGameState } from '@storage/utils';
import { PageShell } from '@shared/components/PageShell';
import { SecondaryButton } from '@shared/components/Button';
import { GameBoard } from '@shared/game/components/GameBoard';
import type { Mode, PuzzleData, CellState } from '@engine/types';
import { isMode } from '@engine/types';
import type { FlowType, GameHistory } from '@storage/types';
import { EMPTY_HISTORY, buildCurationFlowId, parseFlowType } from '@storage/types';
import { VerdictSurface } from '@features/curation/components/VerdictSurface';

type LoadState =
  | { status: 'loading' }
  | { status: 'ready'; puzzle: PuzzleData; flowType: FlowType; flowId: string; initialCells: CellState[][]; initialHistory: GameHistory; timerElapsed: number; timerResumedAt: number | null; startedAt: number }
  | { status: 'no-state' }
  | { status: 'no-puzzles' }
  | { status: 'error'; message: string };

/**
 * Curation/practice play route. Reads `?flow=curation&size=N&mode=M`
 * from the URL, loads a saved game from IndexedDB or fetches a fresh
 * puzzle from the pool, then mounts `<GameBoard>` with the admin
 * `<VerdictSurface>` slot wired up. An unrecognized or missing `flow`
 * redirects home (ST-11).
 *
 * The `?flow=daily` branch lives at the router level (`src/app/router.tsx`)
 * so this page never imports `features/daily/`. Was `pages/GamePage.tsx`
 * before #176.
 */
export function PlayPuzzlePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { loadState, saveState, clearState, addCompletion } = useGameStorage();
  const [loadStatus, setLoadStatus] = useState<LoadState>({ status: 'loading' });
  const [fetchKey, setFetchKey] = useState(0);

  /** Force a re-fetch of the current Flow Slot (used by Retry / Play Again). */
  const retryFetch = useCallback(() => {
    setLoadStatus({ status: 'loading' });
    setFetchKey((k) => k + 1);
  }, []);

  /** Navigate back to home without clearing game state. */
  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  useEffect(() => {
    let cancelled = false;
    const flowType = parseFlowType(searchParams.get('flow'));
    if (flowType === null) {
      setLoadStatus({ status: 'no-state' });
      return () => { cancelled = true; };
    }

    const size = Number(searchParams.get('size')) || 5;
    const modeParam = searchParams.get('mode');
    const mode: Mode = isMode(modeParam) ? modeParam : 'standard';
    // Phase 7 wires curation only; daily / pack producers will branch
    // here when their flowId conventions land.
    const flowId = buildCurationFlowId(size, mode);

    async function fetchFresh(): Promise<void> {
      try {
        const puzzle = await fetchNextPuzzle(size, mode);
        if (cancelled) return;
        const gameState = createFreshGameState(flowType!, flowId, puzzle);
        await saveState(gameState);
        if (cancelled) return;
        setLoadStatus({
          status: 'ready',
          puzzle: gameState.puzzle,
          flowType: gameState.flowType,
          flowId: gameState.flowId,
          initialCells: gameState.cells,
          initialHistory: gameState.history ?? EMPTY_HISTORY,
          timerElapsed: 0,
          timerResumedAt: null,
          startedAt: gameState.startedAt,
        });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof NoPuzzlesAvailableError) {
          setLoadStatus({ status: 'no-puzzles' });
          return;
        }
        const message = err instanceof Error ? err.message : 'Unknown error';
        setLoadStatus({ status: 'error', message });
      }
    }

    void loadState(flowType, flowId).then((saved) => {
      if (cancelled) return;
      // Hit + in-progress → resume. Miss or solved (defensive — clear-on-
      // solve should have removed it) → fetch fresh.
      if (saved && saved.status !== 'solved') {
        setLoadStatus({
          status: 'ready',
          puzzle: saved.puzzle,
          flowType: saved.flowType,
          flowId: saved.flowId,
          initialCells: saved.cells,
          initialHistory: saved.history ?? EMPTY_HISTORY,
          timerElapsed: saved.timer.elapsedAtLastPause,
          timerResumedAt: saved.timer.lastResumedAt,
          startedAt: saved.startedAt,
        });
        return;
      }
      void fetchFresh();
    }).catch(() => {
      if (!cancelled) void fetchFresh();
    });

    return () => { cancelled = true; };
  }, [searchParams, loadState, saveState, fetchKey]);

  if (loadStatus.status === 'loading') {
    return (
      <PageShell onBack={handleBack}>
        <div style={{ padding: '48px 0', fontWeight: 600 }}>Loading...</div>
      </PageShell>
    );
  }

  if (loadStatus.status === 'error') {
    return (
      <PageShell onBack={handleBack}>
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
            Failed to load puzzle: {loadStatus.message}
          </p>
          <SecondaryButton onClick={retryFetch}>
            Try Again
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  if (loadStatus.status === 'no-puzzles') {
    return (
      <PageShell onBack={handleBack}>
        <div
          data-testid="no-puzzles-state"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '16px',
            padding: '48px 0',
          }}
        >
          <p style={{ color: 'var(--color-body)', fontWeight: 600 }}>
            No puzzles available for this size and mode
          </p>
          <SecondaryButton onClick={retryFetch}>
            Retry
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  if (loadStatus.status === 'no-state') {
    return <RedirectToHome />;
  }

  return (
    <GameBoard
      key={loadStatus.puzzle.puzzleId}
      puzzle={loadStatus.puzzle}
      flowType={loadStatus.flowType}
      flowId={loadStatus.flowId}
      initialCells={loadStatus.initialCells}
      initialHistory={loadStatus.initialHistory}
      timerElapsed={loadStatus.timerElapsed}
      timerResumedAt={loadStatus.timerResumedAt}
      startedAt={loadStatus.startedAt}
      navigate={navigate}
      saveState={saveState}
      clearState={clearState}
      addCompletion={addCompletion}
      onBack={handleBack}
      onPlayAgain={retryFetch}
      AdminVerdictSurface={VerdictSurface}
    />
  );
}

/** Small component to redirect home via effect. */
function RedirectToHome() {
  const navigate = useNavigate();
  useEffect(() => { navigate('/', { replace: true }); }, [navigate]);
  return null;
}
