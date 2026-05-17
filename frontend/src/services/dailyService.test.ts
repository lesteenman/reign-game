import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  DAILY_DEVICE_ID_STORAGE_KEY,
  getDaily,
  submitDailyResult,
} from './dailyService';

// Helpers ---------------------------------------------------------------

interface RecordedCall {
  url: string;
  method: string;
  headers: Record<string, string>;
  body?: string;
}

function makeJsonResponse(status: number, body: unknown): Response {
  const text = JSON.stringify(body);
  return new Response(text, {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function recordFetch(
  responses: Array<Response | (() => Response)>,
): { calls: RecordedCall[]; mock: ReturnType<typeof vi.fn> } {
  const calls: RecordedCall[] = [];
  let i = 0;
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString();
    const headerObj: Record<string, string> = {};
    const rawHeaders = (init?.headers ?? {}) as
      | Record<string, string>
      | Headers
      | [string, string][];
    if (rawHeaders instanceof Headers) {
      rawHeaders.forEach((v, k) => {
        headerObj[k] = v;
      });
    } else if (Array.isArray(rawHeaders)) {
      for (const [k, v] of rawHeaders) headerObj[k] = v;
    } else {
      Object.assign(headerObj, rawHeaders);
    }
    calls.push({
      url,
      method: (init?.method ?? 'GET').toUpperCase(),
      headers: headerObj,
      body: init?.body == null ? undefined : String(init.body),
    });
    const next = responses[i++];
    if (!next) throw new Error(`No mock response for call #${i}`);
    return typeof next === 'function' ? next() : next;
  });
  globalThis.fetch = mock as unknown as typeof fetch;
  return { calls, mock };
}

// Tests -----------------------------------------------------------------

describe('dailyService', () => {
  beforeEach(() => {
    localStorage.clear();
    // crypto.randomUUID is available in jsdom (Node 19+) — stub for determinism.
    let n = 0;
    vi.spyOn(crypto, 'randomUUID').mockImplementation(
      () =>
        `00000000-0000-4000-8000-${String(++n).padStart(12, '0')}` as `${string}-${string}-${string}-${string}-${string}`,
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe('getDaily', () => {
    it('defaults date to todayUTC (YYYY-MM-DD)', async () => {
      // Arrange
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-05-02T03:14:00Z'));
      const { calls } = recordFetch([
        makeJsonResponse(200, {
          puzzleId: 'p1',
          grid: 8,
          regions: [[0]],
          assignedAt: '2026-05-02T00:00:00Z',
          outcome: 'started',
        }),
      ]);

      // Act
      await getDaily();

      // Assert
      expect(calls).toHaveLength(1);
      expect(calls[0]?.url).toContain('/api/daily/2026-05-02');
    });

    it('honors explicit date param', async () => {
      // Arrange
      const { calls } = recordFetch([
        makeJsonResponse(200, {
          puzzleId: 'p1',
          grid: 8,
          regions: [[0]],
          assignedAt: '2026-04-15T00:00:00Z',
          outcome: 'started',
        }),
      ]);

      // Act
      await getDaily('2026-04-15');

      // Assert
      expect(calls[0]?.url).toContain('/api/daily/2026-04-15');
    });

    it('sends X-Device-Id header from localStorage', async () => {
      // Arrange
      localStorage.setItem(DAILY_DEVICE_ID_STORAGE_KEY, 'pre-existing-id');
      const { calls } = recordFetch([
        makeJsonResponse(200, {
          puzzleId: 'p1',
          grid: 8,
          regions: [[0]],
          assignedAt: '2026-05-02T00:00:00Z',
          outcome: 'started',
        }),
      ]);

      // Act
      await getDaily('2026-05-02');

      // Assert
      expect(calls[0]?.headers['X-Device-Id']).toBe('pre-existing-id');
    });

    it('mints a deviceId on first call when localStorage is empty', async () => {
      // Arrange
      const { calls } = recordFetch([
        makeJsonResponse(200, {
          puzzleId: 'p1',
          grid: 8,
          regions: [[0]],
          assignedAt: '2026-05-02T00:00:00Z',
          outcome: 'started',
        }),
      ]);

      // Act
      await getDaily('2026-05-02');

      // Assert
      const stored = localStorage.getItem(DAILY_DEVICE_ID_STORAGE_KEY);
      expect(stored).toBeTruthy();
      expect(stored).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
      );
      expect(calls[0]?.headers['X-Device-Id']).toBe(stored);
    });

    it('rotates deviceId on 401 and retries once', async () => {
      // Arrange
      localStorage.setItem(DAILY_DEVICE_ID_STORAGE_KEY, 'stale-id');
      const { calls } = recordFetch([
        makeJsonResponse(401, { message: 'unauthorized' }),
        makeJsonResponse(200, {
          puzzleId: 'p1',
          grid: 8,
          regions: [[0]],
          assignedAt: '2026-05-02T00:00:00Z',
          outcome: 'started',
        }),
      ]);

      // Act
      const result = await getDaily('2026-05-02');

      // Assert
      expect(calls).toHaveLength(2);
      expect(calls[0]?.headers['X-Device-Id']).toBe('stale-id');
      expect(calls[1]?.headers['X-Device-Id']).not.toBe('stale-id');
      expect(calls[1]?.headers['X-Device-Id']).toBe(
        localStorage.getItem(DAILY_DEVICE_ID_STORAGE_KEY),
      );
      expect(result.puzzleId).toBe('p1');
    });

    it('throws on second 401 with .status === 401', async () => {
      // Arrange
      recordFetch([
        makeJsonResponse(401, { message: 'unauthorized' }),
        makeJsonResponse(401, { message: 'still unauthorized' }),
      ]);

      // Act + Assert
      await expect(getDaily('2026-05-02')).rejects.toMatchObject({
        status: 401,
      });
    });
  });

  describe('submitDailyResult', () => {
    it('posts to date derived from assignedAt (not now)', async () => {
      // Arrange
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-05-02T00:00:02Z'));
      localStorage.setItem(DAILY_DEVICE_ID_STORAGE_KEY, 'd1');
      const { calls } = recordFetch([
        makeJsonResponse(200, { serverElapsedMs: 12345 }),
      ]);

      // Act
      await submitDailyResult({
        assignedAt: '2026-05-01T23:55:00Z',
        outcome: 'solved',
        playTimeMs: 12345,
        solution: [[0, 1]],
      });

      // Assert
      expect(calls[0]?.url).toContain('/api/daily/2026-05-01/result');
      expect(calls[0]?.method).toBe('POST');
    });

    it('body shape contains outcome, playTimeMs, solution', async () => {
      // Arrange
      const { calls } = recordFetch([
        makeJsonResponse(200, { serverElapsedMs: 100 }),
      ]);

      // Act
      await submitDailyResult({
        assignedAt: '2026-05-02T00:00:00Z',
        outcome: 'solved',
        playTimeMs: 100,
        solution: [[1, 2, 3]],
      });

      // Assert
      const body = JSON.parse(calls[0]?.body ?? '{}');
      expect(body).toEqual({
        outcome: 'solved',
        playTimeMs: 100,
        solution: [[1, 2, 3]],
      });
    });

    it('returns serverElapsedMs and leaderboardRank', async () => {
      // Arrange
      recordFetch([
        makeJsonResponse(200, { serverElapsedMs: 9876, leaderboardRank: 42 }),
      ]);

      // Act
      const resp = await submitDailyResult({
        assignedAt: '2026-05-02T00:00:00Z',
        outcome: 'solved',
        playTimeMs: 9876,
        solution: [[0]],
      });

      // Assert
      expect(resp).toEqual({ serverElapsedMs: 9876, leaderboardRank: 42 });
    });

    it('propagates 409 with .status set', async () => {
      // Arrange
      recordFetch([makeJsonResponse(409, { message: 'already solved' })]);

      // Act + Assert
      await expect(
        submitDailyResult({
          assignedAt: '2026-05-02T00:00:00Z',
          outcome: 'solved',
          playTimeMs: 100,
          solution: [[0]],
        }),
      ).rejects.toMatchObject({ status: 409 });
    });
  });
});
