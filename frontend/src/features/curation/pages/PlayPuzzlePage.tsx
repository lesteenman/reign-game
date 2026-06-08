import { useEffect, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import {
  useCurationPuzzle,
  NoPuzzlesAvailableError,
} from '@features/curation/hooks/useCurationPuzzle';
import { PageShell } from '@shared/components/PageShell';
import { SecondaryButton } from '@shared/components/Button';
import { GameBoard } from '@shared/game/components/GameBoard';
import type { Mode } from '@reign/core/engine';
import { isMode } from '@reign/core/engine';
import { parseFlowType } from '@reign/core/storage';
import { VerdictSurface } from '@features/curation/components/VerdictSurface';

/**
 * Curation/practice play route. Reads `?flow=curation&size=N&mode=M`
 * from the URL, loads a saved game from IndexedDB or fetches a fresh
 * puzzle from the pool (orchestrated by `useCurationPuzzle`), then
 * mounts `<GameBoard>` with the admin `<VerdictSurface>` slot wired
 * up. An unrecognized or missing `flow` redirects home (ST-11).
 *
 * The `?flow=daily` branch lives at the router level (`src/app/router.tsx`)
 * so this page never imports `features/daily/`. Was `pages/GamePage.tsx`
 * before #176; load-cascade was hand-rolled `useState<LoadState>` +
 * `useEffect` before this slice (#176 Services→TanStack, curation half).
 */
export function PlayPuzzlePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { saveState, clearState, addCompletion } = useGameStorage();

  const flowType = parseFlowType(searchParams.get('flow'));
  const size = Number(searchParams.get('size')) || 5;
  const modeParam = searchParams.get('mode');
  const mode: Mode = isMode(modeParam) ? modeParam : 'standard';

  const { data, error, isPending, refetch } = useCurationPuzzle({
    flowType,
    size,
    mode,
  });

  /** Force a re-run of the load+fetch cascade (Retry / Play Again). */
  const retryFetch = useCallback(() => {
    void refetch();
  }, [refetch]);

  /** Navigate back to home without clearing game state. */
  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  // Invalid / missing flow → redirect home (ST-11). The hook is
  // `enabled: false` in this branch so no fetch / loadState runs.
  if (flowType === null) {
    return <RedirectToHome />;
  }

  if (isPending) {
    return (
      <PageShell onBack={handleBack}>
        <div style={{ padding: '48px 0', fontWeight: 600 }}>Loading...</div>
      </PageShell>
    );
  }

  if (error instanceof NoPuzzlesAvailableError) {
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

  if (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
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
            Failed to load puzzle: {message}
          </p>
          <SecondaryButton onClick={retryFetch}>
            Try Again
          </SecondaryButton>
        </div>
      </PageShell>
    );
  }

  return (
    <GameBoard
      key={data.puzzle.puzzleId}
      puzzle={data.puzzle}
      flowType={data.flowType}
      flowId={data.flowId}
      initialCells={data.initialCells}
      initialHistory={data.initialHistory}
      timerElapsed={data.timerElapsed}
      timerResumedAt={data.timerResumedAt}
      startedAt={data.startedAt}
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
  useEffect(() => {
    navigate('/', { replace: true });
  }, [navigate]);
  return null;
}
