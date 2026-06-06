import { renderHook, waitFor } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useAdminPool } from './useAdminPool';
import type { PoolStatus } from '@shared/types/admin';

const mockFetchPoolStatus = vi.fn();
vi.mock('@features/admin/services/adminService', () => ({
  fetchPoolStatus: (...args: unknown[]) => mockFetchPoolStatus(...args),
}));

const MOCK_POOL: PoolStatus = {
  combos: [
    { size: 5, mode: 'standard', config: { threshold: 3, enabled: true }, readyCount: 2 },
  ],
};

beforeEach(() => {
  mockFetchPoolStatus.mockReset();
});

describe('useAdminPool', () => {
  it('returns pool status on success', async () => {
    // Arrange
    mockFetchPoolStatus.mockResolvedValue(MOCK_POOL);

    // Act
    const { result } = renderHook(() => useAdminPool());

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(MOCK_POOL);
  });

  it('surfaces an error when the fetch fails', async () => {
    // Arrange
    mockFetchPoolStatus.mockRejectedValue(new Error('boom'));

    // Act
    const { result } = renderHook(() => useAdminPool());

    // Assert
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('boom');
  });

  it('passes an AbortSignal into the service queryFn', async () => {
    // Arrange
    mockFetchPoolStatus.mockResolvedValue(MOCK_POOL);

    // Act
    const { result } = renderHook(() => useAdminPool());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Assert
    const arg = mockFetchPoolStatus.mock.calls[0]![0];
    expect(arg).toBeInstanceOf(AbortSignal);
  });
});
