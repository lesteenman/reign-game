import { renderHook, waitFor, act } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useReplenish } from './useReplenish';
import { ADMIN_POOL_KEY } from './useAdminPool';

const mockTriggerReplenish = vi.fn();
vi.mock('@features/admin/services/adminService', () => ({
  triggerReplenish: (...args: unknown[]) => mockTriggerReplenish(...args),
}));

const RESULT = { triggered: [{ size: 5, mode: 'standard', count: 1 }], skipped: [] };

let queryClient: QueryClient;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  mockTriggerReplenish.mockReset();
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
});

describe('useReplenish', () => {
  it('calls triggerReplenish with no args for replenish-all and invalidates the pool', async () => {
    // Arrange
    mockTriggerReplenish.mockResolvedValue(RESULT);
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    // Act
    const { result } = renderHook(() => useReplenish(), { wrapper });
    act(() => result.current.mutate({}));

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockTriggerReplenish).toHaveBeenCalledWith(undefined, undefined);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ADMIN_POOL_KEY });
  });

  it('forwards size and mode for a single-combo replenish', async () => {
    // Arrange
    mockTriggerReplenish.mockResolvedValue(RESULT);

    // Act
    const { result } = renderHook(() => useReplenish(), { wrapper });
    act(() => result.current.mutate({ size: 7, mode: 'double' }));

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockTriggerReplenish).toHaveBeenCalledWith(7, 'double');
  });
});
