import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';

const MOCK_POOL_STATUS = {
  combos: [
    {
      size: 5,
      mode: 'standard',
      config: {
        pipeline: 'region-first',
        solver: 'backtrack',
        regions: 'bfs',
        regionVariance: 0.3,
        deducible: true,
        concurrency: 2,
        threshold: 3,
        enabled: true,
      },
      readyCount: 2,
    },
    {
      size: 7,
      mode: 'double',
      config: {
        pipeline: 'iterative',
        solver: 'propagation',
        regions: 'wfc',
        regionVariance: 0.5,
        deducible: false,
        concurrency: 4,
        threshold: 5,
        enabled: false,
      },
      readyCount: 0,
    },
  ],
};

const MOCK_CONFIG = {
  pipeline: 'region-first',
  solver: 'backtrack',
  regions: 'bfs',
  regionVariance: 0.3,
  deducible: true,
  concurrency: 2,
  threshold: 3,
  enabled: true,
};

const MOCK_REPLENISH_RESULT = {
  triggered: [{ size: 5, mode: 'standard', count: 1 }],
  skipped: [{ size: 7, mode: 'double', ready: 5 }],
};

describe('fetchPoolStatus', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls GET /admin/pool and returns pool status', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_POOL_STATUS),
    });

    // Act
    const { fetchPoolStatus } = await import('./adminService');
    const result = await fetchPoolStatus();

    // Assert
    expect(result).toEqual(MOCK_POOL_STATUS);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/admin/pool');
  });

  test('throws ApiError on server error', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ message: 'internal error' }),
    });

    // Act & Assert
    const { fetchPoolStatus } = await import('./adminService');
    const { ApiError } = await import('./api');
    try {
      await fetchPoolStatus();
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(500);
    }
  });
});

describe('updateConfig', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls PUT /admin/config/{size}/{mode} with config body', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_CONFIG)),
    });

    // Act
    const { updateConfig } = await import('./adminService');
    const result = await updateConfig(5, 'standard', MOCK_CONFIG);

    // Assert
    expect(result).toEqual(MOCK_CONFIG);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/admin/config/5/standard');
    const options = calls[0]![1] as RequestInit;
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body as string)).toEqual(MOCK_CONFIG);
  });

  test('throws ApiError on 400', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve(JSON.stringify({ message: 'bad request' })),
      json: () => Promise.resolve({ message: 'bad request' }),
    });

    // Act & Assert
    const { updateConfig } = await import('./adminService');
    const { ApiError } = await import('./api');
    try {
      await updateConfig(5, 'standard', MOCK_CONFIG);
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(400);
    }
  });
});

describe('createConfig', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls POST /admin/config with full config payload', async () => {
    // Arrange
    const createPayload = { ...MOCK_CONFIG, size: 9, mode: 'standard' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_CONFIG)),
    });

    // Act
    const { createConfig } = await import('./adminService');
    const result = await createConfig(createPayload);

    // Assert
    expect(result).toEqual(MOCK_CONFIG);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/admin/config');
    const options = calls[0]![1] as RequestInit;
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body as string)).toEqual(createPayload);
  });

  test('throws ApiError on 409 conflict', async () => {
    // Arrange
    const createPayload = { ...MOCK_CONFIG, size: 5, mode: 'standard' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve(JSON.stringify({ message: 'already exists' })),
      json: () => Promise.resolve({ message: 'already exists' }),
    });

    // Act & Assert
    const { createConfig } = await import('./adminService');
    const { ApiError } = await import('./api');
    try {
      await createConfig(createPayload);
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(409);
    }
  });
});

describe('triggerReplenish', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('calls POST /admin/replenish with no params for replenish all', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_REPLENISH_RESULT)),
    });

    // Act
    const { triggerReplenish } = await import('./adminService');
    const result = await triggerReplenish();

    // Assert
    expect(result).toEqual(MOCK_REPLENISH_RESULT);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('/api/admin/replenish');
    expect(url).not.toContain('size=');
    expect(url).not.toContain('mode=');
    const options = calls[0]![1] as RequestInit;
    expect(options.method).toBe('POST');
  });

  test('calls POST /admin/replenish with size and mode params', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_REPLENISH_RESULT)),
    });

    // Act
    const { triggerReplenish } = await import('./adminService');
    await triggerReplenish(5, 'standard');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const url = calls[0]![0] as string;
    expect(url).toContain('size=5');
    expect(url).toContain('mode=standard');
  });

  test('throws ApiError on server error', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve(JSON.stringify({ message: 'internal error' })),
      json: () => Promise.resolve({ message: 'internal error' }),
    });

    // Act & Assert
    const { triggerReplenish } = await import('./adminService');
    const { ApiError } = await import('./api');
    try {
      await triggerReplenish();
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(500);
    }
  });
});
