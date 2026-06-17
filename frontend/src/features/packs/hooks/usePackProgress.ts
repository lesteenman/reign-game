import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  emptyPackProgress,
  loadPackProgress,
  savePackProgress,
  markSolved,
} from '@storage/packProgress';
import type { PackProgress } from '@storage/packProgress';

/**
 * Owns the IndexedDB I/O for a pack's local progress (frontend/CLAUDE.md:
 * hooks own I/O). Reads the persisted `pack:{slug}` record on mount,
 * defaulting to an empty record when none exists; exposes a `recordSolve`
 * writer that marks a puzzle solved and persists the result.
 *
 * The persisted shape + the pure resume/mark logic live in
 * `@storage/packProgress`; this hook is the React/TanStack wiring only.
 *
 * The query is disabled when `slug` is null so a missing/unknown URL
 * param doesn't trigger a read.
 */
export function usePackProgress(slug: string | null) {
  const queryClient = useQueryClient();
  const query = useQuery<PackProgress>({
    queryKey: ['pack-progress', slug],
    enabled: slug !== null,
    retry: false,
    // Progress is local and authoritative for the session — never
    // re-read it from IDB on refocus and clobber an in-memory advance.
    staleTime: Infinity,
    gcTime: Infinity,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      const persisted = await loadPackProgress(slug as string);
      return persisted ?? emptyPackProgress(slug as string);
    },
  });

  /**
   * Mark `puzzleId` solved within `puzzleIds` (the pack's ordered ids),
   * persist, and update the in-memory query data so the flow advances
   * without a re-read. Returns the new progress so callers can branch
   * on the resulting cursor (e.g. detect pack-complete).
   */
  async function recordSolve(
    puzzleId: string,
    puzzleIds: string[],
  ): Promise<PackProgress> {
    const current = query.data ?? emptyPackProgress(slug as string);
    const next = markSolved(current, puzzleId, puzzleIds);
    await savePackProgress(next);
    queryClient.setQueryData(['pack-progress', slug], next);
    return next;
  }

  return { query, recordSolve };
}
