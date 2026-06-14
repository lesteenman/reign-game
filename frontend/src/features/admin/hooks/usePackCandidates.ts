import { useQuery } from '@tanstack/react-query';
import { fetchPackCandidates } from '@features/admin/services/adminPackService';
import type { Mode } from '@reign/core/engine';
import type { PackCandidate } from '@shared/types/pack';

/** Query key for the candidate list of a size+mode combo. */
export function packCandidatesKey(size: number, mode: Mode) {
  return ['packCandidates', size, mode] as const;
}

/**
 * Reads verdict-up candidate puzzles for a size+mode combo. Disabled
 * until both `size` and `mode` are set so an unselected combo doesn't
 * fire a request; a freshly-selected combo loads on mount.
 */
export function usePackCandidates(size: number | null, mode: Mode | null) {
  return useQuery<PackCandidate[]>({
    queryKey: packCandidatesKey(size ?? 0, mode ?? 'standard'),
    queryFn: ({ signal }) => fetchPackCandidates(size!, mode!, signal),
    enabled: size !== null && mode !== null,
  });
}
