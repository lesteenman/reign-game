import type { Mode } from '../engine/types';
import { apiPut, ApiError } from './api';

/**
 * Payload for submitVerdict. The persisted DynamoDB row carries every
 * field except `puzzleId` / `size` / `mode` (which determine the URL
 * and query params); see specs/admin-handler.md VH-03 for the wire
 * shape.
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

/**
 * Submit an admin verdict for a puzzle. Calls the
 * `PUT /api/admin/puzzles/{id}/verdict?size=N&mode=M` endpoint
 * with a JSON body shape defined by VH-03.
 *
 * Resolves silently on 401 / 403 (FB-05): the role check that rendered
 * the surface should have prevented this, but if the admin's role was
 * revoked mid-session the buttons may briefly remain visible — a
 * scary error toast for that race is worse UX than letting the click
 * silently no-op.
 *
 * All other non-2xx statuses propagate as `ApiError` so the caller
 * can render a retry-able error state.
 */
export async function submitVerdict(args: SubmitVerdictArgs): Promise<void> {
  const {
    puzzleId,
    size,
    mode,
    value,
    playTimeMs,
    outcome,
    clientVersion,
  } = args;

  const resolvedClientVersion =
    clientVersion ??
    (import.meta.env.VITE_GIT_SHA as string | undefined) ??
    'dev';

  try {
    await apiPut<Record<string, never>>(
      `/api/admin/puzzles/${puzzleId}/verdict`,
      {
        value,
        playTimeMs,
        outcome,
        clientVersion: resolvedClientVersion,
      },
      { params: { size: String(size), mode } },
    );
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      // FB-05: silent swallow. The middleware says no; the UI doesn't
      // need to scream about it.
      return;
    }
    throw err;
  }
}
