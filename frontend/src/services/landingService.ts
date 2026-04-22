import { apiFetch } from './api';
import type { Mode } from './adminService';

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

/** JSON shape of GET /api/config/modes. `modes` is always non-nil. */
interface ConfigModesResponse {
  modes: ModeEntry[];
}

/**
 * Fetch the list of enabled (size, mode) combos. Returns an empty array
 * when no combo is enabled — distinct from the network-error case,
 * which throws.
 */
export async function fetchEnabledModes(): Promise<ModeEntry[]> {
  const resp = await apiFetch<ConfigModesResponse>('/api/config/modes');
  return resp.modes;
}
