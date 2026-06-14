import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createPack } from '@features/admin/services/adminPackService';
import { ADMIN_PACKS_KEY } from './useAdminPacks';
import type { PackView, PackCreateRequest } from '@shared/types/pack';

/**
 * Creates a pack. On success the pack list is invalidated so the new
 * pack appears. No optimistic insert — the server is the source of
 * truth for slug validity (duplicate slug → 409) and the created row.
 */
export function useCreatePack() {
  const queryClient = useQueryClient();

  return useMutation<PackView, Error, PackCreateRequest>({
    mutationFn: (req) => createPack(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ADMIN_PACKS_KEY });
    },
  });
}
