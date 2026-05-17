import type { Mode } from '../../engine/types';

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
