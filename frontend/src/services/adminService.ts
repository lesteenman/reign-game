import { apiFetch, apiPut, apiPost } from './api';
import type { Mode } from '../engine/types';

// Mode / MODES / isMode are the domain primitives. They live in
// `engine/types.ts` so both the admin surface and the game-play surface
// (puzzleService, GamePage) can share them. Re-exported here for
// callers who already reach into adminService for config types.
export { MODES, isMode } from '../engine/types';
export type { Mode };

/**
 * ConfigBody is the payload subset of a CONFIG row — the fields shared
 * by every request/response shape that carries config values. Mirrors
 * `handler.ConfigBody` on the backend.
 */
export interface ConfigBody {
  threshold: number;
  enabled: boolean;
  /** Optional override for the generator's WithMaxAttempts. 0 means default. */
  maxAttempts?: number;
}

/**
 * ConfigView is the flat response shape from PUT /api/admin/config/{size}/{mode}
 * and POST /api/admin/config. Identity fields (size, mode) live alongside
 * the config body.
 */
export interface ConfigView extends ConfigBody {
  size: number;
  mode: Mode;
}

/** Request body for PUT /api/admin/config/{size}/{mode}. */
export type ConfigUpdateRequest = ConfigBody;

/** Request body for POST /api/admin/config. */
export interface ConfigCreateRequest extends ConfigBody {
  size: number;
  mode: Mode;
}

/** Status of a single size/mode combo in the pool. */
export interface ComboStatus {
  size: number;
  mode: Mode;
  config: ConfigBody;
  readyCount: number;
}

/** Overall pool status containing all combos. */
export interface PoolStatus {
  combos: ComboStatus[];
}

/** Entry describing a replenish action for a single combo. */
export interface ReplenishEntry {
  size: number;
  mode: Mode;
  count: number;
}

/** Result of a replenish operation. */
export interface ReplenishResult {
  triggered: ReplenishEntry[];
  skipped: Array<{ size: number; mode: Mode; ready: number }>;
}

/** Fetch the current pool status for all combos. */
export async function fetchPoolStatus(): Promise<PoolStatus> {
  return apiFetch<PoolStatus>('/api/admin/pool');
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
  return apiPost<ReplenishResult>('/api/admin/replenish', {}, params);
}
