import { useMutation } from '@tanstack/react-query';
import { updatePuzzleStatus } from '@services/puzzleService';
import type { Mode } from '@engine/types';

interface UpdatePuzzleStatusArgs {
  puzzleId: string;
  size: number;
  mode: Mode;
  status: 'solved' | 'skipped';
}

/** Wraps the puzzle status update in a TanStack useMutation so leaf
 *  components don't import services directly (leaf-I/O rule). */
export function useUpdatePuzzleStatus() {
  return useMutation({
    mutationFn: ({ puzzleId, size, mode, status }: UpdatePuzzleStatusArgs) =>
      updatePuzzleStatus(puzzleId, size, mode, status),
  });
}
