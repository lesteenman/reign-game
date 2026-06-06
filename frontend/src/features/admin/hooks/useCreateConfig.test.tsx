import { renderHook, waitFor, act } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useCreateConfig } from './useCreateConfig';
import { ADMIN_POOL_KEY } from './useAdminPool';
import type { PoolStatus } from '@shared/types/admin';

const mockCreateConfig = vi.fn();
vi.mock('@features/admin/services/adminService', () => ({
  createConfig: (...args: unknown[]) => mockCreateConfig(...args),
}));

const SEED_POOL: PoolStatus = {
  combos: [
    { size: 5, mode: 'standard', config: { threshold: 3, enabled: true }, readyCount: 2 },
  ],
};

let queryClient: QueryClient;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  mockCreateConfig.mockReset();
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(ADMIN_POOL_KEY, SEED_POOL);
});

describe('useCreateConfig', () => {
  it('optimistically appends the new combo before the server responds', async () => {
    // Arrange — deferred so the optimistic insert is observable.
    let resolve!: (v: unknown) => void;
    mockCreateConfig.mockReturnValue(new Promise((r) => { resolve = r; }));

    // Act
    const { result } = renderHook(() => useCreateConfig(), { wrapper });
    act(() => {
      result.current.mutate({ size: 9, mode: 'standard', threshold: 4, enabled: true });
    });

    // Assert
    await waitFor(() => {
      const cached = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
      expect(cached?.combos).toHaveLength(2);
    });
    const cached = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
    expect(cached!.combos[1]).toEqual({
      size: 9,
      mode: 'standard',
      config: { threshold: 4, enabled: true },
      readyCount: 0,
    });

    act(() => resolve({ size: 9, mode: 'standard', threshold: 4, enabled: true }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  it('rolls back on error', async () => {
    // Arrange
    mockCreateConfig.mockRejectedValue(new Error('create failed'));

    // Act
    const { result } = renderHook(() => useCreateConfig(), { wrapper });
    act(() => {
      result.current.mutate({ size: 9, mode: 'standard', threshold: 4, enabled: true });
    });

    // Assert
    await waitFor(() => expect(result.current.isError).toBe(true));
    const cached = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
    expect(cached?.combos).toHaveLength(1);
  });

  it('invalidates the pool query on settle', async () => {
    // Arrange
    mockCreateConfig.mockResolvedValue({ size: 9, mode: 'standard', threshold: 4, enabled: true });
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    // Act
    const { result } = renderHook(() => useCreateConfig(), { wrapper });
    act(() => {
      result.current.mutate({ size: 9, mode: 'standard', threshold: 4, enabled: true });
    });

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ADMIN_POOL_KEY });
  });
});
