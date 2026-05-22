import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';

// updatePuzzleStatus tests. Was co-located with fetchNextPuzzle tests
// in legacy `services/puzzleService.test.ts`; split when fetchNextPuzzle
// moved to `features/curation/services/fetch-next-puzzle-service.ts`
// in #176. updatePuzzleStatus stays in `services/` because it's
// genuinely cross-feature (used by `shared/game/hooks/useUpdatePuzzleStatus`,
// which is consumed by both curation and daily flows via `GameBoard`).

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
