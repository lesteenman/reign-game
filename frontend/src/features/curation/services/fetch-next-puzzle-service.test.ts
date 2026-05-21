import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';

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
    difficulty: 2,
    maxTier: 2,
    tierCounts: [0, 3, 1, 0, 0],
    traceLen: 4,
    generationDurationMs: 4200,
    createdAt: '2026-04-15T10:30:00Z',
  },
};

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
      text: () => Promise.resolve(JSON.stringify(MOCK_API_RESPONSE)),
    });

    // Act
    const { fetchNextPuzzle } = await import('./fetch-next-puzzle-service');
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
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./fetch-next-puzzle-service');
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
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./fetch-next-puzzle-service');
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
    const { fetchNextPuzzle, NoPuzzlesAvailableError } = await import('./fetch-next-puzzle-service');
    try {
      await fetchNextPuzzle(7, 'standard');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).not.toBeInstanceOf(NoPuzzlesAvailableError);
      expect(err).toBeInstanceOf(Error);
    }
  });
});
