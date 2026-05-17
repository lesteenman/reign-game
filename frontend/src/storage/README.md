# `src/storage/`

IndexedDB wrapper and persisted shapes. Three production files plus three unit tests.

## Responsibility

The single source of truth for "what gets persisted to IndexedDB". Persisted shapes live here once and are imported from every consumer (lesson 4 — don't redeclare a shape in a hook and storage). The wrapper itself is intentionally thin — just enough to open the DB once, upgrade schemas, and compute composite keys.

## Data flow

- **In:** Imported by `hooks/useGameStorage.ts` (the only CRUD caller in production) and by tests + page-level direct callers in the daily-flow short-circuit path (`DailyFlow.tsx` calls `loadState` through `useGameStorage`, but the seeding path in `playwright/e2e/daily-flow.spec.ts` writes raw IDB).
- **Out:** Reads/writes the `reign-game` IndexedDB database (version 2).

## Files

- **`db.ts`** — `openDB()` (cached promise), `idFor(flowType, flowId)` (composite key construction — the only place the `:` separator appears in production code), `resetDBCache()` (test helper). Schema upgrade from v1 → v2: clears the `gameState` store (pre-Phase-7 rows used `id: 'current'` which is incompatible with the per-flow shape; the slice ships a graceful drop rather than a row-level migration).
- **`types.ts`** — Persisted shapes: `FlowType` ('curation' | 'daily' | 'pack'), `GameHistory`, `GameState`, `CompletionRecord`. Helpers: `parseFlowType` (validates URL param), `buildCurationFlowId` (size+mode → composite id string), `EMPTY_HISTORY` (shared empty literal).
- **`utils.ts`** — `createFreshGameState(flowType, flowId, puzzle)` — the canonical builder for a new GameState.

## State management

Module-level `dbPromise: Promise<IDBDatabase> | null` cached across all callers. Reset via `resetDBCache()` in tests.

## Rules specific to this directory

- **Composite key key construction goes through `idFor`.** The `:` separator appears nowhere else in production code. Callers never construct the `:`-joined string by hand.
- **DB upgrades clear, don't migrate.** Schema upgrade from v1 to v2 dropped the old single-`current` row layout. When the next upgrade lands, the same convention applies unless persistence has matured.
- **Lesson 4: persisted shapes live here.** If you find yourself defining a shape in a hook AND a storage row at the same time, the storage definition wins. Import; don't redeclare.

## Track 3 mapping

Unchanged location.
