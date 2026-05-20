// Daily Puzzle service.
//
// X-Device-Id contract: anonymous identity is keyed by a deviceId
// stored in localStorage. The service reads it on every request; mints
// a new one on first invocation if missing. On a 401 (server says
// "missing auth + missing deviceId") the service silently rotates the
// deviceId and retries once — the original was likely rejected as
// malformed; a fresh UUID is the cheap recovery.
//
// Requests go through api.ts (apiFetch / apiPost) with X-Device-Id
// injected via the options.headers surface.

import { apiFetch, apiPost, ApiError } from './api';
import { todayUtcDate, dateFromAssignedAt } from '@shared/dates';
import type { DailyPuzzlePayload, DailySubmitResponse } from '@shared/types/daily';

export type { DailyPuzzlePayload, DailySubmitResponse };

export const DAILY_DEVICE_ID_STORAGE_KEY = 'reign.deviceId';

interface DailySubmitArgs {
  assignedAt: string;
  outcome: 'solved';
  playTimeMs: number;
  solution: number[][];
}

// --- Private helpers ---------------------------------------------------

/** Reads deviceId from localStorage; mints a UUID if missing. */
function getOrMintDeviceId(): string {
  const existing = localStorage.getItem(DAILY_DEVICE_ID_STORAGE_KEY);
  if (existing) return existing;
  const fresh = crypto.randomUUID();
  localStorage.setItem(DAILY_DEVICE_ID_STORAGE_KEY, fresh);
  return fresh;
}

/** Overwrites the stored deviceId with a fresh UUID. */
function mintNewDeviceId(): string {
  const fresh = crypto.randomUUID();
  localStorage.setItem(DAILY_DEVICE_ID_STORAGE_KEY, fresh);
  return fresh;
}

// --- Public API --------------------------------------------------------

/**
 * Fetch the daily puzzle for a UTC date (defaults to today).
 *
 * Sends X-Device-Id (mints one if absent). On a 401 the deviceId is
 * rotated once and the call retried — the second 401 surfaces.
 */
export async function getDaily(date?: string): Promise<DailyPuzzlePayload> {
  const target = date ?? todayUtcDate();
  let deviceId = getOrMintDeviceId();
  try {
    return await apiFetch<DailyPuzzlePayload>(`/api/daily/${target}`, {
      headers: { 'X-Device-Id': deviceId },
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      deviceId = mintNewDeviceId();
      return await apiFetch<DailyPuzzlePayload>(`/api/daily/${target}`, {
        headers: { 'X-Device-Id': deviceId },
      });
    }
    throw err;
  }
}

/**
 * Submit a solved daily result. The path's date is derived from
 * `assignedAt` — never from `now`, so cross-midnight submits land on
 * the correct day's result endpoint.
 */
export async function submitDailyResult(
  args: DailySubmitArgs,
): Promise<DailySubmitResponse> {
  const date = dateFromAssignedAt(args.assignedAt);
  const deviceId = getOrMintDeviceId();
  return await apiPost<DailySubmitResponse>(
    `/api/daily/${date}/result`,
    {
      outcome: args.outcome,
      playTimeMs: args.playTimeMs,
      solution: args.solution,
    },
    {
      headers: { 'X-Device-Id': deviceId },
    },
  );
}
