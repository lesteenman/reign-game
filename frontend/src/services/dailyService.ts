// Daily Puzzle service — implements DP-28, DP-29, DP-30.
//
// X-Device-Id contract (DP-10): anonymous identity is keyed by a
// deviceId stored in localStorage. The service reads it on every
// request; mints a new one on first invocation if missing. On a 401
// (server says "missing auth + missing deviceId") the service silently
// rotates the deviceId and retries once — the original was likely
// rejected as malformed; a fresh UUID is the cheap recovery (DP-30).
//
// Why direct fetch instead of api.ts: api.ts has no header-injection
// surface today, and this slice intentionally avoids touching the
// shared client (out of scope for R-8-02). Mirrors api.ts's URL
// construction (VITE_API_URL fallback to same-origin) and ApiError
// shape so callers can use the same `.status` branching.

const API_BASE_URL = import.meta.env.VITE_API_URL || '';

export const DAILY_DEVICE_ID_STORAGE_KEY = 'reign.deviceId';

/** Error thrown on non-2xx daily API responses; carries HTTP status. */
export class DailyApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = 'DailyApiError';
  }
}

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

function buildUrl(path: string): string {
  return new URL(path, API_BASE_URL || window.location.origin).toString();
}

async function parseError(response: Response): Promise<DailyApiError> {
  const body = (await response.json().catch(() => ({}))) as Record<
    string,
    string
  >;
  return new DailyApiError(
    body.message || `API error: ${response.status}`,
    response.status,
  );
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
  const url = buildUrl(`/api/daily/${target}`);

  let deviceId = getOrMintDeviceId();
  let response = await fetch(url, {
    method: 'GET',
    headers: { 'X-Device-Id': deviceId },
  });

  if (response.status === 401) {
    deviceId = mintNewDeviceId();
    response = await fetch(url, {
      method: 'GET',
      headers: { 'X-Device-Id': deviceId },
    });
  }

  if (!response.ok) throw await parseError(response);
  return (await response.json()) as DailyPuzzlePayload;
}

/**
 * Submit a solved daily result. The path's date is derived from
 * `assignedAt` (DP-29 cross-midnight contract — never from `now`).
 */
export async function submitDailyResult(
  args: DailySubmitArgs,
): Promise<DailySubmitResponse> {
  const date = dateFromAssignedAt(args.assignedAt);
  const url = buildUrl(`/api/daily/${date}/result`);
  const deviceId = getOrMintDeviceId();

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Device-Id': deviceId,
    },
    body: JSON.stringify({
      outcome: args.outcome,
      playTimeMs: args.playTimeMs,
      solution: args.solution,
    }),
  });

  if (!response.ok) throw await parseError(response);
  return (await response.json()) as DailySubmitResponse;
}
