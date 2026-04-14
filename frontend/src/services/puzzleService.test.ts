import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import type { PuzzleData } from '../engine/types';

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
    const result = await apiFetch<PuzzleData>('/puzzles/generate', {
      size: '5',
      mode: 'standard',
    });

    expect(result).toEqual(MOCK_PUZZLE);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/puzzles/generate');
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
  });

  test('throws on non-ok response with server message', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'invalid size' }),
    });

    const { apiFetch } = await import('./api');
    await expect(apiFetch('/puzzles/generate')).rejects.toThrow('invalid size');
  });

  test('throws with status code when body has no message', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });

    const { apiFetch } = await import('./api');
    await expect(apiFetch('/puzzles/generate')).rejects.toThrow(
      'API error: 500',
    );
  });

  test('throws on network failure', async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

    const { apiFetch } = await import('./api');
    await expect(apiFetch('/puzzles/generate')).rejects.toThrow(
      'Failed to fetch',
    );
  });
});

describe('generatePuzzle', () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls apiFetch with correct path and params', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    const { generatePuzzle } = await import('./puzzleService');
    const result = await generatePuzzle(5, 'standard');

    expect(result).toEqual(MOCK_PUZZLE);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/puzzles/generate');
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
  });
});
