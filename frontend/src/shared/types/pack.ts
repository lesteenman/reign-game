import type { Mode } from '@reign/core/engine';

/**
 * Pack-related types for the admin assembly UI. Mirror the backend
 * handler DTOs in `backend/internal/handler/pack_dto.go`:
 *
 *   - PackView           → GET /api/admin/packs, GET /api/admin/packs/{slug}
 *   - PackCreateRequest  → POST /api/admin/packs
 *   - PackUpdateRequest  → PUT  /api/admin/packs/{slug}
 *   - PackCandidate      → GET  /api/admin/packs/candidates?size=&mode=
 *
 * Field names match the backend JSON tags exactly (lesson 8: verify the
 * DTO, not the spec).
 */

/** A single pack as returned by the list / get endpoints. */
export interface PackView {
  slug: string;
  name: string;
  size: number;
  mode: Mode;
  published: boolean;
  puzzleIds: string[];
  createdAt: string;
  updatedAt: string;
}

/**
 * Body of POST /api/admin/packs. Slug, size, and mode are carried here
 * (the create endpoint has no path params) and are immutable afterwards.
 */
export interface PackCreateRequest {
  slug: string;
  name: string;
  size: number;
  mode: Mode;
  puzzleIds: string[];
  published: boolean;
}

/**
 * Body of PUT /api/admin/packs/{slug}. Slug, size, and mode are
 * immutable and not carried here.
 */
export interface PackUpdateRequest {
  name: string;
  puzzleIds: string[];
  published: boolean;
}

/** Per-puzzle metadata block on a candidate (mirrors the serve metadata, minus seed). */
export interface PackCandidateMetadata {
  difficulty: number;
  maxTier: number;
  tierCounts: number[];
  traceLen: number;
  generationDurationMs: number;
  createdAt: string;
}

/**
 * A verdict-up puzzle returned by the candidates endpoint — carries the
 * regionMap for a visual preview plus difficulty/tier metadata, no solution.
 */
export interface PackCandidate {
  puzzleId: string;
  regionMap: number[][];
  metadata: PackCandidateMetadata;
}
