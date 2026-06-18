import { useQuery } from '@tanstack/react-query';
import { getPack } from '@features/packs/services/packService';
import { ApiError } from '@shared/api';
import type { PackServeView } from '@features/packs/types/pack';

/**
 * Read query for a single served pack (`GET /api/packs/{slug}`). The
 * query is disabled when `slug` is null (missing/unknown URL param) so
 * the flow can render a not-found state without firing a request.
 *
 * `retry: false` — a 404 (draft / deleted / unpublished pack) is a
 * terminal, user-visible state with a graceful exit, not a transient
 * error to retry.
 */
export function usePack(slug: string | null) {
  return useQuery<PackServeView, ApiError | Error>({
    queryKey: ['pack', slug],
    enabled: slug !== null,
    retry: false,
    queryFn: () => getPack(slug as string),
  });
}
