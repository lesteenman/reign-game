import type { PuzzleData } from '@engine/types';

/**
 * Hardcoded 5x5 puzzle used as a deterministic fixture for tests that
 * need a known-good puzzle without booting the backend. Lives in
 * `shared/` so any feature's tests can pull it via `@shared/test-fixtures`.
 *
 * (Previously exported from src/App.tsx; moved here in #198 because it
 * is test-only data and doesn't belong in the production app root.)
 */
export const FALLBACK_PUZZLE: PuzzleData = {
  puzzleId: 'playtest-001',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};
