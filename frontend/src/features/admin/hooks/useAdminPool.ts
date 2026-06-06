import { useQuery } from '@tanstack/react-query';
import { fetchPoolStatus } from '@features/admin/services/adminService';
import type { PoolStatus } from '@shared/types/admin';

/** Query key for the admin pool status — the single source of cached pool data. */
export const ADMIN_POOL_KEY = ['adminPool'] as const;

/**
 * Reads the admin pool status via TanStack Query. Replaces the
 * hand-rolled `useState` + `useEffect` + `loadPool()` cascade in
 * `AdminPage`. The mutation hooks reconcile this cache by invalidating
 * `ADMIN_POOL_KEY` (and, for config edits, optimistically patching it).
 *
 * TanStack passes an `AbortSignal` into `queryFn`; we thread it through
 * to `fetchPoolStatus` so a superseded/unmounted fetch is cancelled.
 */
export function useAdminPool() {
  return useQuery<PoolStatus>({
    queryKey: ADMIN_POOL_KEY,
    queryFn: ({ signal }) => fetchPoolStatus(signal),
  });
}
