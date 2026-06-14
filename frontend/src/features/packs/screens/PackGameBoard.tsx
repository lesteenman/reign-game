import { useEffect, useMemo, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { GameBoard } from '@shared/game/components/GameBoard';
import { useGameStorage } from '@shared/game/hooks/useGameStorage';
import { EMPTY_HISTORY } from '@reign/core/storage';
import type { GameHistory, GameState } from '@reign/core/storage';
import type { CellState, Mode, PuzzleData } from '@reign/core/engine';
import { isMode } from '@reign/core/engine';
import type { PackServePuzzle } from '@features/packs/types/pack';

/**
 * Pack-flow grid host. Adapts one pack puzzle to the shared GameBoard
 * contract, mirroring DailyGameBoard:
 *   - the `PuzzleData` adapter (PackServePuzzle + pack mode → PuzzleData),
 *   - per-puzzle IndexedDB resume via the `pack` Flow Slot keyed
 *     `{slug}:{puzzleId}` (each puzzle in the pack gets its own canvas),
 *   - the solve-event delegate that hands (solution, elapsedMs) back to
 *     PackFlow's advance handler.
 *
 * The opt-in `onSolveDetected` GameBoard prop skips GameBoard's built-in
 * curation/practice solve UX (completion overlay + completion record +
 * status update) and hands control to PackFlow's state machine, which
 * owns mark-solved + advance + pack-complete.
 */

export interface PackGameBoardProps {
  slug: string;
  mode: string;
  size: number;
  puzzle: PackServePuzzle;
  /**
   * Fired when the player completes the current pack puzzle. `solution`
   * is a 0/1 marker grid; `elapsedMs` is total play time. PackFlow uses
   * the solve to record progress and advance; the values themselves are
   * not submitted anywhere (packs have no leaderboard for MVP).
   */
  onSolved: (solution: number[][], elapsedMs: number) => void;
}

/** Build an empty cells matrix sized to the puzzle grid. */
function emptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from<CellState>({ length: size }).fill('empty'),
  );
}

interface RestoredState {
  cells: CellState[][];
  history: GameHistory;
  timerElapsed: number;
  timerResumedAt: number | null;
  startedAt: number;
}

export function PackGameBoard({
  slug,
  mode,
  size,
  puzzle,
  onSolved,
}: PackGameBoardProps) {
  const navigate = useNavigate();
  const { saveState, loadState, clearState, addCompletion } = useGameStorage();

  // Adapter: derive PuzzleData from the pack puzzle + pack-level
  // size/mode. The pack manifest carries `mode` as a string; fall back
  // to 'standard' if it's somehow not a known Mode (defensive — the
  // backend only ships 'standard'/'double').
  const puzzleMode: Mode = isMode(mode) ? mode : 'standard';
  const puzzleData: PuzzleData = useMemo(
    () => ({
      puzzleId: puzzle.puzzleId,
      gridSize: size,
      mode: puzzleMode,
      regionMap: puzzle.regionMap,
      metadata: puzzle.metadata,
    }),
    [puzzle.puzzleId, puzzle.regionMap, puzzle.metadata, size, puzzleMode],
  );

  // Per-puzzle Flow Slot id: each puzzle in the pack keeps its own
  // in-progress canvas so navigating between them (or reloading
  // mid-puzzle) restores the right grid.
  const flowId = useMemo(
    () => `${slug}:${puzzle.puzzleId}`,
    [slug, puzzle.puzzleId],
  );

  const [restored, setRestored] = useState<RestoredState | null>(null);
  const [restoreReady, setRestoreReady] = useState(false);
  const [fallbackStartedAt] = useState(() => Date.now());

  // Each puzzle remounts (PackFlow keys PackGameBoard on puzzleId), so
  // restore state is fresh per puzzle — no in-effect reset needed.
  useEffect(() => {
    let cancelled = false;
    void loadState('pack', flowId)
      .then((saved: GameState | null) => {
        if (cancelled) return;
        if (saved && saved.status !== 'solved') {
          setRestored({
            cells: saved.cells,
            history: saved.history ?? EMPTY_HISTORY,
            timerElapsed: saved.timer.elapsedAtLastPause,
            timerResumedAt: saved.timer.lastResumedAt,
            startedAt: saved.startedAt,
          });
        }
        setRestoreReady(true);
      })
      .catch(() => {
        if (!cancelled) setRestoreReady(true);
      });
    return () => {
      cancelled = true;
    };
  }, [loadState, flowId]);

  const handleSolveDetected = useCallback(
    (solution: number[][], elapsedMs: number) => {
      onSolved(solution, elapsedMs);
    },
    [onSolved],
  );

  const handleBack = useCallback(() => {
    navigate('/packs');
  }, [navigate]);

  // GameBoard's onPlayAgain is unused in the delegated path (PackFlow
  // owns advance) — wire a no-op so the prop contract is satisfied.
  const handlePlayAgain = useCallback(() => {
    // intentional no-op — pack advance is owned by PackFlow.
  }, []);

  // Hold render until the restore lookup resolves so we don't render
  // empty cells then flip mid-mount (lesson 5 / lesson 2).
  if (!restoreReady) {
    return (
      <div
        data-testid="pack-game-board-restoring"
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: '16px',
          padding: '48px 0',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          fontWeight: 600,
          color: 'var(--color-body)',
        }}
      >
        <p style={{ margin: 0 }}>Restoring…</p>
      </div>
    );
  }

  const initialCells = restored?.cells ?? emptyCells(size);
  const initialHistory = restored?.history ?? EMPTY_HISTORY;
  const timerElapsed = restored?.timerElapsed ?? 0;
  const timerResumedAt = restored?.timerResumedAt ?? null;
  const startedAt = restored?.startedAt ?? fallbackStartedAt;

  return (
    <div
      data-testid="pack-game-board"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '16px',
        width: '100%',
      }}
    >
      <GameBoard
        key={puzzleData.puzzleId}
        puzzle={puzzleData}
        flowType="pack"
        flowId={flowId}
        initialCells={initialCells}
        initialHistory={initialHistory}
        timerElapsed={timerElapsed}
        timerResumedAt={timerResumedAt}
        startedAt={startedAt}
        navigate={navigate}
        saveState={saveState}
        clearState={clearState}
        addCompletion={addCompletion}
        onBack={handleBack}
        onPlayAgain={handlePlayAgain}
        onSolveDetected={handleSolveDetected}
      />
    </div>
  );
}
