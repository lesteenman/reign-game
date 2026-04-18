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

const MOCK_API_RESPONSE = {
  puzzleId: 'pool-001',
  gridSize: 7,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1, 2, 2],
    [0, 0, 1, 1, 2, 2, 2],
    [3, 3, 1, 4, 4, 2, 5],
    [3, 3, 4, 4, 4, 5, 5],
    [3, 6, 6, 4, 5, 5, 5],
    [6, 6, 6, 4, 4, 5, 5],
    [6, 6, 6, 6, 4, 4, 5],
  ],
  metadata: {
    pipeline: 'iterative',
    solver: 'propagation',
    regions: 'bfs',
    regionVariance: 0.0,
    generationDurationMs: 4200,
    createdAt: '2026-04-15T10:30:00Z',
  },
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
      size: '5',
      mode: 'standard',
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

describe('fetchNextPuzzle', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls GET /api/puzzles/next with size and mode params', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_API_RESPONSE),
    });

    // Act
    const { fetchNextPuzzle } = await import('./puzzleService');
    const result = await fetchNextPuzzle(7, 'standard');

    // Assert
    expect(result.puzzleId).toBe('pool-001');
    expect(result.gridSize).toBe(7);
    expect(result.mode).toBe('standard');
    expect(result.metadata).toEqual(MOCK_API_RESPONSE.metadata);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/puzzles/next');
    expect(url).toContain('size=7');
    expect(url).toContain('mode=standard');
  });

  test('throws NoPuzzlesAvailableError on 404', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'no_puzzles_available', message: 'No puzzles available' }),
    });

    // Act & Assert
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./puzzleService');
    try {
      await fetchNextPuzzle(7, 'standard');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(NoPuzzlesAvailableError);
    }
  });

  test('throws generic ApiError on 400', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'invalid size' }),
    });

    // Act & Assert
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./puzzleService');
    try {
      await fetchNextPuzzle(99, 'standard');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).not.toBeInstanceOf(NoPuzzlesAvailableError);
      expect(err).toBeInstanceOf(Error);
    }
  });

  test('throws generic ApiError on 500', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });

    // Act & Assert
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./puzzleService');
    try {
      await fetchNextPuzzle(7, 'standard');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).not.toBeInstanceOf(NoPuzzlesAvailableError);
      expect(err).toBeInstanceOf(Error);
    }
  });
});

describe('updatePuzzleStatus', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls PUT /api/puzzles/{id}/status with correct body and params', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { updatePuzzleStatus } = await import('./puzzleService');
    await updatePuzzleStatus('puzzle-123', 7, 'standard', 'solved');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/puzzles/puzzle-123/status');
    expect(url).toContain('size=7');
    expect(url).toContain('mode=standard');
    const options = calls[0]![1] as RequestInit;
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body as string)).toEqual({ status: 'solved' });
  });

  test('sends skipped status', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { updatePuzzleStatus } = await import('./puzzleService');
    await updatePuzzleStatus('puzzle-456', 9, 'double', 'skipped');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const options = calls[0]![1] as RequestInit;
    expect(JSON.parse(options.body as string)).toEqual({ status: 'skipped' });
  });
});
