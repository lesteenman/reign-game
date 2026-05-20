import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import type { PuzzleData } from '@engine/types';

// apiFetch is the shared fetch base used by every service in the
// app — these tests cover the helper itself (URL composition,
// ApiError shape on 4xx/5xx, network-failure propagation). They were
// previously co-located with puzzleService's tests; split out in #176
// when fetchNextPuzzle moved into `features/curation/services/`.

const MOCK_PUZZLE: PuzzleData = {
  puzzleId: 'test-001',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

describe('apiFetch', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('builds URL with query params and returns JSON', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    const { apiFetch } = await import('./api');
    const result = await apiFetch<PuzzleData>('/api/puzzles/generate', {
      params: { size: '5', mode: 'standard' },
    });

    expect(result).toEqual(MOCK_PUZZLE);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/puzzles/generate');
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
  });

  test('throws ApiError on non-ok response with server message', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'invalid size' }),
    });

    const { apiFetch, ApiError } = await import('./api');
    try {
      await apiFetch('/api/puzzles/generate');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).message).toBe('invalid size');
      expect((err as InstanceType<typeof ApiError>).status).toBe(400);
    }
  });

  test('throws ApiError with status code when body has no message', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });

    const { apiFetch, ApiError } = await import('./api');
    try {
      await apiFetch('/api/puzzles/generate');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(500);
    }
  });

  test('throws on network failure', async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

    const { apiFetch } = await import('./api');
    await expect(apiFetch('/api/puzzles/generate')).rejects.toThrow(
      'Failed to fetch',
    );
  });
});
