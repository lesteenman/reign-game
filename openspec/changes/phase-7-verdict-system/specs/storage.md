# Spec: Per-Flow IndexedDB Storage

The frontend persistence contract for in-progress play state. Replaces the pre-Phase-7 single-slot pattern (`{ id: 'current' }`) with a per-flow shape so switching between curation pools — and, in later phases, between curation / daily / packs — no longer implicitly skips the prior puzzle.

## ST-01: In-progress play state is keyed by `(flowType, flowId)`

**Rule.** The `gameState` IndexedDB object store keeps `keyPath: 'id'`, but the `id` value becomes a composite string `"{flowType}:{flowId}"` instead of the literal `'current'`. Each `(flowType, flowId)` slot — a **Flow Slot** — holds at most one in-progress puzzle. Switching pools writes a different slot and leaves the prior one untouched.

Examples:
- `id: 'curation:5x5-standard'` — admin in-progress 5×5 standard puzzle from the curation flow.
- `id: 'curation:7x7-standard'` — sibling slot, independent.
- `id: 'daily:2026-04-29'` — future daily-flow slot (Phase 8+).
- `id: 'pack:beginner-001'` — future pack-flow slot (Phase 8+).

**Value.** Pool-switching no longer overwrites another in-progress puzzle. The same shape generalizes to daily and pack flows when they ship — no second migration. R-7-02 added the explicit Skip button so the *only* path to a `skipped` status is intentional; per-flow keying is the storage half of that contract.

**Verification.** Test (`db.test.ts` round-trip): write a row with `flowType='curation', flowId='5x5-standard'` → store has one row keyed `'curation:5x5-standard'`. Write a second row with `flowType='curation', flowId='7x7-standard'` → store has two rows. Neither overwrites the other.

## ST-02: `FlowType` is a typed union; `flowId` is a string

**Rule.** `frontend/src/storage/types.ts` declares:

```ts
export type FlowType = 'curation' | 'daily' | 'pack';
```

Adding a new flow type later is a one-character union extension. `flowId` stays `string` — its per-flow shape (e.g., `<size>x<size>-<mode>` for curation, ISO date for daily, pack slug for pack) is convention, not a type-level constraint. Curation is the only flow wired this slice; `'daily'` and `'pack'` are pre-declared so future slices add producers without touching the storage layer.

**Value.** Compile-time catches "I typed `'pack'` but the producer wrote `'packs'`" — a class of bug that's nearly invisible at runtime because the slot just silently doesn't resume. Cost of typing is one union, one place. Cost of regretting an open `string` is debugging silent-no-resume.

**Verification.** `tsc -b` rejects a producer that passes `flowType: 'curations'` (typo). Test: `loadState('curation', '5x5-standard')` and `loadState('daily', '2026-04-29')` are both accepted.

## ST-03: `GameState` carries `flowType` and `flowId`; the key is derived

**Rule.** The persisted `GameState` interface gains two fields:

```ts
export interface GameState {
  id: string;            // <-- now a composite key, not the literal 'current'
  flowType: FlowType;    // new
  flowId: string;        // new
  puzzle: PuzzleData;
  cells: CellState[][];
  timer: { ... };
  status: 'in-progress' | 'solved';
  startedAt: number;
  history?: GameHistory;
}
```

The `id` field is computed as `"${flowType}:${flowId}"` at write time inside `useGameStorage` — callers pass `(flowType, flowId)` as separate arguments and never construct the composite string themselves. `flowType` / `flowId` are duplicated onto the row body for readability when scanning IndexedDB in DevTools (cheap; ~30 bytes per row).

**Value.** Single source of truth for the key shape. A future "list all in-progress slots" feature reads `flowType` / `flowId` from the row body instead of re-parsing the composite key. The composite-key construction lives in one helper in `storage/db.ts`, not in every caller.

**Verification.** Test: a `GameState` written via `saveState({flowType: 'curation', flowId: '5x5-standard', ...})` has both `id === 'curation:5x5-standard'` and `flowType === 'curation'`, `flowId === '5x5-standard'` on read-back. Test: code review — no callers build the `id` string by hand.

## ST-04: `DB_VERSION` bumps to 2; existing rows are wiped on upgrade

**Rule.** `DB_VERSION` is bumped from `1` to `2`. The `onupgradeneeded` handler clears the `gameState` object store when the previous version is `< 2`. The `completions` store is preserved (its keyPath / shape are unchanged). No row-level migration: pre-upgrade `{id: 'current'}` rows are dropped on first post-deploy load.

**Value.** Audience is admins (the only role that produces in-progress state today). The cost of "lose one in-progress puzzle on upgrade day" is negligible; the cost of writing and forever-shipping a one-shot migration that infers `(flowType, flowId)` from `puzzle.gridSize` + `puzzle.mode` is non-zero engineering effort, plus null-check branches for very-old rows that lack one of those fields.

**Verification.** Test (`db.test.ts` upgrade path): seed IndexedDB with a v1 `{id: 'current', ...}` row, bump version to 2 via `openDB()`, verify the `gameState` store is empty afterward and `completions` is preserved. Manual: deploy → admin's first navigation to the curation flow fetches a fresh puzzle (no resume).

## ST-05: At most one row per Flow Slot — upsert semantics

**Rule.** Every write goes through `IDBObjectStore.put(...)` keyed by the composite `id`. A repeated write to the same slot overwrites; the row count for that slot stays at one. Two-rows-per-slot is structurally impossible with `keyPath: 'id'` and `put`.

**Value.** No duplicate-row accumulation. No "which row is canonical" question. The slot is either present (one row) or absent (zero rows).

**Verification.** Test: write the same `(curation, 5x5-standard)` slot three times with different cell states; `getAll()` against the store returns exactly one row with the third write's content.

## ST-06: Resume contract — URL specifies the flow; storage decides resume vs. fresh

**Rule.** `/play` URLs always carry the flow + flow-identifying parameters:

- Curation: `/play?flow=curation&size=5&mode=standard`
- Daily (future): `/play?flow=daily&date=2026-04-29`
- Pack (future): `/play?flow=pack&id=beginner-001`

`GamePage` derives `(flowType, flowId)` from the query params, then calls `loadState(flowType, flowId)`. The branching:

- **Hit, `status !== 'solved'`** → resume the in-progress puzzle. Render from the persisted `cells` / `history` / `timer`.
- **Hit, `status === 'solved'`** → treat as miss (defensive — the slot should have been cleared per ST-07, but a stale row from a crashed completion is recoverable). Fetch a new puzzle.
- **Miss** → fetch a new puzzle via `fetchNextPuzzle(flowType, ...)`, then `saveState(createFreshGameState(flowType, flowId, puzzle))`.

**Value.** The URL carries enough information to identify the slot; the storage layer carries enough information to decide resume vs. fresh. No `?new=true` flag needed (see ST-08). One code path.

**Verification.** Test (`GamePage.test.tsx`): mount with `/play?flow=curation&size=5&mode=standard` and a pre-seeded matching slot in IDB → resume path renders persisted cells. Mount with same URL and no slot → fetch path runs and saves a fresh slot. Mount with same URL and a `status: 'solved'` slot → fetch path runs (defensive). Manual: reload mid-puzzle → puzzle resumes; switch pools → slot for prior pool is unchanged on next visit.

## ST-07: Clear-on-solve — completion clears the Flow Slot

**Rule.** The completion handler in `GamePage` (the same place that calls `addCompletion`) calls `clearState(flowType, flowId)` after a successful solve. The slot returns to "no in-progress" state. Visiting that flow + flowId again hits the miss branch in ST-06 and fetches fresh.

**Value.** Without clear-on-solve, ST-06's resume path would resume a `status: 'solved'` puzzle on the next visit — silly. ST-06's defensive fallback handles the crash case; ST-07 keeps the happy path clean.

**Verification.** Test: solve a puzzle in `(curation, 5x5-standard)` → after the completion overlay is shown, IDB has zero rows for that slot. Test: re-mount `/play?flow=curation&size=5&mode=standard` after a solve → fetch path runs.

## ST-08: The `?new=true` URL contract is removed

**Rule.** `/play` no longer recognizes `new=true`. Producers (CurationPage, the completion overlay's "Play Again" button, any other caller) drop the parameter. Consumers (GamePage's effect that reads `searchParams.get('new')`) drop the branch. Tests that asserted `new=true` behavior are updated or removed. The flow + flow-identifying params (ST-06) plus clear-on-solve (ST-07) cover every UX the parameter served.

The removal is a sweep: `grep -rn "new=true\|searchParams.*['\"]new['\"]" frontend/src` after the slice should return zero matches in production code (test files that assert the parameter is *gone* are fine).

**Value.** Two ways to do the same thing — "force a fresh puzzle" — was already a smell. With clear-on-solve, the natural URL after a completion (the same `/play?flow=...&size=...&mode=...`) does the right thing without a flag. One code path; less to test; less to drift.

**Verification.** Grep at slice-close: zero `new=true` references in `frontend/src` outside of negative-assertion tests. Test: clicking "Play Again" on the completion overlay navigates to `/play?flow=curation&size=5&mode=standard` (no `new=true`) and lands on a fresh puzzle (because ST-07 already cleared the slot).

## ST-09: `useGameStorage` API takes `(flowType, flowId)`

**Rule.** The hook's signatures change:

```ts
saveState(state: GameState): Promise<void>          // unchanged signature; state.id already encodes the slot
loadState(flowType: FlowType, flowId: string): Promise<GameState | null>
clearState(flowType: FlowType, flowId: string): Promise<void>
addCompletion(record: CompletionRecord): Promise<void>  // unchanged
```

Composite-key construction lives inside `loadState` / `clearState` (a single `idFor(flowType, flowId)` helper in `storage/db.ts`). `saveState` reads `state.id` directly because the caller already constructed the full state via `createFreshGameState(flowType, flowId, puzzle)`.

**Value.** Callers pass the typed `(flowType, flowId)` pair; the hook owns key construction. No string concatenation in component code.

**Verification.** Test: `loadState('curation', '5x5-standard')` and `clearState('curation', '5x5-standard')` operate on the same row written by `saveState({flowType: 'curation', flowId: '5x5-standard', ...})`. Test: `idFor` is the only place the `':'` separator appears in production code (`grep -rn "':'\\|\":\"" frontend/src/storage`).

## ST-10: Cross-tab safety — last-write-wins, unchanged from today

**Rule.** Two tabs open on `/play?flow=curation&size=5&mode=standard` both write to the same slot. IndexedDB is per-origin shared; `put` is last-write-wins on a per-transaction basis. This is identical to the pre-Phase-7 behavior of two tabs writing to `id: 'current'` — no new failure mode.

**Value.** No new locking, no per-tab BroadcastChannel coordination. The slice ships the storage shape; multi-tab UX is a separate problem if it ever becomes one (it has not, in three phases of solo-user-style use).

**Verification.** Documented in `design.md` Risks section. No code or test changes needed.

## ST-11: `flowType` consistency in code

**Rule.** Every producer of a `flow` URL parameter and every consumer that derives `flowType` MUST use a string from the `FlowType` union. `frontend/src/storage/types.ts` is the single source of truth. No raw `'curation'` / `'daily'` / `'pack'` literals appear outside type-safe contexts (i.e., they're either the union value itself or the `FlowType`-typed variable that holds it).

URL-derivation: a small helper `parseFlowType(raw: string | null): FlowType | null` lives next to the type and returns null for unknown / missing values. `GamePage` uses it on `searchParams.get('flow')`; an unknown value redirects to `/` (consistent with how an unknown URL today is handled).

**Value.** Hard validation at the URL boundary (per CLAUDE.md lesson 9 — validate URL params before type assertion). A typo'd `?flow=curations` redirects to home rather than running a `loadState('curations', ...)` that silently misses.

**Verification.** Test (`GamePage.test.tsx`): mount with `/play?flow=junk&size=5&mode=standard` → redirect to `/`. Mount with `/play?flow=curation&size=99&mode=standard` (valid flow, invalid size) → existing size-validation path catches it (per Phase-4 lesson 9, no regression here).

## ST-12: No GSI / no DynamoDB schema impact

**Rule.** Per-flow storage is a frontend-only change. No backend repository methods change. No DynamoDB attributes change. No Terraform diff.

**Value.** Risk envelope is contained to the frontend and one IDB version bump. Backend tests do not need to re-run for this slice.

**Verification.** R-7-03 PR's `git diff` against `main` touches zero files under `backend/` and `infra/`. Code review: no new repository methods.
