import { apiGet } from '@shared/api';
import type { ModeEntry } from '@shared/types/modes';

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
  const resp = await apiGet<ConfigModesResponse>('/api/config/modes');
  return resp.modes;
}
