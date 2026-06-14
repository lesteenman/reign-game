import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';

const MOCK_PACK = {
  slug: 'starter',
  name: 'Starter',
  size: 7,
  mode: 'standard',
  published: false,
  puzzleIds: ['p1', 'p2'],
  createdAt: '2026-06-14T00:00:00Z',
  updatedAt: '2026-06-14T00:00:00Z',
};

const MOCK_CANDIDATES = [
  {
    puzzleId: 'p1',
    regionMap: [
      [0, 1],
      [0, 1],
    ],
    metadata: {
      difficulty: 3,
      maxTier: 2,
      tierCounts: [1, 2],
      traceLen: 5,
      generationDurationMs: 120,
      createdAt: '2026-06-14T00:00:00Z',
    },
  },
];

describe('adminPackService', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  test('fetchPacks GETs /api/admin/packs', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify([MOCK_PACK])),
    });

    // Act
    const { fetchPacks } = await import('./adminPackService');
    const result = await fetchPacks();

    // Assert
    expect(result).toEqual([MOCK_PACK]);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0]![0] as string).toContain('/api/admin/packs');
  });

  test('fetchPackCandidates GETs candidates with size+mode params', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_CANDIDATES)),
    });

    // Act
    const { fetchPackCandidates } = await import('./adminPackService');
    const result = await fetchPackCandidates(7, 'standard');

    // Assert
    expect(result).toEqual(MOCK_CANDIDATES);
    const url = (globalThis.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0]![0] as string;
    expect(url).toContain('/api/admin/packs/candidates');
    expect(url).toContain('size=7');
    expect(url).toContain('mode=standard');
  });

  test('createPack POSTs to /api/admin/packs with the body', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_PACK)),
    });
    const payload = {
      slug: 'starter',
      name: 'Starter',
      size: 7,
      mode: 'standard' as const,
      puzzleIds: ['p1'],
      published: false,
    };

    // Act
    const { createPack } = await import('./adminPackService');
    const result = await createPack(payload);

    // Assert
    expect(result).toEqual(MOCK_PACK);
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0]![0] as string).toContain('/api/admin/packs');
    const init = calls[0]![1] as RequestInit;
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });

  test('createPack throws ApiError on 409 duplicate slug', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ message: 'pack already exists: starter' }),
    });

    // Act & Assert
    const { createPack } = await import('./adminPackService');
    const { ApiError } = await import('@shared/api');
    try {
      await createPack({
        slug: 'starter',
        name: 'Starter',
        size: 7,
        mode: 'standard',
        puzzleIds: [],
        published: false,
      });
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(409);
      expect((err as Error).message).toBe('pack already exists: starter');
    }
  });

  test('updatePack PUTs to /api/admin/packs/{slug}', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(MOCK_PACK)),
    });

    // Act
    const { updatePack } = await import('./adminPackService');
    await updatePack('starter', {
      name: 'Renamed',
      puzzleIds: ['p2', 'p1'],
      published: true,
    });

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0]![0] as string).toContain('/api/admin/packs/starter');
    expect((calls[0]![1] as RequestInit).method).toBe('PUT');
  });

  test('deletePack DELETEs /api/admin/packs/{slug}', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    // Act
    const { deletePack } = await import('./adminPackService');
    await deletePack('starter');

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0]![0] as string).toContain('/api/admin/packs/starter');
    expect((calls[0]![1] as RequestInit).method).toBe('DELETE');
  });

  test('updatePack throws ApiError on 422 invalid puzzle', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 422,
      json: () =>
        Promise.resolve({ message: 'puzzle p9 is invalid: not verdict-up' }),
    });

    // Act & Assert
    const { updatePack } = await import('./adminPackService');
    const { ApiError } = await import('@shared/api');
    try {
      await updatePack('starter', {
        name: 'x',
        puzzleIds: ['p9'],
        published: false,
      });
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as InstanceType<typeof ApiError>).status).toBe(422);
    }
  });
});
