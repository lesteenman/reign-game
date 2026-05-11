import { useState, useEffect, useCallback, useRef, useLayoutEffect, type ReactNode } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useUser } from '@clerk/react';
import { Grid } from '../components/grid/Grid';
import { useGame } from '../hooks/useGame';
import { useTimer } from '../hooks/useTimer';
import { useGameStorage } from '../hooks/useGameStorage';
import { fetchNextPuzzle, updatePuzzleStatus, NoPuzzlesAvailableError } from '../services/puzzleService';
import { createFreshGameState } from '../storage/utils';
import { PageShell } from '../components/common/PageShell';
import { PrimaryButton, SecondaryButton, GhostButton } from '../components/common/Button';
import { getClerkUserRole } from '../components/auth/role';
import { VerdictSurface } from '../components/game/VerdictSurface';
import type { Mode, PuzzleData, CellState } from '../engine/types';
import { isMode } from '../engine/types';
import type { FlowType, GameState, GameHistory, CompletionRecord } from '../storage/types';
import { EMPTY_HISTORY, buildCurationFlowId, parseFlowType } from '../storage/types';
import { DailyFlow } from './DailyFlow';

/** Format seconds as MM:SS. */
function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

/** Format generation duration for display. */
function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

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

  // R-8-02 chunk 3: when the URL says `?flow=daily`, delegate the
  // entire flow to DailyFlow. The existing pool/practice/curation
  // path below remains untouched for non-daily flows.
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

export interface GameBoardProps {
  puzzle: PuzzleData;
  flowType: FlowType;
  flowId: string;
  initialCells: CellState[][];
  initialHistory: GameHistory;
  timerElapsed: number;
  timerResumedAt: number | null;
  startedAt: number;
  navigate: ReturnType<typeof useNavigate>;
  saveState: (state: GameState) => Promise<void>;
  clearState: (flowType: FlowType, flowId: string) => Promise<void>;
  addCompletion: (record: CompletionRecord) => Promise<void>;
  onBack: () => void;
  onPlayAgain: () => void;
  /**
   * Optional opt-in solve-event delegate. When provided, GameBoard
   * skips its own post-solve actions (addCompletion / clearState /
   * updatePuzzleStatus / completion overlay) and fires this callback
   * instead — the caller takes ownership of the solve UX. The
   * `solution` payload is a 0/1 marker grid (1 where the player
   * placed a marker; 0 elsewhere) and `elapsedMs` is the timer's
   * elapsed milliseconds at the moment of solve.
   *
   * R-8-02 chunk 7: the daily flow uses this hook so DailyFlow's
   * state machine takes over (POST submit → PostCompletionScreen)
   * instead of GameBoard's built-in completion overlay + completion
   * record write, which is curation/practice-shaped.
   */
  onSolveDetected?: (solution: number[][], elapsedMs: number) => void;
}

/** Build the current GameState from refs, preserving the original startedAt. */
function buildCurrentState(
  puzzle: PuzzleData,
  flowType: FlowType,
  flowId: string,
  cellsRef: React.RefObject<CellState[][]>,
  historyRef: React.RefObject<GameHistory>,
  timerRef: React.RefObject<ReturnType<typeof useTimer>>,
  isSolvedRef: React.RefObject<boolean>,
  startedAtRef: React.RefObject<number>,
): GameState {
  return {
    id: `${flowType}:${flowId}`,
    flowType,
    flowId,
    puzzle,
    cells: cellsRef.current,
    timer: timerRef.current.timerState,
    status: isSolvedRef.current ? 'solved' : 'in-progress',
    startedAt: startedAtRef.current,
    history: historyRef.current,
  };
}

export function GameBoard({
  puzzle,
  flowType,
  flowId,
  initialCells,
  initialHistory,
  timerElapsed,
  timerResumedAt,
  startedAt,
  navigate,
  saveState,
  clearState,
  addCompletion,
  onBack,
  onPlayAgain,
  onSolveDetected,
}: GameBoardProps) {
  const [ready, setReady] = useState(false);
  useLayoutEffect(() => { setReady(true); }, []);

  const {
    cells,
    conflicts,
    isSolved,
    draggedCells,
    canUndo,
    canRedo,
    history,
    handlePointerDown,
    handleDragEnter,
    handlePointerUp,
    resetGame: originalResetGame,
    undo,
    redo,
  } = useGame(puzzle, initialCells, initialHistory);

  const timer = useTimer();
  const timerStartedRef = useRef(false);
  const completionHandledRef = useRef(false);
  const [showCompletion, setShowCompletion] = useState(false);
  const [completionTime, setCompletionTime] = useState(0);
  const [showSkipModal, setShowSkipModal] = useState(false);

  // Admin-only verdict surface + Skip button gating per FB-01 / FB-10.
  // Reads from the same Clerk hook used by UserMenu / ProtectedAdminRoute
  // so all role-gated UI uses one source of truth.
  const { user, isLoaded: userIsLoaded } = useUser();
  const isAdmin =
    userIsLoaded && getClerkUserRole(user?.publicMetadata) === 'admin';

  // Preserve the original startedAt across puzzle-state refreshes so a
  // mid-game re-render can't reset the elapsed-time anchor.
  const startedAtRef = useRef(startedAt);

  // Restore timer on mount, then start it. The timer ticks from the
  // moment the grid renders for ALL flows (daily, curation, practice).
  // Previously the timer started on first pointer interaction, which
  // both surprised users and let them read the puzzle for an arbitrary
  // amount of time before the clock began.
  //
  // Ordering: restore first so the elapsed display reflects any saved
  // progress; then start() (idempotent if already running from a
  // restored `lastResumedAt`). Skip if the puzzle is already solved —
  // a no-op start would still be safe (stop() sets `stopped`), but
  // this avoids ticking the visible "completed" state.
  useEffect(() => {
    if (timerElapsed > 0 || timerResumedAt !== null) {
      timer.restore({
        elapsedAtLastPause: timerElapsed,
        lastResumedAt: timerResumedAt,
      });
    }
    if (!isSolved) {
      timer.start();
      timerStartedRef.current = true;
    }
    // Only run on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Refs for debounced saves
  const cellsRef = useRef(cells);
  cellsRef.current = cells;
  const historyRef = useRef(history);
  historyRef.current = history;
  const timerRef = useRef(timer);
  timerRef.current = timer;
  const isSolvedRef = useRef(isSolved);
  isSolvedRef.current = isSolved;

  // Debounced save on cell or history changes
  useEffect(() => {
    if (!ready) return;
    const timeout = setTimeout(() => {
      void saveState(buildCurrentState(puzzle, flowType, flowId, cellsRef, historyRef, timerRef, isSolvedRef, startedAtRef));
    }, 200);
    return () => clearTimeout(timeout);
    // Save whenever cells or history change
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cells, history, ready]);

  // Visibility change: pause/resume timer + save
  useEffect(() => {
    function handleVisibility() {
      if (document.hidden) {
        timerRef.current.pause();
        void saveState(buildCurrentState(puzzle, flowType, flowId, cellsRef, historyRef, timerRef, isSolvedRef, startedAtRef));
      } else {
        if (timerStartedRef.current && !isSolvedRef.current) {
          timerRef.current.start();
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility);
    return () => document.removeEventListener('visibilitychange', handleVisibility);
  }, [puzzle, saveState]);

  // Before unload: best-effort save
  useEffect(() => {
    function handleBeforeUnload() {
      timerRef.current.pause();
      void saveState(buildCurrentState(puzzle, flowType, flowId, cellsRef, historyRef, timerRef, isSolvedRef, startedAtRef));
    }
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [puzzle, saveState]);

  // Stable ref for the optional solve-event delegate so the solve
  // effect's deps don't churn when the caller passes an inline arrow.
  const onSolveDetectedRef = useRef(onSolveDetected);
  onSolveDetectedRef.current = onSolveDetected;

  // Handle puzzle completion. Clear-on-solve (ST-07): the Flow Slot is
  // removed so the next visit to this `(flowType, flowId)` URL fetches
  // a fresh puzzle without a defensive read of a solved row.
  //
  // When the caller provides `onSolveDetected`, GameBoard delegates
  // the solve UX entirely: no completion overlay, no addCompletion/
  // clearState/updatePuzzleStatus side-effects. The caller (e.g.
  // DailyFlow via DailyGameBoard) receives the solution + elapsed and
  // owns the post-solve experience. R-8-02 chunk 7.
  useEffect(() => {
    if (isSolved && !completionHandledRef.current) {
      completionHandledRef.current = true;
      timer.stop();
      const finalTimeSeconds = timer.elapsed;
      setCompletionTime(finalTimeSeconds);

      const delegate = onSolveDetectedRef.current;
      if (delegate) {
        const solution = cellsRef.current.map((row) =>
          row.map((cell) => (cell === 'marked' ? 1 : 0)),
        );
        const elapsedMs = finalTimeSeconds * 1000;
        delegate(solution, elapsedMs);
        return;
      }

      setShowCompletion(true);
      void addCompletion({
        puzzleId: puzzle.puzzleId,
        time: finalTimeSeconds,
        completedAt: Date.now(),
      });
      void clearState(flowType, flowId);
      void updatePuzzleStatus(puzzle.puzzleId, puzzle.gridSize, puzzle.mode, 'solved');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSolved]);

  // Keyboard shortcuts: Ctrl/Cmd+Z = undo, Ctrl/Cmd+Shift+Z = redo.
  // Skip when focus is inside an input/textarea so typing Z works normally.
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key !== 'z' && e.key !== 'Z') return;
      const target = e.target as HTMLElement | null;
      if (target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable) {
        return;
      }
      e.preventDefault();
      if (e.shiftKey) {
        redo();
      } else {
        undo();
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [undo, redo]);

  const resetGame = useCallback(() => {
    originalResetGame();
    completionHandledRef.current = false;
    setShowCompletion(false);
    setCompletionTime(0);
  }, [originalResetGame]);

  const handlePlayAgain = onPlayAgain;

  const handleGoHome = useCallback(() => {
    navigate('/');
  }, [navigate]);

  // Delegated mode: when the caller wires `onSolveDetected` it also
  // owns the page chrome (DailyFlow → PageShell with the subtitle).
  // GameBoard rendering its own PageShell here would stack two "Reign"
  // headers. Standalone mode (curation/practice, no delegate) still
  // wraps itself so the page is self-contained.
  //
  // Note: a wrapping component identity must be stable across renders.
  // Inlining a `ShellWrapper` arrow component into the render would
  // re-create the component type each render, forcing React to unmount
  // and remount everything beneath it on every tick — which silently
  // resets `useGame`'s reducer state. Emit one of two render trees
  // straight from the JSX instead.
  const isDelegated = onSolveDetected !== undefined;

  const board: ReactNode = (
    <>
      {/* Timer display */}
      <div
        data-testid="timer-display"
        style={{
          alignSelf: 'flex-end',
          fontFamily: '"Space Mono", ui-monospace, monospace',
          fontSize: '1.5rem',
          fontWeight: 700,
          fontVariantNumeric: 'tabular-nums',
          color: 'var(--color-muted)',
          maxWidth: 600,
          width: '100%',
          textAlign: 'right',
        }}
      >
        {formatTime(timer.elapsed)}
      </div>

      {/* Grid area with completion overlay */}
      <div style={{ position: 'relative', width: '100%', maxWidth: 600, display: 'flex', justifyContent: 'center' }}>
        <Grid
          puzzle={puzzle}
          cells={cells}
          conflicts={conflicts}
          isSolved={isSolved}
          draggedCells={draggedCells}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          onDragEnter={handleDragEnter}
        />

        {/* Completion overlay */}
        {ready && showCompletion && (
          <div
            data-testid="completion-overlay"
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              backgroundColor: 'rgba(0, 0, 0, 0.5)',
              backdropFilter: 'blur(4px)',
              borderRadius: 'var(--radius)',
              zIndex: 50,
            }}
          >
            <div
              style={{
                // Completion overlay card — 32px padding matches
                // BRAND_GUIDELINES §5.6 prominent modal.
                backgroundColor: 'var(--color-surface)',
                border: '2px solid var(--color-ink)',
                borderRadius: 'var(--radius)',
                padding: '32px',
                boxShadow: '0 4px 0 var(--color-ink), 0 12px 32px rgba(0,0,0,0.08)',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: '16px',
                maxWidth: '90%',
              }}
            >
              <h2
                style={{
                  fontSize: '1.5rem',
                  fontWeight: 700,
                  margin: 0,
                  color: 'var(--color-success)',
                }}
              >
                Puzzle Complete!
              </h2>
              <p
                style={{
                  fontFamily: '"Space Mono", ui-monospace, monospace',
                  fontSize: '1.5rem',
                  fontWeight: 700,
                  fontVariantNumeric: 'tabular-nums',
                  margin: 0,
                }}
              >
                {formatTime(completionTime)}
              </p>
              <div style={{ display: 'flex', gap: '12px' }}>
                <PrimaryButton onClick={handlePlayAgain}>Play Again</PrimaryButton>
                <SecondaryButton onClick={handleGoHome}>Home</SecondaryButton>
              </div>
              {isAdmin && (
                <VerdictSurface
                  variant="completion"
                  outcome="solved"
                  puzzleId={puzzle.puzzleId}
                  size={puzzle.gridSize}
                  mode={puzzle.mode}
                  playTimeMs={completionTime * 1000}
                />
              )}
            </div>
          </div>
        )}
      </div>

      {/* Puzzle metadata display */}
      {puzzle.metadata && (
        <div
          data-testid="puzzle-metadata"
          style={{
            fontFamily: '"Nunito Sans", system-ui, sans-serif',
            fontSize: '0.75rem',
            color: 'var(--color-muted)',
            textAlign: 'center',
            maxWidth: 600,
            width: '100%',
          }}
        >
          difficulty {puzzle.metadata.difficulty} / {formatDuration(puzzle.metadata.generationDurationMs)}
          {puzzle.metadata.seed && (
            <>
              {' / '}
              <span data-testid="puzzle-seed" title="Generator seed — paste into `task reproduce -- --seed=<this> --n=<size> --k=<k>` to regenerate.">
                seed {puzzle.metadata.seed}
              </span>
            </>
          )}
        </div>
      )}

      <div
        style={{
          display: 'flex',
          gap: '12px',
          flexWrap: 'wrap',
          justifyContent: 'center',
        }}
      >
        <SecondaryButton
          onClick={undo}
          disabled={!canUndo}
          data-testid="undo-button"
          aria-label="Undo (Ctrl+Z)"
        >
          Undo
        </SecondaryButton>
        <SecondaryButton
          onClick={redo}
          disabled={!canRedo}
          data-testid="redo-button"
          aria-label="Redo (Ctrl+Shift+Z)"
        >
          Redo
        </SecondaryButton>
        <SecondaryButton onClick={resetGame} data-testid="reset-button">
          Reset
        </SecondaryButton>
        {/* Skip is the admin verdict surface for curation/practice. The
            daily flow doesn't expose a verdict step — admins playing
            the daily play it like everyone else. Gate the Skip button
            on flowType so the admin role alone doesn't surface it. */}
        {isAdmin && !isSolved && flowType !== 'daily' && (
          <GhostButton
            onClick={() => setShowSkipModal(true)}
            data-testid="skip-button"
            aria-label="Skip puzzle (abandon and rate)"
          >
            Skip puzzle
          </GhostButton>
        )}
      </div>

      {/* Skip modal — admin-only. Mounts <VerdictSurface variant="skip">
          inside the BRAND_GUIDELINES §5.6 modal pattern. Cancel
          dismisses; "I hate this" / "Just skip" navigate forward. */}
      {showSkipModal && (
        <div
          data-testid="skip-modal"
          style={{
            position: 'fixed',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            backgroundColor: 'rgba(0, 0, 0, 0.5)',
            backdropFilter: 'blur(4px)',
            zIndex: 50,
          }}
        >
          <div
            style={{
              // Skip modal card — 24px padding (vs 32px on the
              // completion overlay) implements FB-02's de-emphasized
              // chrome for the skip variant.
              backgroundColor: 'var(--color-surface)',
              border: '2px solid var(--color-ink)',
              borderRadius: 'var(--radius)',
              padding: '24px',
              boxShadow: '0 4px 0 var(--color-ink), 0 12px 32px rgba(0,0,0,0.08)',
              maxWidth: '90%',
              minWidth: '280px',
            }}
          >
            <VerdictSurface
              variant="skip"
              outcome="skipped"
              puzzleId={puzzle.puzzleId}
              size={puzzle.gridSize}
              mode={puzzle.mode}
              playTimeMs={timer.elapsed * 1000}
              onDismiss={() => setShowSkipModal(false)}
              onAfterVerdict={() => {
                setShowSkipModal(false);
                navigate('/curation');
              }}
            />
          </div>
        </div>
      )}
    </>
  );

  return isDelegated ? board : <PageShell onBack={onBack}>{board}</PageShell>;
}
