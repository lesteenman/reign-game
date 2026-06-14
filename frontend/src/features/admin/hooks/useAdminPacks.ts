import { useQuery } from '@tanstack/react-query';
import { fetchPacks } from '@features/admin/services/adminPackService';
import type { PackView } from '@shared/types/pack';

/** Query key for the pack list — the single source of cached pack data. */
export const ADMIN_PACKS_KEY = ['adminPacks'] as const;

/**
 * Reads the list of packs via TanStack Query. The mutation hooks
 * reconcile this cache by invalidating `ADMIN_PACKS_KEY`. The
 * `AbortSignal` is threaded through so a superseded/unmounted fetch is
 * cancelled.
 */
export function useAdminPacks() {
  return useQuery<PackView[]>({
    queryKey: ADMIN_PACKS_KEY,
    queryFn: ({ signal }) => fetchPacks(signal),
  });
}
