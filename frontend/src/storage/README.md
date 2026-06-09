# `src/storage/`

The web-specific IndexedDB implementation of `@reign/core`'s storage contract.

The persisted shapes (`GameState`, `CompletionRecord`, `FlowType`, `GameHistory`), the `GameStorage` interface, and the pure helpers (`idFor`, `createFreshGameState`, `parseFlowType`, `buildCurationFlowId`, `EMPTY_HISTORY`) live in `@reign/core/storage` (#130). This directory holds only the IndexedDB wiring.

## Responsibility

`db.ts` *implements* `@reign/core`'s `GameStorage` over IndexedDB and owns the connection lifecycle. The wrapper is intentionally thin — just enough to open the DB once, upgrade schemas, and run a single transaction per CRUD call.

## Data flow

- **In:** Imported by `hooks/useGameStorage.ts` (the only CRUD caller in production) and by tests + page-level direct callers in the daily-flow short-circuit path (`DailyFlow.tsx` calls `loadState` through `useGameStorage`, but the seeding path in `playwright/e2e/daily-flow.spec.ts` writes raw IDB).
- **Out:** Reads/writes the `reign-game` IndexedDB database (version 2).

## Files

- **`db.ts`** — `openDB()` (cached promise), `resetDBCache()` (test helper), and `indexedDbGameStorage` — the `GameStorage` implementation (`saveState`/`loadState`/`clearState`/`addCompletion`), each wrapping a single IDB transaction. Composite keys come from `idFor` in `@reign/core/storage`. Schema upgrade from v1 → v2: clears the `gameState` store (pre-Phase-7 rows used `id: 'current'` which is incompatible with the per-flow shape; the slice ships a graceful drop rather than a row-level migration).

The persisted shapes, the `GameStorage` interface, and the pure helpers live in `@reign/core/storage`; `hooks/useGameStorage.ts` returns `indexedDbGameStorage` typed as `GameStorage`.

## State management

Module-level `dbPromise: Promise<IDBDatabase> | null` cached across all callers. Reset via `resetDBCache()` in tests.

## Rules specific to this directory

- **Composite key construction goes through `idFor`** (in `@reign/core/storage`). The `:` separator appears nowhere else in production code. Callers never construct the `:`-joined string by hand.
- **DB upgrades clear, don't migrate.** Schema upgrade from v1 to v2 dropped the old single-`current` row layout. When the next upgrade lands, the same convention applies unless persistence has matured.
- **Lesson 4: persisted shapes live in `@reign/core/storage`.** If you find yourself defining a shape in a hook AND a storage row at the same time, the core definition wins. Import; don't redeclare.
