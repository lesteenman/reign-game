import { useState, useEffect, useCallback, useRef, useLayoutEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Grid } from '../components/grid/Grid';
import { useGame } from '../hooks/useGame';
import { useTimer } from '../hooks/useTimer';
import { useGameStorage } from '../hooks/useGameStorage';
import { generatePuzzle } from '../services/puzzleService';
import type { PuzzleData, CellState } from '../engine/types';
import type { GameState, CompletionRecord } from '../storage/types';

/** Format seconds as MM:SS. */
function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

type LoadState =
  | { status: 'loading' }
  | { status: 'ready'; puzzle: PuzzleData; initialCells: CellState[][]; timerElapsed: number; timerResumedAt: number | null }
  | { status: 'no-state' };

/**
 * Main gameplay page. Loads persisted state from IndexedDB,
 * manages the timer, and handles puzzle completion.
 */
export function GamePage() {
  const navigate = useNavigate();
  const { loadState, saveState, addCompletion } = useGameStorage();
  const [loadStatus, setLoadStatus] = useState<LoadState>({ status: 'loading' });

  useEffect(() => {
    let cancelled = false;
    void loadState().then((saved) => {
      if (cancelled) return;
      if (!saved) {
        setLoadStatus({ status: 'no-state' });
        return;
      }
      setLoadStatus({
        status: 'ready',
        puzzle: saved.puzzle,
        initialCells: saved.cells,
        timerElapsed: saved.timer.elapsedAtLastPause,
        timerResumedAt: saved.timer.lastResumedAt,
      });
    }).catch(() => {
      if (!cancelled) setLoadStatus({ status: 'no-state' });
    });
    return () => { cancelled = true; };
  }, [loadState]);

  if (loadStatus.status === 'loading') {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          backgroundColor: 'var(--color-background)',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          color: 'var(--color-ink)',
          fontWeight: 600,
        }}
      >
        Loading...
      </div>
    );
  }

  if (loadStatus.status === 'no-state') {
    // Redirect to landing
    return <RedirectToHome />;
  }

  return (
    <GameBoard
      key={loadStatus.puzzle.puzzleId}
      puzzle={loadStatus.puzzle}
      initialCells={loadStatus.initialCells}
      timerElapsed={loadStatus.timerElapsed}
      timerResumedAt={loadStatus.timerResumedAt}
      navigate={navigate}
      saveState={saveState}
      addCompletion={addCompletion}
    />
  );
}

/** Small component to redirect home via effect. */
function RedirectToHome() {
  const navigate = useNavigate();
  useEffect(() => { navigate('/', { replace: true }); }, [navigate]);
  return null;
}

interface GameBoardProps {
  puzzle: PuzzleData;
  initialCells: CellState[][];
  timerElapsed: number;
  timerResumedAt: number | null;
  navigate: ReturnType<typeof useNavigate>;
  saveState: (state: GameState) => Promise<void>;
  addCompletion: (record: CompletionRecord) => Promise<void>;
}

function GameBoard({
  puzzle,
  initialCells,
  timerElapsed,
  timerResumedAt,
  navigate,
  saveState,
  addCompletion,
}: GameBoardProps) {
  const [ready, setReady] = useState(false);
  useLayoutEffect(() => { setReady(true); }, []);

  const {
    cells,
    conflicts,
    isSolved,
    draggedCells,
    handlePointerDown,
    handleDragEnter,
    handlePointerUp: originalPointerUp,
    resetGame: originalResetGame,
  } = useGame(puzzle, initialCells);

  const timer = useTimer();
  const timerStartedRef = useRef(false);
  const completionHandledRef = useRef(false);
  const [showCompletion, setShowCompletion] = useState(false);
  const [completionTime, setCompletionTime] = useState(0);

  // Restore timer on mount
  useEffect(() => {
    if (timerElapsed > 0 || timerResumedAt !== null) {
      timer.restore({
        elapsedAtLastPause: timerElapsed,
        lastResumedAt: timerResumedAt,
      });
      timerStartedRef.current = true;
    }
    // Only run on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Debounced save on cell changes
  const cellsRef = useRef(cells);
  cellsRef.current = cells;
  const timerRef = useRef(timer);
  timerRef.current = timer;
  const isSolvedRef = useRef(isSolved);
  isSolvedRef.current = isSolved;

  useEffect(() => {
    if (!ready) return;
    const timeout = setTimeout(() => {
      void saveState({
        id: 'current',
        puzzle,
        cells: cellsRef.current,
        timer: timerRef.current.timerState,
        status: isSolvedRef.current ? 'solved' : 'in-progress',
        startedAt: Date.now(),
      });
    }, 200);
    return () => clearTimeout(timeout);
    // Save whenever cells change
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cells, ready]);

  // Visibility change: pause/resume timer + save
  useEffect(() => {
    function handleVisibility() {
      if (document.hidden) {
        timerRef.current.pause();
        void saveState({
          id: 'current',
          puzzle,
          cells: cellsRef.current,
          timer: timerRef.current.timerState,
          status: isSolvedRef.current ? 'solved' : 'in-progress',
          startedAt: Date.now(),
        });
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
      // Pause timer first to get accurate state
      timerRef.current.pause();
      const state: GameState = {
        id: 'current',
        puzzle,
        cells: cellsRef.current,
        timer: timerRef.current.timerState,
        status: isSolvedRef.current ? 'solved' : 'in-progress',
        startedAt: Date.now(),
      };
      // Best effort: use sendBeacon or just fire-and-forget
      void saveState(state);
    }
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [puzzle, saveState]);

  // Handle puzzle completion
  useEffect(() => {
    if (isSolved && !completionHandledRef.current) {
      completionHandledRef.current = true;
      timer.stop();
      const finalTime = timer.elapsed;
      setCompletionTime(finalTime);
      setShowCompletion(true);
      // Save final state + completion record
      void saveState({
        id: 'current',
        puzzle,
        cells,
        timer: timer.timerState,
        status: 'solved',
        startedAt: Date.now(),
      });
      void addCompletion({
        puzzleId: puzzle.puzzleId,
        time: finalTime,
        completedAt: Date.now(),
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSolved]);

  // Wrap pointerUp to start timer on first interaction
  const handlePointerUp = useCallback(() => {
    originalPointerUp();
    if (!timerStartedRef.current && !isSolvedRef.current) {
      timerStartedRef.current = true;
      timerRef.current.start();
    }
  }, [originalPointerUp]);

  const resetGame = useCallback(() => {
    originalResetGame();
    timer.reset();
    timerStartedRef.current = false;
    completionHandledRef.current = false;
    setShowCompletion(false);
    setCompletionTime(0);
  }, [originalResetGame, timer]);

  const handlePlayAgain = useCallback(async () => {
    try {
      const newPuzzle = await generatePuzzle(5, 'standard');
      const gameState: GameState = {
        id: 'current',
        puzzle: newPuzzle,
        cells: Array.from({ length: newPuzzle.gridSize }, () =>
          Array.from({ length: newPuzzle.gridSize }, () => 'empty' as const),
        ),
        timer: { elapsedAtLastPause: 0, lastResumedAt: null },
        status: 'in-progress',
        startedAt: Date.now(),
      };
      await saveState(gameState);
      // Force full remount by navigating
      navigate('/play', { replace: true });
      // Since navigate to same route won't remount, we reload
      window.location.reload();
    } catch {
      // If fetch fails, just stay on completion screen
    }
  }, [saveState, navigate]);

  const handleGoHome = useCallback(() => {
    navigate('/');
  }, [navigate]);

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
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
                <button
                  type="button"
                  onClick={() => void handlePlayAgain()}
                  style={{
                    padding: '12px 24px',
                    backgroundColor: 'var(--color-accent)',
                    color: 'var(--color-on-accent)',
                    border: '2px solid var(--color-ink)',
                    borderRadius: 'var(--radius)',
                    boxShadow: '0 3px 0 var(--color-accent-shadow)',
                    fontFamily: '"Nunito Sans", system-ui, sans-serif',
                    fontWeight: 700,
                    fontSize: '1rem',
                    cursor: 'pointer',
                  }}
                >
                  Play Again
                </button>
                <button
                  type="button"
                  onClick={handleGoHome}
                  style={{
                    padding: '12px 24px',
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
                  Home
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Reset button */}
      <button
        type="button"
        onClick={resetGame}
        hidden={!ready}
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
        Reset
      </button>
    </div>
  );
}
