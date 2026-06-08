import { useMutation, useQueryClient } from '@tanstack/react-query';
import { triggerReplenish } from '@features/admin/services/adminService';
import { ADMIN_POOL_KEY } from './useAdminPool';
import type { Mode } from '@reign/core/engine';
import type { ReplenishResult } from '@shared/types/admin';

/** Args for a replenish mutation. Omit both for "replenish all". */
export interface ReplenishArgs {
  size?: number;
  mode?: Mode;
}

/**
 * Triggers pool replenishment (all combos, or one when size+mode given).
 * Replenish changes server-side `readyCount`s, so `onSettled` invalidates
 * `ADMIN_POOL_KEY` to reconcile from the server — no optimistic patch,
 * since the new counts only exist on the backend. The invalidation
 * refetch runs in the background; the cached table stays visible.
 */
export function useReplenish() {
  const queryClient = useQueryClient();

  return useMutation<ReplenishResult, Error, ReplenishArgs>({
    mutationFn: ({ size, mode } = {}) => triggerReplenish(size, mode),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_POOL_KEY });
    },
  });
}
