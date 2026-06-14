import { renderHook, waitFor } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useAdminPacks } from './useAdminPacks';
import type { PackView } from '@shared/types/pack';

const mockFetchPacks = vi.fn();
vi.mock('@features/admin/services/adminPackService', () => ({
  fetchPacks: (...args: unknown[]) => mockFetchPacks(...args),
}));

const MOCK_PACKS: PackView[] = [
  {
    slug: 'starter',
    name: 'Starter',
    size: 7,
    mode: 'standard',
    published: false,
    puzzleIds: ['p1'],
    createdAt: '2026-06-14T00:00:00Z',
    updatedAt: '2026-06-14T00:00:00Z',
  },
];

beforeEach(() => {
  mockFetchPacks.mockReset();
});

describe('useAdminPacks', () => {
  it('returns the pack list on success', async () => {
    // Arrange
    mockFetchPacks.mockResolvedValue(MOCK_PACKS);

    // Act
    const { result } = renderHook(() => useAdminPacks());

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(MOCK_PACKS);
  });

  it('surfaces an error when the fetch fails', async () => {
    // Arrange
    mockFetchPacks.mockRejectedValue(new Error('boom'));

    // Act
    const { result } = renderHook(() => useAdminPacks());

    // Assert
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('boom');
  });

  it('passes an AbortSignal into the service', async () => {
    // Arrange
    mockFetchPacks.mockResolvedValue(MOCK_PACKS);

    // Act
    const { result } = renderHook(() => useAdminPacks());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Assert
    expect(mockFetchPacks.mock.calls[0]![0]).toBeInstanceOf(AbortSignal);
  });
});
