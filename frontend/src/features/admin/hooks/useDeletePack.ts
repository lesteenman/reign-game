import { useMutation, useQueryClient } from '@tanstack/react-query';
import { deletePack } from '@features/admin/services/adminPackService';
import { ADMIN_PACKS_KEY } from './useAdminPacks';

/**
 * Deletes a pack by slug. On success the pack list is invalidated so
 * the deleted row disappears.
 */
export function useDeletePack() {
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: (slug) => deletePack(slug),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_PACKS_KEY });
    },
  });
}
