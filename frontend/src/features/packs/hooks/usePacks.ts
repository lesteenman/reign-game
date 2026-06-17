import { useQuery } from '@tanstack/react-query';
import { listPacks } from '@features/packs/services/packService';
import { ApiError } from '@shared/api';
import type { PackSummary } from '@features/packs/types/pack';

/**
 * Read query for the packs-browse surface — the published-pack list
 * from `GET /api/packs`. `retry: false` because the browse page renders
 * an explicit error state with a retry affordance; auto-retry would
 * mask it.
 */
export function usePacks() {
  return useQuery<PackSummary[], ApiError | Error>({
    queryKey: ['packs'],
    retry: false,
    queryFn: listPacks,
  });
}
