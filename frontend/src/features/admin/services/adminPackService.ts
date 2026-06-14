import { apiGet, apiPut, apiPost, apiDelete } from '@shared/api';
import type { Mode } from '@reign/core/engine';
import type {
  PackView,
  PackCreateRequest,
  PackUpdateRequest,
  PackCandidate,
} from '@shared/types/pack';

/** List all packs (GET /api/admin/packs). */
export async function fetchPacks(signal?: AbortSignal): Promise<PackView[]> {
  return apiGet<PackView[]>('/api/admin/packs', { signal });
}

/** Fetch a single pack by slug (GET /api/admin/packs/{slug}). */
export async function fetchPack(
  slug: string,
  signal?: AbortSignal,
): Promise<PackView> {
  return apiGet<PackView>(`/api/admin/packs/${slug}`, { signal });
}

/**
 * Fetch verdict-up candidate puzzles for a size+mode combo
 * (GET /api/admin/packs/candidates?size=&mode=).
 */
export async function fetchPackCandidates(
  size: number,
  mode: Mode,
  signal?: AbortSignal,
): Promise<PackCandidate[]> {
  return apiGet<PackCandidate[]>('/api/admin/packs/candidates', {
    signal,
    params: { size: String(size), mode },
  });
}

/** Create a pack (POST /api/admin/packs). */
export async function createPack(data: PackCreateRequest): Promise<PackView> {
  return apiPost<PackView>('/api/admin/packs', data);
}

/** Update an existing pack (PUT /api/admin/packs/{slug}). */
export async function updatePack(
  slug: string,
  data: PackUpdateRequest,
): Promise<PackView> {
  return apiPut<PackView>(`/api/admin/packs/${slug}`, data);
}

/** Delete a pack (DELETE /api/admin/packs/{slug}). */
export async function deletePack(slug: string): Promise<void> {
  await apiDelete<unknown>(`/api/admin/packs/${slug}`);
}
