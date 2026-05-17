import type { Mode } from '../../engine/types';

/**
 * SubmitVerdictArgs is the payload for the verdict submission API call.
 * Defined here (not in services/) so hooks and tests import types from
 * the source rather than through the service module.
 */
export interface SubmitVerdictArgs {
  puzzleId: string;
  size: number;
  mode: Mode;
  value: 'up' | 'down';
  playTimeMs: number;
  outcome: 'solved' | 'skipped';
  /**
   * Frontend git SHA for forensic correlation between a verdict row
   * and the build it came from. Defaults to the `VITE_GIT_SHA` env
   * var, then to `'dev'` when neither is set (local dev).
   */
  clientVersion?: string;
}
