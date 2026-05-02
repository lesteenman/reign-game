import { useCallback } from 'react';
import { PrimaryButton } from '../components/common/Button';
import type { DailyPuzzlePayload } from '../services/dailyService';

/**
 * Daily-flow grid host (R-8-02 chunk 6 placeholder shape).
 *
 * Owns the player-facing grid + timer for the active daily puzzle and
 * fires `onSolved(solution, elapsedMs)` when the player completes the
 * board. Chunk 6 lands the wiring contract (props + callback) without
 * the full grid rewire — a follow-up chunk replaces the placeholder
 * body with the real `<Grid />` + `useGame` + `useTimer` integration
 * (mirroring `GameBoard` in GamePage.tsx) once the daily-storage hooks
 * are in place.
 *
 * The placeholder renders the puzzle metadata and a "Mark as solved"
 * affordance so the submit pipeline (DailyFlow → service → storage →
 * PostCompletionScreen) is end-to-end exercisable in dev/e2e while the
 * grid renderer is being plumbed.
 */

export interface DailyGameBoardProps {
  payload: DailyPuzzlePayload;
  /**
   * Fired when the player completes the puzzle. The `solution` is the
   * marker grid (rows × cols of region indices); `elapsedMs` is the
   * total play time the timer measured. Both flow straight into
   * `submitDailyResult` per DP-29.
   */
  onSolved: (solution: number[][], elapsedMs: number) => void;
}

export function DailyGameBoard({ payload, onSolved }: DailyGameBoardProps) {
  // Placeholder solve trigger — the real grid replaces this body in a
  // follow-up chunk. The solution + elapsed values mirror what the
  // production `useGame` + `useTimer` would emit (regions grid, ms
  // since first interaction).
  const handlePlaceholderSolve = useCallback(() => {
    onSolved(payload.regions, 0);
  }, [onSolved, payload.regions]);

  return (
    <div
      data-testid="daily-game-board"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '16px',
        padding: '32px 24px',
        fontFamily: '"Nunito Sans", system-ui, sans-serif',
        color: 'var(--color-body)',
        textAlign: 'center',
      }}
    >
      <p style={{ margin: 0, fontWeight: 600 }}>
        Daily puzzle: {payload.puzzleId}
      </p>
      <p style={{ margin: 0, fontSize: '0.875rem', color: 'var(--color-muted)' }}>
        Grid renderer wires in a follow-up chunk.
      </p>
      <PrimaryButton
        onClick={handlePlaceholderSolve}
        data-testid="daily-placeholder-solve"
      >
        Mark as solved
      </PrimaryButton>
    </div>
  );
}
