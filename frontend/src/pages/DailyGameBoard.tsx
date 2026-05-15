import { useEffect, useMemo, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { GameBoard } from './GamePage';
import { useGameStorage } from '../hooks/useGameStorage';
import { EMPTY_HISTORY } from '../storage/types';
import type { GameHistory, GameState } from '../storage/types';
import type { CellState, PuzzleData } from '../engine/types';
import type { DailyPuzzlePayload } from '../services/dailyService';

/**
 * Daily-flow grid host (R-8-02 chunk 7 — real GameBoard integration).
 *
 * Adapts the daily-payload shape to the GameBoard contract used by the
 * curation flow. Owns:
 *   - the `PuzzleData` adapter (DailyPuzzlePayload → PuzzleData),
 *   - per-flow IndexedDB resume (DP-32: refresh / second device picks
 *     up the same puzzle and the same `assignedAt`),
 *   - the solve-event delegate that converts GameBoard's internal
 *     marker grid + elapsed timer into the (solution, elapsedMs)
 *     tuple DailyFlow's submit handler expects.
 *
 * The opt-in `onSolveDetected` GameBoard prop lets us skip the built-in
 * curation/practice solve UX (completion overlay, addCompletion,
 * clearState, updatePuzzleStatus) and hand control back to DailyFlow's
 * state machine, which owns the daily POST → PostCompletionScreen
 * transition.
 */

export interface DailyGameBoardProps {
  payload: DailyPuzzlePayload;
  /**
   * Fired when the player completes the puzzle. The `solution` is a
   * 0/1 marker grid (1 = player placed a marker; 0 = empty/excluded);
   * `elapsedMs` is total play time in milliseconds. Both flow into
   * `submitDailyResult` per DP-29.
   */
  onSolved: (solution: number[][], elapsedMs: number) => void;
}

/** Extracts YYYY-MM-DD (UTC) from an RFC3339 timestamp. Mirrors the
 *  helper in DailyFlow so the per-flow storage key agrees on both
 *  sides. */
function dateFromAssignedAt(assignedAt: string): string {
  return new Date(assignedAt).toISOString().slice(0, 10);
}

/** Build empty cells matrix sized to the daily grid. Used as the
 *  fallback when no in-progress state is restored from IndexedDB. */
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

export function DailyGameBoard({ payload, onSolved }: DailyGameBoardProps) {
  const navigate = useNavigate();
  const { saveState, loadState, clearState, addCompletion } = useGameStorage();

  // Adapter: derive a PuzzleData object from the daily payload. The
  // daily mode is fixed to 'standard' per design D1 — daily is always
  // 9×9 Standard for now. `metadata` is intentionally omitted: the
  // engine's PuzzleMetadata type is strict (difficulty, tierCounts,
  // ...) and the daily payload doesn't carry those fields. Adding an
  // `assignedAt` extension would ripple through every consumer of
  // PuzzleMetadata for no current benefit; the payload's assignedAt
  // is read directly from `payload` where needed instead.
  const puzzle: PuzzleData = useMemo(
    () => ({
      puzzleId: payload.puzzleId,
      gridSize: payload.grid,
      mode: 'standard',
      regionMap: payload.regions,
    }),
    [payload.puzzleId, payload.grid, payload.regions],
  );

  const flowId = useMemo(
    () => dateFromAssignedAt(payload.assignedAt),
    [payload.assignedAt],
  );

  // Restore in-progress state from the per-flow IDB slot on mount
  // (DP-32). A solved row never reaches this component — DailyFlow's
  // chunk-7 short-circuit lifts the loading-state to `solved` before
  // DailyGameBoard would render. Defensively, we still treat any
  // unexpected `status === 'solved'` row as "no restore" so the player
  // gets a fresh canvas rather than a frozen pre-solved grid.
  const [restored, setRestored] = useState<RestoredState | null>(null);
  const [restoreReady, setRestoreReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void loadState('daily', flowId).then((saved: GameState | null) => {
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
    }).catch(() => {
      if (!cancelled) setRestoreReady(true);
    });
    return () => {
      cancelled = true;
    };
  }, [loadState, flowId]);

  // Solve delegate — translates GameBoard's internal solve event
  // (marker grid + ms elapsed) into the (solution, elapsedMs) tuple
  // DailyFlow expects. Stable identity so GameBoard's solve effect
  // doesn't re-fire on render.
  const handleSolveDetected = useCallback(
    (solution: number[][], elapsedMs: number) => {
      onSolved(solution, elapsedMs);
    },
    [onSolved],
  );

  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  // Daily can't be replayed — the next puzzle drops at next UTC
  // midnight. Wire onPlayAgain as a no-op rather than a navigate so
  // the (currently disabled, gated by `onSolveDetected`) Play Again
  // affordance can't accidentally bounce the player anywhere weird.
  const handlePlayAgain = useCallback(() => {
    // intentional no-op — daily flow doesn't support replay.
  }, []);

  // Hold the render until the restore lookup resolves so we don't
  // render with empty cells then flip mid-mount (avoids the
  // first-paint flicker called out in lesson 5).
  if (!restoreReady) {
    return (
      <div
        data-testid="daily-game-board-restoring"
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

  const initialCells = restored?.cells ?? emptyCells(payload.grid);
  const initialHistory = restored?.history ?? EMPTY_HISTORY;
  const timerElapsed = restored?.timerElapsed ?? 0;
  const timerResumedAt = restored?.timerResumedAt ?? null;
  const startedAt = restored?.startedAt ?? Date.now();

  // Match PageShell's flex-column layout. Without it, GameBoard's
  // delegated render (fragment of timer / grid / metadata / actions)
  // collapses against this wrapper with no inter-element spacing,
  // because PageShell's gap only applies to its own direct children.
  return (
    <div
      data-testid="daily-game-board"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '16px',
        width: '100%',
      }}
    >
      <GameBoard
        key={puzzle.puzzleId}
        puzzle={puzzle}
        flowType="daily"
        flowId={flowId}
        initialCells={initialCells}
        initialHistory={initialHistory}
        timerElapsed={timerElapsed}
        timerResumedAt={timerResumedAt}
        startedAt={startedAt}
        assignedAt={payload.assignedAt}
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
