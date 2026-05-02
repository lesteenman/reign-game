// TODO(R-8-02 follow-up): Recycle-day copy is spec'd in DP-25 but
// the backend responses do not yet expose recycle metadata
// (DP-09 / POST response don't carry an isRecycle flag).
// When backend adds the metadata, surface it as an optional prop
// `isRecycle?: boolean` and render an additional line:
//   "Today's puzzle is a recycle of yesterday's — fresh puzzle
//   tomorrow."
// Spec reference: design.md §7 residual risk mitigation.

import { useEffect, useState, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '../components/common/PageShell';
import { SecondaryButton } from '../components/common/Button';

export interface PostCompletionScreenProps {
  serverElapsedMs: number;
  submittedAt: string; // RFC3339
  leaderboardRank?: number; // present only for signed-in (DP-13)
  /**
   * Optional injection point for "now" so countdown logic is
   * deterministic in tests. Defaults to `new Date()`.
   */
  now?: Date;
}

/** Pads a non-negative integer to a 2-digit string. */
function pad2(n: number): string {
  return n.toString().padStart(2, '0');
}

/**
 * Formats a positive duration in milliseconds.
 *
 *   < 1 hour:  m:ss   (e.g. "1:15", "0:05")
 *   ≥ 1 hour:  h:mm:ss (e.g. "1:30:00")
 *
 * Mirrors the timer convention from BRAND_GUIDELINES §5.3.
 */
function formatSolveTime(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${pad2(minutes)}:${pad2(seconds)}`;
  }
  return `${minutes}:${pad2(seconds)}`;
}

/** Returns the next UTC midnight strictly after `from`. */
function nextUtcMidnight(from: Date): Date {
  const next = new Date(
    Date.UTC(
      from.getUTCFullYear(),
      from.getUTCMonth(),
      from.getUTCDate() + 1,
      0,
      0,
      0,
      0,
    ),
  );
  return next;
}

/** Formats a non-negative ms duration as HH:MM:SS. */
function formatCountdown(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return `${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`;
}

/**
 * Daily post-completion screen (DP-25).
 *
 * Pure presentational component — no service calls, no state machine.
 * Renders solve time, optional leaderboard rank (signed-in only),
 * the localized submission timestamp, and a live-updating countdown
 * to the next UTC midnight.
 */
export function PostCompletionScreen({
  serverElapsedMs,
  submittedAt,
  leaderboardRank,
  now,
}: PostCompletionScreenProps) {
  const navigate = useNavigate();
  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  const initialNow = useMemo(() => now ?? new Date(), [now]);
  const [currentTime, setCurrentTime] = useState<Date>(initialNow);

  useEffect(() => {
    setCurrentTime(initialNow);
    const interval = setInterval(() => {
      setCurrentTime((prev) => new Date(prev.getTime() + 1000));
    }, 1000);
    return () => clearInterval(interval);
  }, [initialNow]);

  const target = useMemo(() => nextUtcMidnight(currentTime), [currentTime]);
  const remainingMs = target.getTime() - currentTime.getTime();
  const countdownText = formatCountdown(remainingMs);

  const submittedAtDate = useMemo(() => new Date(submittedAt), [submittedAt]);
  const submittedAtText = submittedAtDate.toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  });

  return (
    <PageShell onBack={handleBack}>
      <div
        data-testid="daily-post-completion"
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: '24px',
          backgroundColor: 'var(--color-surface)',
          border: '2px solid var(--color-ink)',
          borderRadius: 'var(--radius)',
          boxShadow: '0 3px 0 var(--color-ink)',
          padding: '32px 24px',
          maxWidth: 480,
          width: '100%',
          margin: '24px auto',
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          textAlign: 'center',
        }}
      >
        <h1
          style={{
            margin: 0,
            fontSize: '1.875rem',
            lineHeight: 1.27,
            fontWeight: 800,
            color: 'var(--color-ink)',
            letterSpacing: '-0.01em',
          }}
        >
          Done for today
        </h1>

        <div
          data-testid="daily-solve-time"
          style={{
            fontFamily: '"Space Mono", ui-monospace, monospace',
            fontSize: '2.25rem',
            fontWeight: 700,
            color: 'var(--color-ink)',
            fontVariantNumeric: 'tabular-nums',
            lineHeight: 1.22,
          }}
          aria-label={`Solve time ${formatSolveTime(serverElapsedMs)}`}
        >
          {formatSolveTime(serverElapsedMs)}
        </div>

        {typeof leaderboardRank === 'number' && (
          <p
            data-testid="daily-leaderboard-rank"
            style={{
              margin: 0,
              fontSize: '1.125rem',
              fontWeight: 600,
              color: 'var(--color-body)',
            }}
          >
            Today&apos;s rank:{' '}
            <span style={{ color: 'var(--color-accent)', fontWeight: 700 }}>
              #{leaderboardRank}
            </span>
          </p>
        )}

        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '4px',
            marginTop: '8px',
          }}
        >
          <span
            style={{
              fontSize: '0.875rem',
              color: 'var(--color-muted)',
              fontWeight: 500,
            }}
          >
            Next puzzle in
          </span>
          <span
            data-testid="daily-countdown"
            style={{
              fontFamily: '"Space Mono", ui-monospace, monospace',
              fontSize: '1.5rem',
              fontWeight: 700,
              color: 'var(--color-muted)',
              fontVariantNumeric: 'tabular-nums',
            }}
            aria-live="polite"
          >
            {countdownText}
          </span>
        </div>

        <p
          data-testid="daily-submitted-at"
          style={{
            margin: 0,
            fontSize: '0.75rem',
            color: 'var(--color-muted)',
            fontWeight: 400,
          }}
        >
          Submitted at {submittedAtText}
        </p>

        <SecondaryButton onClick={handleBack} data-testid="daily-back-home">
          Back to home
        </SecondaryButton>
      </div>
    </PageShell>
  );
}
