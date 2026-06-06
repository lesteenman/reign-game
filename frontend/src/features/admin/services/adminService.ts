import { apiGet, apiPut, apiPost } from '@shared/api';
import type { Mode } from '@engine/types';
import type {
  ConfigView,
  ConfigUpdateRequest,
  ConfigCreateRequest,
  PoolStatus,
  ReplenishResult,
} from '@shared/types/admin';

/** Fetch the current pool status for all combos. */
export async function fetchPoolStatus(
  signal?: AbortSignal,
): Promise<PoolStatus> {
  return apiGet<PoolStatus>('/api/admin/pool', { signal });
}

/** Update the config for an existing combo. */
export async function updateConfig(
  size: number,
  mode: Mode,
  config: ConfigUpdateRequest,
): Promise<ConfigView> {
  return apiPut<ConfigView>(`/api/admin/config/${size}/${mode}`, config);
}

/** Create a new combo config. */
export async function createConfig(
  data: ConfigCreateRequest,
): Promise<ConfigView> {
  return apiPost<ConfigView>('/api/admin/config', data);
}

/** Trigger replenishment for all combos, or a specific one if size/mode provided. */
export async function triggerReplenish(
  size?: number,
  mode?: Mode,
): Promise<ReplenishResult> {
  const params: Record<string, string> = {};
  if (size !== undefined) {
    params.size = String(size);
  }
  if (mode !== undefined) {
    params.mode = mode;
  }
  return apiPost<ReplenishResult>('/api/admin/replenish', {}, { params });
}
