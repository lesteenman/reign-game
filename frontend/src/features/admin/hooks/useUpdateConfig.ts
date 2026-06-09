import { useMutation, useQueryClient } from '@tanstack/react-query';
import { updateConfig } from '@features/admin/services/adminService';
import { ADMIN_POOL_KEY } from './useAdminPool';
import type { Mode } from '@reign/core/engine';
import type {
  ConfigView,
  ConfigUpdateRequest,
  PoolStatus,
} from '@shared/types/admin';

/** Args for an update-config mutation. */
export interface UpdateConfigArgs {
  size: number;
  mode: Mode;
  config: ConfigUpdateRequest;
}

/** Snapshot carried from onMutate to onError for rollback. */
interface UpdateConfigContext {
  previous?: PoolStatus;
}

/**
 * Updates an existing combo's config with an optimistic cache patch.
 *
 * `onMutate` cancels any in-flight pool query, snapshots the cached
 * `PoolStatus`, and patches the matching combo's `config` so the user
 * sees the new value immediately. `onError` restores the snapshot.
 * `onSettled` invalidates `ADMIN_POOL_KEY` to reconcile with the
 * server in the background (the cached table stays visible — no
 * loading flash).
 */
export function useUpdateConfig() {
  const queryClient = useQueryClient();

  return useMutation<
    ConfigView,
    Error,
    UpdateConfigArgs,
    UpdateConfigContext
  >({
    mutationFn: ({ size, mode, config }) => updateConfig(size, mode, config),
    onMutate: async ({ size, mode, config }) => {
      await queryClient.cancelQueries({ queryKey: ADMIN_POOL_KEY });
      const previous = queryClient.getQueryData<PoolStatus>(ADMIN_POOL_KEY);
      if (previous) {
        queryClient.setQueryData<PoolStatus>(ADMIN_POOL_KEY, {
          combos: previous.combos.map((combo) =>
            combo.size === size && combo.mode === mode
              ? { ...combo, config }
              : combo,
          ),
        });
      }
      return { previous };
    },
    onError: (_err, _args, context) => {
      if (context?.previous) {
        queryClient.setQueryData(ADMIN_POOL_KEY, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_POOL_KEY });
    },
  });
}
