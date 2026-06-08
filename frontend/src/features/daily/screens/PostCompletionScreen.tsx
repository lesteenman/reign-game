// TODO: Recycle-day copy — the backend POST response doesn't yet carry an
// isRecycle flag. When backend adds the metadata, surface it as an optional
// prop `isRecycle?: boolean` and render an additional line:
//   "Today's puzzle is a recycle of yesterday's — fresh puzzle tomorrow."

import { useEffect, useState, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { styled, Text, View } from 'tamagui';
import { PageShell } from '@shared/components/PageShell';
import { SecondaryButton } from '@shared/components/Button';

export interface PostCompletionScreenProps {
  serverElapsedMs: number;
  submittedAt: string; // RFC3339
  leaderboardRank?: number; // present only for signed-in users
  /**
   * True when the solve was already recorded server-side (409). Shows
   * an "already submitted" message in place of the (synthetic 0:00)
   * solve time and submission timestamp.
   */
  synthetic409?: boolean;
  /**
   * Optional injection point for "now" so countdown logic is
   * deterministic in tests. Defaults to `new Date()`.
   */
  now?: Date;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const Card = styledAny(View, {
  name: 'PostCompletionCard',
  alignItems: 'center',
  gap: 24,
  backgroundColor: '$surface',
  borderWidth: 2,
  borderColor: '$ink',
  borderRadius: 10,
  shadowColor: '$ink',
  shadowOffset: { width: 0, height: 3 },
  shadowRadius: 0,
  shadowOpacity: 1,
  paddingHorizontal: 24,
  paddingVertical: 32,
  maxWidth: 480,
  width: '100%',
  marginVertical: 24,
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
});

const Heading = styledAny(Text, {
  name: 'PostCompletionHeading',
  fontSize: '$8',
  lineHeight: '$1',
  fontWeight: '800',
  color: '$ink',
  letterSpacing: -0.3,
  textAlign: 'center',
  render: <h1 />,
  margin: 0,
});

const SolveTime = styledAny(Text, {
  name: 'PostCompletionSolveTime',
  fontFamily: '"Space Mono", ui-monospace, monospace',
  fontSize: 36,
  fontWeight: '700',
  color: '$ink',
  textAlign: 'center',
  // `font-variant-numeric: tabular-nums` set at the usage site via the
  // raw `style` prop. Tamagui v2-RC leaks unknown style keys as
  // lowercase DOM attributes (`fontvariantnumeric`); the inline
  // `style={}` channel goes through React's normal camelCase →
  // kebab-case translation instead.
});

const AlreadySubmitted = styledAny(Text, {
  name: 'PostCompletionAlreadySubmitted',
  fontSize: '$5',
  fontWeight: '600',
  color: '$body',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

const RankLine = styledAny(Text, {
  name: 'PostCompletionRank',
  fontSize: '$5',
  fontWeight: '600',
  color: '$body',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

const RankNumber = styledAny(Text, {
  name: 'PostCompletionRankNumber',
  color: '$accent',
  fontWeight: '700',
});

const CountdownBlock = styledAny(View, {
  name: 'PostCompletionCountdown',
  alignItems: 'center',
  gap: 4,
  marginTop: 8,
});

const CountdownLabel = styledAny(Text, {
  name: 'PostCompletionCountdownLabel',
  fontSize: '$3',
  color: '$muted',
  fontWeight: '500',
  textAlign: 'center',
});

const CountdownValue = styledAny(Text, {
  name: 'PostCompletionCountdownValue',
  fontFamily: '"Space Mono", ui-monospace, monospace',
  fontSize: '$6',
  fontWeight: '700',
  color: '$muted',
  textAlign: 'center',
});

const SubmittedAt = styledAny(Text, {
  name: 'PostCompletionSubmittedAt',
  fontSize: '$2',
  color: '$muted',
  fontWeight: '400',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

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
 * Daily post-completion screen.
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
  synthetic409,
  now,
}: PostCompletionScreenProps) {
  const navigate = useNavigate();
  const handleBack = useCallback(() => {
    navigate('/');
  }, [navigate]);

  const initialNow = useMemo(() => now ?? new Date(), [now]);
  const [currentTime, setCurrentTime] = useState<Date>(initialNow);

  useEffect(() => {
    const interval = setInterval(() => {
      // Read wall-clock time directly instead of incrementing the
      // previous value by 1000ms. Browsers throttle setInterval on
      // backgrounded tabs, so a `prev + 1000` loop accumulates drift
      // (the countdown lags behind real time). `new Date()` resyncs
      // every tick so the display catches up immediately when the
      // tab refocuses.
      setCurrentTime(new Date());
    }, 1000);
    return () => clearInterval(interval);
  }, []);

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
      <Card data-testid="daily-post-completion">
        <Heading>Done for today</Heading>

        {synthetic409 ? (
          <AlreadySubmitted data-testid="daily-already-submitted">
            Already submitted earlier today
          </AlreadySubmitted>
        ) : (
          <SolveTime
            data-testid="daily-solve-time"
            aria-label={`Solve time ${formatSolveTime(serverElapsedMs)}`}
            style={{ fontVariantNumeric: 'tabular-nums' }}
          >
            {formatSolveTime(serverElapsedMs)}
          </SolveTime>
        )}

        {typeof leaderboardRank === 'number' && (
          <RankLine data-testid="daily-leaderboard-rank">
            Today&apos;s rank:{' '}
            <RankNumber>#{leaderboardRank}</RankNumber>
          </RankLine>
        )}

        <CountdownBlock>
          <CountdownLabel>Next puzzle in</CountdownLabel>
          <CountdownValue
            data-testid="daily-countdown"
            aria-live="polite"
            style={{ fontVariantNumeric: 'tabular-nums' }}
          >
            {countdownText}
          </CountdownValue>
        </CountdownBlock>

        {!synthetic409 && (
          <SubmittedAt data-testid="daily-submitted-at">
            Submitted at {submittedAtText}
          </SubmittedAt>
        )}

        <SecondaryButton onClick={handleBack} data-testid="daily-back-home">
          Back to home
        </SecondaryButton>
      </Card>
    </PageShell>
  );
}
