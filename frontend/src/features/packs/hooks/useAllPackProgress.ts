import { useQuery } from '@tanstack/react-query';
import { loadAllPackProgress } from '@storage/packProgress';
import type { PackProgress } from '@storage/packProgress';

/**
 * Reads every persisted pack-progress record (keyed by slug) for the
 * browse surface's per-pack local progress badge. Owns the IDB read so
 * the page stays I/O-free.
 *
 * Returns an empty map on failure rather than surfacing an error — the
 * browse list is still useful without progress badges, and a missing
 * badge degrades gracefully to "0/N".
 */
export function useAllPackProgress() {
  return useQuery<Record<string, PackProgress>>({
    queryKey: ['pack-progress-all'],
    retry: false,
    queryFn: async () => {
      try {
        return await loadAllPackProgress();
      } catch {
        return {};
      }
    },
  });
}
