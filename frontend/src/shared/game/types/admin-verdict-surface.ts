import type { ComponentType } from 'react';
import type { Mode } from '@engine/types';

/**
 * Contract for an admin-only verdict surface that GameBoard renders
 * at two sites: inside the completion card (variant="completion") and
 * inside the skip modal (variant="skip"). The contract lives in
 * `shared/game/types/` so GameBoard (shared) can declare the slot
 * without importing from `features/curation/` (which would violate
 * the unidirectional `shared → features → app` rule enforced by
 * `import/no-restricted-paths` in eslint.config.js).
 *
 * features/curation/components/VerdictSurface.tsx implements this
 * contract; consumers (pages/GamePage.tsx for the curation flow) pass
 * it to GameBoard via the `AdminVerdictSurface` prop. Daily flow does
 * not pass it (no verdict step).
 */

interface AdminVerdictSurfaceCommonProps {
  puzzleId: string;
  size: number;
  mode: Mode;
  /** Player's elapsed time on the attempt that produced this verdict, in ms. */
  playTimeMs: number;
}

export interface AdminVerdictCompletionProps extends AdminVerdictSurfaceCommonProps {
  variant: 'completion';
  outcome: 'solved';
  onDismiss?: never;
  onAfterVerdict?: () => void;
}

export interface AdminVerdictSkipProps extends AdminVerdictSurfaceCommonProps {
  variant: 'skip';
  outcome: 'skipped';
  /** Called when the admin clicks Cancel inside the skip modal. */
  onDismiss: () => void;
  /** Called after a successful "I hate this" / "Just skip" submission. */
  onAfterVerdict: () => void;
}

export type AdminVerdictSurfaceProps =
  | AdminVerdictCompletionProps
  | AdminVerdictSkipProps;

export type AdminVerdictSurfaceComponent = ComponentType<AdminVerdictSurfaceProps>;
