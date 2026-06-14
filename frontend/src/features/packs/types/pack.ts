import type { PuzzleMetadata } from '@reign/core/engine';

/**
 * Pack DTOs — mirror the backend handler shapes in
 * `backend/internal/handler/pack_dto.go`. Defined here (not in
 * services/) so screens and tests import types from the source rather
 * than through the service module (frontend/CLAUDE.md import rules).
 */

/**
 * One published-pack summary from `GET /api/packs` (the `PackManifest`
 * Go DTO). `size`/`mode` are pack-level (single-combo pack);
 * `puzzleCount` is the number of puzzles the pack holds.
 */
export interface PackSummary {
  slug: string;
  name: string;
  size: number;
  mode: string;
  puzzleCount: number;
}

/**
 * One puzzle's play data within a served pack — the `PackServePuzzle`
 * Go DTO. No solution. `regionMap` matches `PuzzleData.regionMap`;
 * `metadata` matches the serve metadata block (`PuzzleMetadata`).
 */
export interface PackServePuzzle {
  puzzleId: string;
  regionMap: number[][];
  metadata: PuzzleMetadata;
}

/**
 * The `GET /api/packs/{slug}` response — the `PackServeView` Go DTO:
 * the manifest plus every puzzle's play data.
 */
export interface PackServeView {
  pack: PackSummary;
  puzzles: PackServePuzzle[];
}
