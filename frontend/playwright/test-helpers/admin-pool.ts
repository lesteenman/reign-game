import { expect, type APIRequestContext } from "@playwright/test";

/**
 * Shared types + reader for GET /api/admin/pool. Backend source of truth
 * is `backend/internal/handler/config_dto.go::ConfigBody` and the admin
 * pool handler's response shape. `maxAttempts` is `json:"...,omitempty"`
 * on the backend so it is optional here.
 */

export interface ConfigBody {
  threshold: number;
  enabled: boolean;
  maxAttempts?: number;
}

export interface ComboStatus {
  size: number;
  mode: string;
  config: ConfigBody;
  readyCount: number;
}

export interface AdminPoolResponse {
  combos: ComboStatus[];
}

/**
 * Read the `size#mode` combo from GET /api/admin/pool. Throws loudly if
 * the combo is missing so a fixture/seed drift fails the test instead
 * of silently treating the absence as `readyCount = 0`.
 */
export async function readCombo(
  request: APIRequestContext,
  size: number,
  mode: string,
): Promise<ComboStatus> {
  const res = await request.get("/api/admin/pool");
  expect(res.ok(), `GET /api/admin/pool failed: ${res.status()}`).toBe(true);
  const body = (await res.json()) as AdminPoolResponse;
  const combo = body.combos.find((c) => c.size === size && c.mode === mode);
  if (!combo) {
    throw new Error(
      `combo ${size}#${mode} missing from /api/admin/pool response — ` +
        `seed/CONFIG fixture likely not loaded`,
    );
  }
  return combo;
}
