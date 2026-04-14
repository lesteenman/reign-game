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

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('builds clean URL with only size and mode when no optional params set', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    // Act
    const { generatePuzzle } = await import('./puzzleService');
    const result = await generatePuzzle({ size: 5, mode: 'standard' });

    // Assert
    expect(result).toEqual(MOCK_PUZZLE);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/puzzles/generate');
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
    expect(url).not.toContain('pipeline');
    expect(url).not.toContain('solver');
    expect(url).not.toContain('regions');
    expect(url).not.toContain('regionVariance');
  });

  test('includes all optional params when set', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    // Act
    const { generatePuzzle } = await import('./puzzleService');
    await generatePuzzle({
      size: 9,
      mode: 'double',
      pipeline: 'constraint-aware',
      solver: 'propagation',
      regions: 'wfc',
      regionVariance: 0.75,
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('size=9');
    expect(url).toContain('mode=double');
    expect(url).toContain('pipeline=constraint-aware');
    expect(url).toContain('solver=propagation');
    expect(url).toContain('regions=wfc');
    expect(url).toContain('regionVariance=0.75');
  });

  test('includes only provided optional params', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    // Act
    const { generatePuzzle } = await import('./puzzleService');
    await generatePuzzle({
      size: 7,
      mode: 'standard',
      solver: 'backtrack',
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('size=7');
    expect(url).toContain('mode=standard');
    expect(url).toContain('solver=backtrack');
    expect(url).not.toContain('pipeline');
    expect(url).not.toContain('regions');
    expect(url).not.toContain('regionVariance');
  });

  test('handles regionVariance of 0', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_PUZZLE),
    });

    // Act
    const { generatePuzzle } = await import('./puzzleService');
    await generatePuzzle({
      size: 5,
      mode: 'standard',
      regionVariance: 0,
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('regionVariance=0');
  });
});
