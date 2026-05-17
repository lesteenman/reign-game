import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useGameStorage } from '../hooks/useGameStorage';
import { fetchNextPuzzle, NoPuzzlesAvailableError } from '../services/puzzleService';
import { createFreshGameState } from '../storage/utils';
import { PageShell } from '../components/common/PageShell';
import { SecondaryButton } from '../components/common/Button';
import { GameBoard } from '../shared/game/components/GameBoard';
import type { Mode, PuzzleData, CellState } from '../engine/types';
import { isMode } from '../engine/types';
import type { FlowType, GameHistory } from '../storage/types';
import { EMPTY_HISTORY, buildCurationFlowId, parseFlowType } from '../storage/types';
import { DailyFlow } from '../features/daily/screens/DailyFlow';

type LoadState =
  | { status: 'loading' }
  | { status: 'ready'; puzzle: PuzzleData; flowType: FlowType; flowId: string; initialCells: CellState[][]; initialHistory: GameHistory; timerElapsed: number; timerResumedAt: number | null; startedAt: number }
  | { status: 'no-state' }
  | { status: 'no-puzzles' }
  | { status: 'error'; message: string };

/**
 * Main gameplay page. The URL specifies the Flow Slot
 * (`?flow=curation&size=N&mode=M`); storage decides resume vs. fetch.
 * An unrecognized or missing `flow` redirects home (ST-11).
 */
export function GamePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { loadState, saveState, clearState, addCompletion } = useGameStorage();
  const [loadStatus, setLoadStatus] = useState<LoadState>({ status: 'loading' });
  const [fetchKey, setFetchKey] = useState(0);

  // When the URL says `?flow=daily`, delegate the entire flow to
  // DailyFlow. The existing pool/practice/curation path below remains
  // untouched for non-daily flows.
  const isDailyFlow = searchParams.get('flow') === 'daily';

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
    // The daily flow is handled entirely by <DailyFlow /> below; skip
    // the pool/curation fetcher so it doesn't race or flip loadStatus.
    if (isDailyFlow) {
      return () => { cancelled = true; };
    }
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
  }, [searchParams, loadState, saveState, fetchKey, isDailyFlow]);

  if (isDailyFlow) {
    return (
      <div data-testid="daily-flow">
        <DailyFlow />
      </div>
    );
  }

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
    />
  );
}

/** Small component to redirect home via effect. */
function RedirectToHome() {
  const navigate = useNavigate();
  useEffect(() => { navigate('/', { replace: true }); }, [navigate]);
  return null;
}

// Re-exported for backward-compat. Canonical location: shared/game/components/GameBoard.
export { GameBoard, type GameBoardProps } from '../shared/game/components/GameBoard';
