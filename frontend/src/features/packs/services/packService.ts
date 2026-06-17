// Pack service — public read surface for the player pack-play vertical.
//
// Wraps @shared/api (apiGet) against the unauthenticated pack endpoints:
//   GET /api/packs          → published-pack summaries (browse)
//   GET /api/packs/{slug}    → manifest + every puzzle's play data
//
// No X-Device-Id / auth headers — these routes are public reads.

import { apiGet } from '@shared/api';
import type { PackSummary, PackServeView } from '@features/packs/types/pack';

export type { PackSummary, PackServeView };

/** List published packs (summary shape) for the browse surface. */
export async function listPacks(): Promise<PackSummary[]> {
  return await apiGet<PackSummary[]>('/api/packs');
}

/**
 * Fetch a published pack's manifest + all puzzles' play data. Throws
 * `ApiError` with status 404 for a draft or unknown slug (the backend
 * makes draft/unknown indistinguishable so drafts stay undiscoverable).
 */
export async function getPack(slug: string): Promise<PackServeView> {
  return await apiGet<PackServeView>(`/api/packs/${slug}`);
}
