import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createConfig } from '@features/admin/services/adminService';
import { ADMIN_POOL_KEY } from './useAdminPool';
import type {
  ConfigView,
  ConfigCreateRequest,
  ComboStatus,
  PoolStatus,
} from '@shared/types/admin';

/** Snapshot carried from onMutate to onError for rollback. */
interface CreateConfigContext {
  previous?: PoolStatus;
}

/**
 * Creates a new combo config with an optimistic cache insert.
 *
 * `onMutate` cancels any in-flight pool query, snapshots the cached
 * `PoolStatus`, and appends the new combo (`readyCount: 0` — the real
 * count only exists server-side) so the row appears immediately.
 * `onError` restores the snapshot; `onSettled` invalidates
 * `ADMIN_POOL_KEY` to reconcile the true `readyCount` in the
 * background (cached table stays visible — no loading flash).
 */
export function useCreateConfig() {
  const queryClient = useQueryClient();

  return useMutation<
    ConfigView,
    Error,
    ConfigCreateRequest,
    CreateConfigContext
  >({
    mutationFn: (req) => createConfig(req),
    onMutate: async (req) => {
      await queryClient.cancelQueries({ queryKey: ADMIN_POOL_KEY });
      const previous = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
      if (previous) {
        const { size, mode, ...config } = req;
        const newCombo: ComboStatus = { size, mode, config, readyCount: 0 };
        queryClient.setQueryData<PoolStatus>(ADMIN_POOL_KEY, {
          combos: [...previous.combos, newCombo],
        });
      }
      return { previous };
    },
    onError: (_err, _req, context) => {
      if (context?.previous) {
        queryClient.setQueryData(ADMIN_POOL_KEY, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_POOL_KEY });
    },
  });
}
