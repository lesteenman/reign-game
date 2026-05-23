import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import type { PuzzleData } from '@engine/types';

/**
 * Tests for the consolidated `apiRequest` client and its `apiGet` /
 * `apiPut` / `apiPost` wrappers. Includes empty-body / 204-No-Content
 * coverage across all three methods to pin the text-then-parse path
 * that makes empty responses resolve to `{}` (not throw).
 */

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

const originalFetch = globalThis.fetch;

beforeEach(() => {
  vi.resetModules();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe('apiGet', () => {
  test('builds URL with query params and returns parsed JSON', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_PUZZLE)),
    });

    // Act
    const { apiGet } = await import('./client');
    const result = await apiGet<PuzzleData>('/api/puzzles/generate', {
      params: { size: '5', mode: 'standard' },
    });

    // Assert
    expect(result).toEqual(MOCK_PUZZLE);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/puzzles/generate');
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
    // GET sends no body and no Content-Type header (no body to declare).
    const init = calls[0]![1] as RequestInit;
    expect(init.method).toBe('GET');
    expect(init.body).toBeUndefined();
  });

  test('forwards caller-provided headers', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    // Act
    const { apiGet } = await import('./client');
    await apiGet('/api/x', { headers: { 'X-Test': 'value' } });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect((init.headers as Record<string, string>)['X-Test']).toBe('value');
  });
});

describe('apiPut + apiPost', () => {
  test('apiPut sends method PUT with JSON body + Content-Type', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{"ok":true}'),
    });

    // Act
    const { apiPut } = await import('./client');
    await apiPut('/api/x', { foo: 'bar' });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect(init.method).toBe('PUT');
    expect(init.body).toBe(JSON.stringify({ foo: 'bar' }));
    expect((init.headers as Record<string, string>)['Content-Type']).toBe(
      'application/json',
    );
  });

  test('apiPost sends method POST with JSON body + Content-Type', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    // Act
    const { apiPost } = await import('./client');
    await apiPost('/api/x', { a: 1 });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect(init.method).toBe('POST');
    expect(init.body).toBe(JSON.stringify({ a: 1 }));
    expect((init.headers as Record<string, string>)['Content-Type']).toBe(
      'application/json',
    );
  });
});

describe('empty-body handling — unified across all methods', () => {
  // Pre-#120 `apiFetch` called `response.json()` directly and would
  // throw `SyntaxError: Unexpected end of JSON input` here; only
  // `apiPut` / `apiPost` did the text-then-parse dance. The
  // consolidated `apiRequest` applies the safe text-then-parse path
  // to every method, so all three resolve uniformly to `{}`.

  test('apiGet returns {} when the response body is empty (e.g. 204)', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { apiGet } = await import('./client');
    const result = await apiGet<Record<string, never>>('/api/x');

    // Assert
    expect(result).toEqual({});
  });

  test('apiPut returns {} when the response body is empty', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { apiPut } = await import('./client');
    const result = await apiPut<Record<string, never>>('/api/x', {});

    // Assert
    expect(result).toEqual({});
  });

  test('apiPost returns {} when the response body is empty', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { apiPost } = await import('./client');
    const result = await apiPost<Record<string, never>>('/api/x', {});

    // Assert
    expect(result).toEqual({});
  });
});

describe('x-api-key header (browser-distributed AWS API Gateway key)', () => {
  // The client reads VITE_API_KEY at module load. vi.stubEnv +
  // vi.resetModules (in the global beforeEach) lets each case lock
  // in a different value before the dynamic import.

  test('attaches x-api-key when VITE_API_KEY is set', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', 'sample-api-key-from-aws');
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    // Act
    const { apiGet } = await import('./client');
    await apiGet('/api/x');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect((init.headers as Record<string, string>)['x-api-key']).toBe(
      'sample-api-key-from-aws',
    );

    vi.unstubAllEnvs();
  });

  test('omits x-api-key when VITE_API_KEY is empty (dev / test path)', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    // Act
    const { apiGet } = await import('./client');
    await apiGet('/api/x');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect(
      (init.headers as Record<string, string>)['x-api-key'],
    ).toBeUndefined();

    vi.unstubAllEnvs();
  });

  test('caller-provided x-api-key wins over the bundle default', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', 'bundle-default');
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    // Act
    const { apiGet } = await import('./client');
    await apiGet('/api/x', { headers: { 'x-api-key': 'caller-override' } });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1] as RequestInit;
    expect((init.headers as Record<string, string>)['x-api-key']).toBe(
      'caller-override',
    );

    vi.unstubAllEnvs();
  });
});

describe('ApiError on non-2xx', () => {
  test('throws ApiError with server-provided message', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'invalid size' }),
    });

    // Act + Assert
    const { apiGet, ApiError } = await import('./client');
    try {
      await apiGet('/api/x');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).message).toBe('invalid size');
      expect((err as InstanceType<typeof ApiError>).status).toBe(400);
    }
  });

  test('falls back to "API error: <status>" when body has no message', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });

    // Act + Assert
    const { apiGet, ApiError } = await import('./client');
    try {
      await apiGet('/api/x');
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(500);
      expect((err as InstanceType<typeof ApiError>).message).toBe(
        'API error: 500',
      );
    }
  });

  test('propagates network failures (TypeError) without wrapping', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

    // Act + Assert
    const { apiGet } = await import('./client');
    await expect(apiGet('/api/x')).rejects.toThrow('Failed to fetch');
  });
});
