import { useMutation, useQueryClient } from '@tanstack/react-query';
import { updatePack } from '@features/admin/services/adminPackService';
import { ADMIN_PACKS_KEY } from './useAdminPacks';
import type { PackView, PackUpdateRequest } from '@shared/types/pack';

interface UpdatePackVars {
  slug: string;
  data: PackUpdateRequest;
}

/**
 * Updates an existing pack (name, puzzle ordering, publish state). On
 * success the pack list is invalidated to reconcile the cached rows.
 */
export function useUpdatePack() {
  const queryClient = useQueryClient();

  return useMutation<PackView, Error, UpdatePackVars>({
    mutationFn: ({ slug, data }) => updatePack(slug, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_PACKS_KEY });
    },
  });
}
