import type { Mode } from '@reign/core/engine';

/**
 * ModeEntry mirrors `handler.ModeEntry` — one {size, mode} pair of an
 * enabled combo. The public GET /api/config/modes endpoint returns a
 * list of these for the landing page to render mode buttons without
 * talking to the admin surface.
 */
export interface ModeEntry {
  size: number;
  mode: Mode;
}
