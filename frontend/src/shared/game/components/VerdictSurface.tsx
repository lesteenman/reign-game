import { useCallback, useState, type CSSProperties } from 'react';
import type { Mode } from '../../../engine/types';
import { useSubmitVerdict } from '../hooks/useSubmitVerdict';
import { useUpdatePuzzleStatus } from '../hooks/useUpdatePuzzleStatus';
import { PrimaryButton, SecondaryButton, GhostButton } from '../../../shared/components/Button';

/**
 * Verdict surface for admin curation. Renders different button sets
 * by variant (FB-02 / FB-03):
 *
 * - `completion`: "Good puzzle" (up) + "Bad puzzle" (down). Submits
 *   the verdict on click; the parent overlay's existing navigation
 *   (Play Again / Home) stays functional independently.
 * - `skip`: "Cancel" (dismiss; calls onDismiss) + "I hate this"
 *   (status PUT + verdict POST in parallel) + "Just skip" (status PUT
 *   only). Calls onAfterVerdict after a successful "I hate this" or
 *   "Just skip"; the parent navigates forward.
 *
 * After a successful submission the puzzleId is added to
 * `reign:verdict:submitted` in sessionStorage. On subsequent mounts
 * with the same puzzleId, the surface returns null (FB-07) — best-
 * effort UX guard against the buttons re-appearing if the same admin
 * re-completes a re-served puzzle in the same tab.
 */
const SUBMITTED_KEY = 'reign:verdict:submitted';

function isSubmitted(puzzleId: string): boolean {
  try {
    const raw = sessionStorage.getItem(SUBMITTED_KEY);
    if (!raw) return false;
    const ids = JSON.parse(raw) as unknown;
    return Array.isArray(ids) && ids.includes(puzzleId);
  } catch {
    // Defensive: if sessionStorage is unavailable or contains
    // malformed JSON, fall through to "not submitted" rather than
    // silently hiding the surface forever.
    return false;
  }
}

function markSubmitted(puzzleId: string): void {
  try {
    const raw = sessionStorage.getItem(SUBMITTED_KEY);
    const ids = raw ? (JSON.parse(raw) as unknown) : [];
    const list = Array.isArray(ids) ? (ids as string[]) : [];
    if (!list.includes(puzzleId)) list.push(puzzleId);
    sessionStorage.setItem(SUBMITTED_KEY, JSON.stringify(list));
  } catch {
    // Same defensive stance — losing the cache is mildly annoying,
    // not a blocker for a successful action.
  }
}

interface VerdictSurfaceCommonProps {
  puzzleId: string;
  size: number;
  mode: Mode;
  /** Player's elapsed time on the attempt that produced this verdict, in milliseconds. */
  playTimeMs: number;
}

interface CompletionVerdictSurfaceProps extends VerdictSurfaceCommonProps {
  variant: 'completion';
  outcome: 'solved';
  onDismiss?: never;
  onAfterVerdict?: () => void;
}

interface SkipVerdictSurfaceProps extends VerdictSurfaceCommonProps {
  variant: 'skip';
  outcome: 'skipped';
  /** Called when the admin clicks Cancel. Parent should restore the playable view. */
  onDismiss: () => void;
  /**
   * Called after a successful "I hate this" or "Just skip" — the parent navigates
   * forward (typically to /curation or /).
   */
  onAfterVerdict: () => void;
}

export type VerdictSurfaceProps =
  | CompletionVerdictSurfaceProps
  | SkipVerdictSurfaceProps;

type SubmissionStatus =
  | { kind: 'idle' }
  | { kind: 'submitting' }
  | { kind: 'done' }
  | { kind: 'error'; retry: () => Promise<void> };

const containerStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '12px',
};

const completionPromptStyle: CSSProperties = {
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontSize: '1rem',
  fontWeight: 700,
  color: 'var(--color-ink)',
  margin: 0,
};

const skipPromptStyle: CSSProperties = {
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontSize: '0.875rem',
  fontWeight: 600,
  color: 'var(--color-muted)',
  margin: 0,
};

const buttonRowStyle: CSSProperties = {
  display: 'flex',
  gap: '12px',
  flexWrap: 'wrap',
  justifyContent: 'center',
};

const skipButtonRowStyle: CSSProperties = {
  ...buttonRowStyle,
  gap: '8px',
};

export function VerdictSurface(props: VerdictSurfaceProps) {
  const { puzzleId, size, mode, playTimeMs, variant } = props;
  // Cache check is captured ONCE at mount via the lazy initial state.
  // After a successful submission we mark the puzzleId in
  // sessionStorage AND set status='done', but `wasSubmittedAtMount`
  // stays false so the "Thanks — recorded." beat still renders for
  // the active session — the cache only hides the surface on the
  // NEXT mount with the same puzzleId.
  const [wasSubmittedAtMount] = useState(() => isSubmitted(puzzleId));
  const [status, setStatus] = useState<SubmissionStatus>({ kind: 'idle' });

  const submitVerdictMutation = useSubmitVerdict();
  const updatePuzzleStatusMutation = useUpdatePuzzleStatus();

  // Status-machine wrapper. `work` is the variant-specific API call;
  // success → markSubmitted + optional onSuccess hook + done. Failure
  // re-stores `action` as the retry so the error-state Retry button
  // re-runs the same flow with the same payload.
  const runSubmission = useCallback(
    (work: () => Promise<void>, onSuccess?: () => void) => {
      const action = async () => {
        setStatus({ kind: 'submitting' });
        try {
          await work();
          markSubmitted(puzzleId);
          onSuccess?.();
          setStatus({ kind: 'done' });
        } catch {
          setStatus({ kind: 'error', retry: action });
        }
      };
      return action();
    },
    [puzzleId],
  );

  const submitCompletionVerdict = useCallback(
    (value: 'up' | 'down') =>
      runSubmission(() =>
        submitVerdictMutation.mutateAsync({
          puzzleId,
          size,
          mode,
          value,
          playTimeMs,
          outcome: 'solved',
        }),
      ),
    [runSubmission, submitVerdictMutation, puzzleId, size, mode, playTimeMs],
  );

  // Both skip handlers narrow the union via `props.variant === 'skip'`
  // before reading `props.onAfterVerdict` — the destructured `variant`
  // local at the top of the component does not propagate type-narrowing
  // to `props` itself, so the in-callback discriminant check is what
  // gives TS confidence that `onAfterVerdict` is required (not optional)
  // on the SkipVerdictSurfaceProps branch. The check also protects
  // against accidental cross-variant invocation; the runtime guard is
  // a no-op on the wrong variant rather than a crash.
  const submitIHateThis = useCallback(() => {
    if (props.variant !== 'skip') return Promise.resolve();
    const onAfterVerdict = props.onAfterVerdict;
    return runSubmission(
      // Parallel — verdict + status are orthogonal flows; running in
      // parallel saves a round-trip and the two writes touch
      // independent DynamoDB row families.
      async () => {
        await Promise.all([
          updatePuzzleStatusMutation.mutateAsync({ puzzleId, size, mode, status: 'skipped' }),
          submitVerdictMutation.mutateAsync({
            puzzleId,
            size,
            mode,
            value: 'down',
            playTimeMs,
            outcome: 'skipped',
          }),
        ]);
      },
      onAfterVerdict,
    );
  }, [runSubmission, updatePuzzleStatusMutation, submitVerdictMutation, props, puzzleId, size, mode, playTimeMs]);

  const submitJustSkip = useCallback(() => {
    if (props.variant !== 'skip') return Promise.resolve();
    const onAfterVerdict = props.onAfterVerdict;
    return runSubmission(
      () => updatePuzzleStatusMutation.mutateAsync({ puzzleId, size, mode, status: 'skipped' }),
      onAfterVerdict,
    );
  }, [runSubmission, updatePuzzleStatusMutation, props, puzzleId, size, mode]);

  // FB-07 cache check, captured once at mount. Hides the surface on
  // re-mount within the same tab session (e.g. an admin re-completes
  // a re-served puzzle). Active submissions still flow through the
  // state machine below.
  if (wasSubmittedAtMount) {
    return null;
  }

  if (status.kind === 'error') {
    return (
      <div style={containerStyle} data-testid="verdict-surface-error">
        <p
          style={{
            color: 'var(--color-destructive, #c33)',
            margin: 0,
            fontWeight: 600,
            fontSize: '0.875rem',
          }}
        >
          Couldn&apos;t save your verdict.
        </p>
        <SecondaryButton onClick={() => void status.retry()}>Retry</SecondaryButton>
      </div>
    );
  }

  if (status.kind === 'done') {
    // Completion variant shows a brief "Thanks — recorded." beat;
    // skip variant returns null (the parent navigates forward via
    // onAfterVerdict, so the navigation IS the feedback).
    if (variant === 'completion') {
      return (
        <p
          style={{
            ...completionPromptStyle,
            color: 'var(--color-success, var(--color-ink))',
          }}
          data-testid="verdict-surface-thanks"
        >
          Thanks — recorded.
        </p>
      );
    }
    return null;
  }

  const submitting = status.kind === 'submitting';

  if (variant === 'completion') {
    return (
      <div style={containerStyle} data-testid="verdict-surface-completion">
        <p style={completionPromptStyle}>How was that puzzle?</p>
        <div style={buttonRowStyle}>
          <PrimaryButton
            onClick={() => void submitCompletionVerdict('up')}
            disabled={submitting}
          >
            Good puzzle
          </PrimaryButton>
          <SecondaryButton
            onClick={() => void submitCompletionVerdict('down')}
            disabled={submitting}
          >
            Bad puzzle
          </SecondaryButton>
        </div>
      </div>
    );
  }

  // variant === 'skip'
  return (
    <div style={containerStyle} data-testid="verdict-surface-skip">
      <p style={skipPromptStyle}>Skip this puzzle?</p>
      <div style={skipButtonRowStyle}>
        <GhostButton
          onClick={() => props.onDismiss()}
          disabled={submitting}
        >
          Cancel
        </GhostButton>
        <SecondaryButton
          onClick={() => void submitIHateThis()}
          disabled={submitting}
        >
          I hate this
        </SecondaryButton>
        <SecondaryButton
          onClick={() => void submitJustSkip()}
          disabled={submitting}
        >
          Just skip
        </SecondaryButton>
      </div>
    </div>
  );
}
