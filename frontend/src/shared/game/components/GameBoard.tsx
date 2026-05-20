import { useState, useEffect, useCallback, useRef, useLayoutEffect, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useUser } from '@clerk/react';
import { Grid } from '@components/grid/Grid';
import { useGame } from '@hooks/useGame';
import { useTimer } from '@hooks/useTimer';
import { useUpdatePuzzleStatus } from '@shared/game/hooks/useUpdatePuzzleStatus';
import { PageShell } from '@shared/components/PageShell';
import { PrimaryButton, SecondaryButton, GhostButton } from '@shared/components/Button';
import { getClerkUserRole } from '@shared/auth/role';
import type { PuzzleData, CellState } from '@engine/types';
import type { FlowType, GameState, GameHistory, CompletionRecord } from '@storage/types';
import type { AdminVerdictSurfaceComponent } from '@shared/game/types/admin-verdict-surface';

/** Format seconds as MM:SS (under 1h) or H:MM:SS (1h+). Hour digit is
 * un-padded; presence of the leading `H:` itself signals hour-scale. */
export function formatTime(seconds: number): string {
  const s = seconds % 60;
  const m = Math.floor(seconds / 60) % 60;
  const h = Math.floor(seconds / 3600);
  const ss = String(s).padStart(2, '0');
  const mm = String(m).padStart(2, '0');
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

/** Format generation duration for display. */
function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
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
  /**
   * Optional server-issued anchor for the displayed elapsed time.
   * When set, the timer renders `Date.now() − assignedAt` and the
   * useTimer lifecycle (restore / start / pause-on-visibility / save)
   * is skipped — there is no client state to lose, and the displayed
   * value matches the leaderboard's server-authoritative elapsed time
   * (`submittedAt − assignedAt`; `assignedAt` is set once on first GET
   * and never overwritten).
   *
   * RFC3339 timestamp; parsed to epoch ms on receipt. Used only by
   * the daily flow — prevents the timer resetting to 00:00 on
   * navigate-back.
   */
  assignedAt?: string;
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
   * The daily flow uses this so DailyFlow's state machine takes over
   * (POST submit → PostCompletionScreen) instead of GameBoard's
   * built-in completion overlay + completion record write, which is
   * curation/practice-shaped.
   */
  onSolveDetected?: (solution: number[][], elapsedMs: number) => void;
  /**
   * Optional admin-only verdict surface component (curation flow). When
   * provided, GameBoard renders it at the completion overlay (after
   * solve, admin only) and inside the skip modal. When omitted (e.g.
   * daily flow), no admin verdict UI is shown — the Skip button is
   * also suppressed in that case because the modal would have no
   * surface to mount.
   *
   * Type contract lives in `@shared/game/types/admin-verdict-surface`
   * so GameBoard (shared) does not import from `features/curation/`
   * (which would violate the unidirectional `shared → features → app`
   * rule enforced by `import/no-restricted-paths` in eslint.config.js).
   * features/curation/components/VerdictSurface.tsx is the canonical
   * implementation; pages/GamePage.tsx wires it up for the curation
   * flow.
   */
  AdminVerdictSurface?: AdminVerdictSurfaceComponent;
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
  assignedAt,
  navigate,
  saveState,
  clearState,
  addCompletion,
  onBack,
  onPlayAgain,
  onSolveDetected,
  AdminVerdictSurface,
}: GameBoardProps) {
  const assignedAtMs = assignedAt ? new Date(assignedAt).getTime() : null;
  const isWallClockAnchored = assignedAtMs !== null && !Number.isNaN(assignedAtMs);
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

  const updatePuzzleStatusMutation = useUpdatePuzzleStatus();

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
  // moment the grid renders for non-daily flows (curation, practice).
  // Daily flows are wall-clock-anchored — they read elapsed straight
  // off `Date.now() − assignedAt` instead of restoring + resuming
  // useTimer state, so they skip this whole effect. See
  // `isWallClockAnchored` above.
  //
  // Ordering for the non-daily path: restore first so the elapsed
  // display reflects any saved progress; then start() (idempotent if
  // already running from a restored `lastResumedAt`). Skip start when
  // the puzzle is already solved — a no-op start would still be safe
  // (stop() sets `stopped`), but this avoids ticking the visible
  // "completed" state.
  useEffect(() => {
    if (isWallClockAnchored) return;
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

  // Wall-clock tick — re-renders the timer display every second when
  // anchored on `assignedAt`. No pause-on-hide; the leaderboard ticks
  // too (`serverElapsedMs = submittedAt − assignedAt`).
  const [, setWallClockTick] = useState(0);
  useEffect(() => {
    if (!isWallClockAnchored) return;
    if (isSolved) return;
    const id = setInterval(() => setWallClockTick((t) => t + 1), 1000);
    return () => clearInterval(id);
  }, [isWallClockAnchored, isSolved]);

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

  // Visibility change: pause/resume timer + save. Wall-clock-anchored
  // flows skip the pause/resume — the leaderboard counts wall time
  // regardless of tab visibility, so the displayed timer must too —
  // but still snapshot cells/history on hide.
  useEffect(() => {
    function handleVisibility() {
      if (document.hidden) {
        if (!isWallClockAnchored) {
          timerRef.current.pause();
        }
        void saveState(buildCurrentState(puzzle, flowType, flowId, cellsRef, historyRef, timerRef, isSolvedRef, startedAtRef));
      } else {
        if (!isWallClockAnchored && timerStartedRef.current && !isSolvedRef.current) {
          timerRef.current.start();
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility);
    return () => document.removeEventListener('visibilitychange', handleVisibility);
  }, [puzzle, saveState, isWallClockAnchored]);

  // Before unload: best-effort save. Wall-clock-anchored flows skip
  // the pause (nothing to pause) but still snapshot cells/history.
  useEffect(() => {
    function handleBeforeUnload() {
      if (!isWallClockAnchored) {
        timerRef.current.pause();
      }
      void saveState(buildCurrentState(puzzle, flowType, flowId, cellsRef, historyRef, timerRef, isSolvedRef, startedAtRef));
    }
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [puzzle, saveState, isWallClockAnchored]);

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
  // owns the post-solve experience.
  useEffect(() => {
    if (isSolved && !completionHandledRef.current) {
      completionHandledRef.current = true;
      let finalTimeSeconds: number;
      let elapsedMs: number;
      if (isWallClockAnchored && assignedAtMs !== null) {
        elapsedMs = Math.max(0, Date.now() - assignedAtMs);
        finalTimeSeconds = Math.floor(elapsedMs / 1000);
      } else {
        timer.stop();
        finalTimeSeconds = timer.elapsed;
        elapsedMs = finalTimeSeconds * 1000;
      }
      setCompletionTime(finalTimeSeconds);

      const delegate = onSolveDetectedRef.current;
      if (delegate) {
        const solution = cellsRef.current.map((row) =>
          row.map((cell) => (cell === 'marked' ? 1 : 0)),
        );
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
      void updatePuzzleStatusMutation.mutateAsync({ puzzleId: puzzle.puzzleId, size: puzzle.gridSize, mode: puzzle.mode, status: 'solved' }).catch(() => {});
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
        {formatTime(
          isWallClockAnchored && assignedAtMs !== null
            ? Math.max(0, Math.floor((Date.now() - assignedAtMs) / 1000))
            : timer.elapsed,
        )}
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
              {isAdmin && AdminVerdictSurface && (
                <AdminVerdictSurface
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
            on flowType so the admin role alone doesn't surface it.
            Also require AdminVerdictSurface — if the slot isn't wired
            up, clicking Skip would open a modal with nothing to mount. */}
        {isAdmin && !isSolved && flowType !== 'daily' && AdminVerdictSurface && (
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
            {AdminVerdictSurface && (
              <AdminVerdictSurface
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
            )}
          </div>
        </div>
      )}
    </>
  );

  return isDelegated ? board : <PageShell onBack={onBack}>{board}</PageShell>;
}
