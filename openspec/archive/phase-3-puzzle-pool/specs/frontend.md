# Spec: Frontend

Covers R-046 (switch to /puzzles/next), R-047 (generation metadata display), and R-048 (pool-empty state).

## Requirements

### FE-01: API Client Update

- `puzzleService.ts`: add `fetchNextPuzzle(size, mode)` function calling `GET /puzzles/next`
- New types:
  ```typescript
  interface PuzzleMetadata {
    pipeline: string;
    solver: string;
    regions: string;
    regionVariance: number;
    generationDurationMs: number;
    createdAt: string;
  }

  interface PuzzleResponse {
    puzzleId: string;
    gridSize: number;
    mode: string;
    regionMap: number[][];
    metadata: PuzzleMetadata;
  }
  ```
- `fetchNextPuzzle` returns `PuzzleResponse` on 200
- On 404: throws a typed error (e.g., `NoPuzzlesAvailableError`) so the caller can distinguish from network errors
- Remove `GenerateOptions` interface and `generatePuzzle()` function (no longer used)
- Tests: mock fetch for 200 with metadata, 404 throws correct error type, 400/500 throw generic errors

### FE-02: PuzzleSelector Simplification

- Remove the "Advanced" toggle button and collapsed advanced section
- Remove all advanced state: `advPipeline`, `advSolver`, `advRegions`, `advVarianceIndex`, `useAdvanced`
- Keep the four preset buttons: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens
- `onSelect` callback receives `{ size, mode }` only (no pipeline/solver/regions/variance)
- Update `GenerateOptions` type (or replace with a simpler `PuzzleSelection` type) to match
- Tests: presets render, selection works, no advanced section rendered, Play calls onSelect with size + mode only

### FE-03: GamePage Integration

- Replace `generatePuzzle(options)` call with `fetchNextPuzzle(size, mode)`
- Store `metadata` from the response in game state (persisted in IndexedDB)
- Remove URL param handling for `pipeline`, `solver`, `regions`, `regionVariance`
- Play URL simplifies to `/play?new=true&size=7&mode=standard`
- `buildPlayUrl` in `LandingPage.tsx` updated to omit removed params
- Tests: GamePage calls fetchNextPuzzle with correct params, stores metadata, simplified URL params

### FE-04: Pool-Empty State

- When `fetchNextPuzzle` throws `NoPuzzlesAvailableError`:
  - Show message: "No puzzles available for this size and mode"
  - Show a "Retry" button that re-attempts the fetch
  - Do not fall back to on-demand generation
- Styled per BRAND_GUIDELINES.md
- Tests: 404 response renders empty-pool message, retry button re-fetches, successful retry loads puzzle

### FE-05: Metadata Display

- GamePage shows generation metadata when a puzzle is loaded
- Format: compact text like `iterative / propagation / 4.2s`
- Position: below the grid or in a subtle info line
- Always visible (no toggle — current UI is the admin/curation flow)
- Reads metadata from game state (survives page reload)
- If metadata is absent (e.g., old saved game from before this change): show nothing, no error
- Tests: metadata renders with correct values, absent metadata renders nothing, duration formatted correctly

### FE-06: Game State Update

- `PuzzleData` type in `engine/types.ts` gains an optional `metadata?: PuzzleMetadata` field
- `GameState` in `storage/types.ts` carries the metadata through to IndexedDB
- `createFreshGameState` accepts the full `PuzzleResponse` including metadata
- Backward compatible: existing saved games without metadata load without error
- Tests: save + load round-trip includes metadata, old state without metadata loads cleanly

## Acceptance Criteria

All FE-01 through FE-06 pass. Frontend fetches from pool, shows metadata, handles empty pool gracefully. No references to the old `generatePuzzle` function remain in production code.
