import { apiFetch, apiPut, apiPost } from './api';

/** Configuration data for a puzzle pool combo. */
export interface ConfigData {
  pipeline: string;
  solver: string;
  regions: string;
  regionVariance: number;
  deducible: boolean;
  concurrency: number;
  threshold: number;
  enabled: boolean;
}

/** Status of a single size/mode combo in the pool. */
export interface ComboStatus {
  size: number;
  mode: string;
  config: ConfigData;
  readyCount: number;
}

/** Overall pool status containing all combos. */
export interface PoolStatus {
  combos: ComboStatus[];
}

/** Entry describing a replenish action for a single combo. */
export interface ReplenishEntry {
  size: number;
  mode: string;
  count: number;
}

/** Result of a replenish operation. */
export interface ReplenishResult {
  triggered: ReplenishEntry[];
  skipped: Array<{ size: number; mode: string; ready: number }>;
}

/** Request payload for creating a new combo config. */
export interface CreateConfigRequest extends ConfigData {
  size: number;
  mode: string;
}

/** Fetch the current pool status for all combos. */
export async function fetchPoolStatus(): Promise<PoolStatus> {
  return apiFetch<PoolStatus>('/api/admin/pool');
}

/** Update the config for an existing combo. */
export async function updateConfig(
  size: number,
  mode: string,
  config: ConfigData,
): Promise<ConfigData> {
  return apiPut<ConfigData>(`/api/admin/config/${size}/${mode}`, config);
}

/** Create a new combo config. */
export async function createConfig(
  data: CreateConfigRequest,
): Promise<ConfigData> {
  return apiPost<ConfigData>('/api/admin/config', data);
}

/** Trigger replenishment for all combos, or a specific one if size/mode provided. */
export async function triggerReplenish(
  size?: number,
  mode?: string,
): Promise<ReplenishResult> {
  const params: Record<string, string> = {};
  if (size !== undefined) {
    params.size = String(size);
  }
  if (mode !== undefined) {
    params.mode = mode;
  }
  return apiPost<ReplenishResult>('/api/admin/replenish', {}, params);
}
