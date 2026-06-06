import { renderHook, waitFor, act } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useUpdateConfig } from './useUpdateConfig';
import { ADMIN_POOL_KEY } from './useAdminPool';
import type { PoolStatus } from '@shared/types/admin';

const mockUpdateConfig = vi.fn();
vi.mock('@features/admin/services/adminService', () => ({
  updateConfig: (...args: unknown[]) => mockUpdateConfig(...args),
}));

const SEED_POOL: PoolStatus = {
  combos: [
    { size: 5, mode: 'standard', config: { threshold: 3, enabled: true }, readyCount: 2 },
    { size: 7, mode: 'double', config: { threshold: 5, enabled: false }, readyCount: 0 },
  ],
};

let queryClient: QueryClient;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  mockUpdateConfig.mockReset();
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(ADMIN_POOL_KEY, SEED_POOL);
});

describe('useUpdateConfig', () => {
  it('optimistically patches the matching combo config before the server responds', async () => {
    // Arrange — a deferred mock so the optimistic state is observable.
    let resolve!: (v: unknown) => void;
    mockUpdateConfig.mockReturnValue(new Promise((r) => { resolve = r; }));

    // Act
    const { result } = renderHook(() => useUpdateConfig(), { wrapper });
    act(() => {
      result.current.mutate({
        size: 5,
        mode: 'standard',
        config: { threshold: 9, enabled: false },
      });
    });

    // Assert — cache shows the optimistic value while the request is pending.
    await waitFor(() => {
      const cached = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
      expect(cached?.combos[0]!.config).toEqual({ threshold: 9, enabled: false });
    });
    expect(queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY)?.combos[1]!.config).toEqual({
      threshold: 5,
      enabled: false,
    });

    // Cleanup the pending promise.
    act(() => resolve({ size: 5, mode: 'standard', threshold: 9, enabled: false }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  it('rolls back to the snapshot on error', async () => {
    // Arrange
    mockUpdateConfig.mockRejectedValue(new Error('save failed'));

    // Act
    const { result } = renderHook(() => useUpdateConfig(), { wrapper });
    act(() => {
      result.current.mutate({
        size: 5,
        mode: 'standard',
        config: { threshold: 9, enabled: false },
      });
    });

    // Assert — after the error settles, the cache is back to the snapshot.
    await waitFor(() => expect(result.current.isError).toBe(true));
    const cached = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
    expect(cached?.combos[0]!.config).toEqual({ threshold: 3, enabled: true });
  });

  it('invalidates the pool query on settle', async () => {
    // Arrange
    mockUpdateConfig.mockResolvedValue({ size: 5, mode: 'standard', threshold: 9, enabled: false });
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    // Act
    const { result } = renderHook(() => useUpdateConfig(), { wrapper });
    act(() => {
      result.current.mutate({
        size: 5,
        mode: 'standard',
        config: { threshold: 9, enabled: false },
      });
    });

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ADMIN_POOL_KEY });
  });
});
