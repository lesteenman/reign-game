import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';

describe('submitVerdict', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('PUTs to /api/admin/puzzles/{id}/verdict with size+mode query params and JSON body (FB-04)', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });
    const { submitVerdict } = await import('./verdictService');

    // Act
    await submitVerdict({
      puzzleId: 'puz-123',
      size: 7,
      mode: 'standard',
      value: 'up',
      playTimeMs: 42000,
      outcome: 'solved',
      clientVersion: '0.1.0',
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls).toHaveLength(1);
    const [url, init] = calls[0]!;
    expect(url).toContain('/api/admin/puzzles/puz-123/verdict');
    expect(url).toContain('size=7');
    expect(url).toContain('mode=standard');
    expect(init.method).toBe('PUT');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(JSON.parse(init.body as string)).toEqual({
      value: 'up',
      playTimeMs: 42000,
      outcome: 'solved',
      clientVersion: '0.1.0',
    });
  });

  test('uses `dev` as the default clientVersion when caller does not pass one', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });
    const { submitVerdict } = await import('./verdictService');

    // Act
    await submitVerdict({
      puzzleId: 'puz-1',
      size: 5,
      mode: 'standard',
      value: 'down',
      playTimeMs: 0,
      outcome: 'skipped',
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1];
    expect(JSON.parse(init.body as string).clientVersion).toBe('dev');
  });

  test('resolves silently on 401 — admin role may have been revoked mid-session (FB-05)', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ message: 'unauthorized' }),
    });
    const { submitVerdict } = await import('./verdictService');

    // Act + Assert
    await expect(
      submitVerdict({
        puzzleId: 'p',
        size: 5,
        mode: 'standard',
        value: 'up',
        playTimeMs: 1,
        outcome: 'solved',
      }),
    ).resolves.toBeUndefined();
  });

  test('resolves silently on 403 — same reason as 401 (FB-05)', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ message: 'forbidden' }),
    });
    const { submitVerdict } = await import('./verdictService');

    // Act + Assert
    await expect(
      submitVerdict({
        puzzleId: 'p',
        size: 5,
        mode: 'standard',
        value: 'up',
        playTimeMs: 1,
        outcome: 'solved',
      }),
    ).resolves.toBeUndefined();
  });

  test('throws ApiError on 404 — puzzle not found at the verdict endpoint', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ message: 'puzzle not found' }),
    });
    const { submitVerdict } = await import('./verdictService');
    const { ApiError } = await import('@shared/api');

    // Act + Assert
    try {
      await submitVerdict({
        puzzleId: 'missing',
        size: 5,
        mode: 'standard',
        value: 'up',
        playTimeMs: 1,
        outcome: 'solved',
      });
      throw new Error('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(404);
    }
  });

  test('throws ApiError on 500 — transient backend failure surfaces to caller for retry', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ message: 'internal error' }),
    });
    const { submitVerdict } = await import('./verdictService');
    const { ApiError } = await import('@shared/api');

    // Act + Assert
    try {
      await submitVerdict({
        puzzleId: 'p',
        size: 5,
        mode: 'standard',
        value: 'up',
        playTimeMs: 1,
        outcome: 'solved',
      });
      throw new Error('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(500);
    }
  });

  test('throws ApiError on 400 — body validation failure surfaces to caller', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'invalid_params: value' }),
    });
    const { submitVerdict } = await import('./verdictService');
    const { ApiError } = await import('@shared/api');

    // Act + Assert
    try {
      await submitVerdict({
        puzzleId: 'p',
        size: 5,
        mode: 'standard',
        // @ts-expect-error — testing that backend 400 propagates
        value: 'invalid',
        playTimeMs: 1,
        outcome: 'solved',
      });
      throw new Error('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(400);
    }
  });
});
