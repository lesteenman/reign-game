// Daily Puzzle service — implements DP-28, DP-29, DP-30.
//
// X-Device-Id contract (DP-10): anonymous identity is keyed by a
// deviceId stored in localStorage. The service reads it on every
// request; mints a new one on first invocation if missing. On a 401
// (server says "missing auth + missing deviceId") the service silently
// rotates the deviceId and retries once — the original was likely
// rejected as malformed; a fresh UUID is the cheap recovery (DP-30).
//
// Requests go through api.ts (apiFetch / apiPost) with X-Device-Id
// injected via the options.headers surface.

import { apiFetch, apiPost, ApiError } from './api';

export const DAILY_DEVICE_ID_STORAGE_KEY = 'reign.deviceId';

export interface DailyPuzzlePayload {
  puzzleId: string;
  grid: number;
  regions: number[][];
  assignedAt: string;
  outcome: 'started' | 'solved';
  serverElapsedMs?: number;
  submittedAt?: string;
}

export interface DailySubmitResponse {
  serverElapsedMs: number;
  leaderboardRank?: number;
}

interface DailySubmitArgs {
  assignedAt: string;
  outcome: 'solved';
  playTimeMs: number;
  solution: number[][];
}

// --- Private helpers ---------------------------------------------------

/** Returns YYYY-MM-DD for the current UTC date. */
function todayUTC(now: Date = new Date()): string {
  return now.toISOString().slice(0, 10);
}

/** Extracts YYYY-MM-DD from an RFC3339 timestamp's UTC date component. */
function dateFromAssignedAt(assignedAt: string): string {
  // RFC3339 / ISO8601 — Date parses Z and offset suffixes consistently.
  return new Date(assignedAt).toISOString().slice(0, 10);
}

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
  const target = date ?? todayUTC();
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
 * `assignedAt` (DP-29 cross-midnight contract — never from `now`).
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
