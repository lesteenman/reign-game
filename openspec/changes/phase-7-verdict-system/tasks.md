# Phase 7 Tasks: Verdict System

## Slice Dependency Graph

```
R-081 (Backend handler + repository + schema migration)
    │
    └── R-7-02 (Frontend verdict surface + landing reorg + curation route + explicit Skip + glossary + close-out)
            │
            └── R-7-03 (Per-flow IndexedDB storage — no implicit skips)
```

R-081 (historical name, retained) ships first. R-7-02 depends on R-081's API being live (the frontend posts to it) and folds the landing-page reorganization, the curation route, the explicit Skip button, the docs / glossary / ROADMAP sweep, and the verdict surface into one slice. R-7-03 follows R-7-02 — it reworks `frontend/src/storage/db.ts` to a `(flowType, flowId) → currentPuzzleId` shape so each pool keeps its in-progress puzzle independently and switching pools no longer implicitly skips.

ID-scheme note: this phase uses the new `R-<phase>-<slice>` naming for new slices. R-081 keeps its historical name because it shipped under the old scheme.

## Status

All Phase 7 slices are `[ ]` until completed. Per CLAUDE.md lesson 17, each slice's PR must flip its row to `[x]` as a required artifact — no post-hoc sweeps.

| ID     | Slice                                                                                 | Layer | Status |
|--------|---------------------------------------------------------------------------------------|-------|--------|
| R-081  | Backend verdict handler + repository + schema migration                               | 1     | [x]    |
| R-7-02 | Frontend verdict surface + landing reorg + curation route + explicit Skip + close-out | 2     | [x]    |
| R-7-03 | Per-flow IndexedDB storage (no implicit skips)                                        | 3     | [ ]    |

## Tasks

### R-081: Backend verdict handler + repository + schema migration

- **Roadmap:** R-081
- **Agent:** backend-dev
- **OpenSpec:** `specs/admin-handler.md` (VH-01 through VH-11), `specs/repository.md` (VR-01 through VR-10)

**Work**

- Extend `backend/internal/repository/puzzle.go`:
  - Remove the `Verdict string \`dynamodbav:"verdict"\`` field from `PuzzleRecord`.
  - Add `VerdictSummary` struct (`Up int`, `Down int`, `LastUpdatedAt string`) with `dynamodbav` and `json` tags.
  - Add `VerdictSummary VerdictSummary \`dynamodbav:"verdictSummary"\`` field on `PuzzleRecord`.
  - Add `VerdictRecord` struct per VR-02. PK / SK derived fields tagged `dynamodbav:"-"`.
  - Add `PutVerdict(ctx, *VerdictRecord) error` — unconditional `PutItem` to the `puzzle-pool` table with PK = `"VERDICT#{size}#{mode}#{puzzleId}"`, SK = `"{raterRole}#{raterId}"`.
  - Add `ListVerdictsForPuzzle(ctx, size, mode, puzzleID) ([]VerdictRecord, error)` — single Query against the verdict partition.
  - Add `RecomputeVerdictSummary(ctx, size, mode, puzzleID) (VerdictSummary, error)` — read row family, count up/down, write to `PuzzleRecord.verdictSummary` via `UpdateItem` with `attribute_exists(PK)`.
  - Add `GetPuzzle(ctx, size, mode, puzzleID) (*PuzzleRecord, error)` — single `GetItem` for the 404 check in the handler. Returns `(nil, nil)` if absent.
- Add tests in `backend/internal/repository/puzzle_test.go`:
  - VerdictRecord round-trip (marshal → write → query → unmarshal preserves all fields including PK/SK-derived).
  - `PutVerdict` overwrite proof (three writes, one row).
  - `ListVerdictsForPuzzle` returns expected rows / empty slice on never-voted puzzle.
  - `RecomputeVerdictSummary` correctness across overwrite and multi-rater cases.
  - Legacy-row tolerance: marshal an item with `verdict: "none"` and no `verdictSummary`, unmarshal as PuzzleRecord, verify zero-value summary and no decode error.
- Sweep: grep for every remaining `verdict` literal:
  - `worker/generator.go` line 145 — drop the `Verdict: "none"` initialization.
  - `worker/generator_test.go` lines 140–141 — drop the assertion.
  - `cmd/genfixtures/main.go` line 166 — drop the `"verdict": s("none")` seed.
  - Confirm no other consumers via `grep -rn "[Vv]erdict" backend/ --include="*.go"`. The only remaining matches after the sweep should be the new `VerdictRecord` / `VerdictSummary` / handler code.
- Create `backend/internal/handler/verdict.go`:
  - `VerdictHandler(repo VerdictRepo) http.HandlerFunc` per VH-01..VH-11.
  - Reads `auth.UserFromContext(r.Context())` to source `raterId = u.ID`. Hard-codes `raterRole = "admin"` (VH-07).
  - Validates body (`value` ∈ {up,down}, `outcome` ∈ {solved,skipped}, `playTimeMs` ≥ 0).
  - Validates query params (`size`, `mode` required).
  - Calls `repo.GetPuzzle` → 404 on absent.
  - Calls `repo.PutVerdict`.
  - Calls `repo.RecomputeVerdictSummary`. On error, log WARN and still respond 200 (VH-09).
  - Responds with `{ summary: {...} }`.
- Create `backend/internal/handler/verdict_test.go`:
  - Auth matrix (anonymous → 401, user → 403, admin → 200) using `mountAdminWithAuth` helper from `auth_test.go`.
  - Validation: invalid value, invalid outcome, negative `playTimeMs`, missing query params → 400.
  - 404: puzzle not in DynamoDB → 404, no verdict row written.
  - Idempotency: same admin three writes (up/down/up), only one row, summary reflects last value.
  - Multi-rater: two admins vote, summary counts both rows.
  - VH-06: malicious `raterId` in body is ignored; row's SK uses authenticated user ID.
  - VH-09: failure-injected `RecomputeVerdictSummary` → handler still returns 200 + WARN log.
- Wire the route in `backend/cmd/api/main.go`:
  - Inside the existing `r.Route("/admin", func(r chi.Router) { ... })` block, add `r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))`.
- Update `PROJECT_STRUCTURE.md` API endpoints table:
  - Move the verdict row from "Future" to "Implemented" with auth=Admin.
  - Add `backend/internal/handler/verdict.go` to the backend tree.
- Flip the R-081 row in this `tasks.md` from `[ ]` to `[x]` as part of the same PR.

**Gate**

- `go build ./...` passes.
- `go test -short ./...` green.
- `golangci-lint run` green.
- Grep sweep clean: no `verdict: "none"` writes remain, no `Verdict string` field references remain.
- Manual: `task dev:up` → `curl -X PUT http://localhost:5181/api/admin/puzzles/<id>/verdict?size=5&mode=standard` (anonymous) → 401. With a Clerk admin cookie → 200 and a verdict row visible in LocalStack DynamoDB.

**Files touched**

- `backend/internal/repository/puzzle.go` (update — schema change + 4 new methods)
- `backend/internal/repository/puzzle_test.go` (update — verdict round-trip + summary tests)
- `backend/internal/handler/verdict.go` (new)
- `backend/internal/handler/verdict_test.go` (new)
- `backend/internal/worker/generator.go` (update — drop legacy verdict write)
- `backend/internal/worker/generator_test.go` (update — drop legacy assertion)
- `backend/cmd/api/main.go` (update — register the new admin route)
- `backend/cmd/genfixtures/main.go` (update — drop legacy verdict seed)
- `PROJECT_STRUCTURE.md` (update — move verdict row from Future to Implemented; add verdict.go)
- `openspec/changes/phase-7-verdict-system/tasks.md` (update — flip R-081 row to `[x]`)

**Dependencies:** Phase 6 admin auth (R-089, R-08A, R-08B, R-08C — all `[x]`).

**Commit after completion.**

---

### R-7-02: Frontend verdict surface + landing reorg + curation route + explicit Skip + close-out

- **Roadmap:** R-7-02
- **Agent:** frontend-dev (component + service work) + general-purpose (docs sweep)
- **OpenSpec:** `specs/frontend-button.md` (FB-01 through FB-10), `specs/glossary-terms.md` (GT-01 through GT-06)

**Work**

- Create `frontend/src/services/verdictService.ts`:
  - `submitVerdict(args: SubmitVerdictArgs): Promise<void>` per FB-04.
  - Reads `import.meta.env.VITE_GIT_SHA` for `clientVersion`, defaults to `"dev"`.
  - Catches `ApiError` 401 / 403 and resolves successfully (FB-05).
  - All other errors propagate.
- Create `frontend/src/services/verdictService.test.ts`:
  - Stubs `fetch`; asserts URL, query params, method (`PUT`), `Content-Type: application/json`, body shape (FB-04).
  - 401 / 403 → resolves silently (FB-05).
  - 500 → throws `ApiError`.
- Create `frontend/src/components/game/VerdictSurface.tsx`:
  - Props per FB-02 + FB-03: `variant: 'completion' | 'skip'`, `outcome: 'solved' | 'skipped'`, `puzzleId`, `gridSize`, `mode`, `playTimeMs`, optional `onDismiss` (skip-Cancel callback) and `onAfterVerdict` (post-action navigation callback).
  - **Variant behavior — diverged button sets, not just visual de-emphasis (post-second-grill):**
    - `completion`: two buttons — "Good puzzle" (up) and "Bad puzzle" (down). Prominent layout. After click, runs the verdict service; the parent overlay's existing Play Again / Home navigation buttons remain functional independently. Silence on the verdict surface = "no opinion" — admin can navigate forward without voting.
    - `skip`: three buttons — "Cancel" (Ghost variant, dismiss; calls `onDismiss`), "I hate this" (status PUT + verdict POST in parallel), "Just skip" (status PUT only). De-emphasized chrome: smaller padding, quieter prompt copy, secondary-button styling on the action buttons. After "I hate this" or "Just skip", calls `onAfterVerdict`.
  - State machine: `idle` → `submitting` → `done` | `error` (FB-06). Applies to both variants; disable all buttons (including Cancel) during `submitting`.
  - Submitted-set `sessionStorage` cache (FB-07): on successful verdict submission OR successful skip, mark `puzzleId` in `reign:verdict:submitted` so re-mount returns null.
  - Error state: "Couldn't save." + Retry button. Retry re-invokes the same action that just failed.
  - Done state: brief "Thanks — recorded." (completion) or just navigates forward (skip).
- Create `frontend/src/components/game/VerdictSurface.test.tsx`:
  - Idle render with `variant="completion"`: "Good puzzle" + "Bad puzzle" buttons present; no Cancel.
  - Idle render with `variant="skip"`: Cancel + "I hate this" + "Just skip" buttons present; no Good/Bad.
  - Click "Good puzzle" → `submitVerdict({ value: 'up', outcome: 'solved' })`. Click "Bad puzzle" → `value: 'down', outcome: 'solved'`.
  - Click Cancel → `onDismiss()` called, no service calls.
  - Click "I hate this" → both `updatePuzzleStatus(..., 'skipped')` AND `submitVerdict({ value: 'down', outcome: 'skipped' })` called (parallel — assert both called before resolution).
  - Click "Just skip" → only `updatePuzzleStatus(..., 'skipped')` called; no `submitVerdict` call.
  - Submitting state disables every button in the row (FB-06).
  - Success on completion → `done` state renders. Success on skip → `onAfterVerdict()` called, no `done` state needed.
  - 500 → `error` state with Retry; Retry re-invokes the same action.
  - sessionStorage cache: pre-seed key with puzzleId → component returns null (both variants).
  - Variant rendering: chrome class differs (prominent vs de-emphasized) per FB-02.
- Update `frontend/src/pages/GamePage.tsx`:
  - Read `useUser()` from `@clerk/react` inside `GameBoard`.
  - Compute `isAdmin = getClerkUserRole(user?.publicMetadata) === 'admin'` using existing helper from `frontend/src/components/auth/role.ts`.
  - Render `<VerdictSurface variant="completion" outcome="solved" ...>` inside the completion overlay JSX (after Play Again / Home), conditional on `isAdmin && ready && showCompletion` (FB-01, FB-02 §1).
  - **Add explicit Skip button in the bottom action row** (FB-11) using the **Ghost** button variant, alongside Undo / Redo / Reset. Visible only when `isAdmin`. Click opens a state flag `showSkipModal` that mounts `<VerdictSurface variant="skip" outcome="skipped" onDismiss={() => setShowSkipModal(false)} onAfterVerdict={() => navigate('/curation')} ...>` (or `/` if curation route is not yet the landing entry — wire to whichever is in this slice).
  - The Skip button click does NOT directly call `updatePuzzleStatus` — that happens inside the verdict surface on "I hate this" or "Just skip".
  - `playTimeMs` prop: `completionTime * 1000` for the completion path, `timer.elapsed * 1000` for the skip path. (Note: `useTimer.elapsed` is in seconds.)
- Update `frontend/src/pages/GamePage.test.tsx`:
  - Three Clerk hook stubs (signedOut / role=user / role=admin); only admin stub finds verdict buttons in DOM (FB-01).
  - Admin completion path → buttons visible, click → service called with `outcome: 'solved'`; rendered with `variant="completion"` (prominent layout class in DOM) (FB-02, FB-08).
  - Admin skip path → buttons visible after status PUT, click → service called with `outcome: 'skipped'`; rendered with `variant="skip"` (de-emphasized layout class in DOM, prominent class absent) (FB-02).
  - Submission failure does not block the Play Again / Home buttons (FB-09).
- **Reorganize the landing page (`frontend/src/pages/LandingPage.tsx`).** Replace the current PuzzleSelector-as-landing pattern with three top-level Card-pattern tiles (BRAND_GUIDELINES §5.5):
  - **Daily** — disabled (reduced opacity, `pointer-events: none`), supporting line "Coming soon."
  - **Packs** — disabled, supporting line "Coming soon."
  - **Curation** — enabled and rendered ONLY when `getClerkUserRole(user.publicMetadata) === 'admin'` (FB-01 / FB-10 visibility model). Anonymous and User-role players see two disabled tiles and no Curation tile.
  - Tiles stack vertically on mobile, three across at `md`+ per BRAND_GUIDELINES §4.2 breakpoints.
  - "Resume" / "New puzzle" branching for in-progress play state moves *into* the curation route (since it's the only flow that produces in-progress state this phase).
- **Add a `/curation` route** in `frontend/src/App.tsx` mounted under `<ProtectedAdminRoute>` (same auth guard as `/admin`), pointing at a new `frontend/src/pages/CurationPage.tsx`:
  - The page reuses the existing `<PuzzleSelector>` component (button per enabled pool) and the existing has-progress / fresh state branching from the old LandingPage.
  - A **Settings** button at the top of the picker links to `/admin` via `useNavigate('/admin')`. Ghost variant; small.
  - "Play" launches `/play?new=true&size=N&mode=M` with the `flow=curation` query param so GamePage knows to show the verdict surface + Skip button (Phase-7 scope keeps this as a query param; R-7-03 may move it into IndexedDB).
- Create `frontend/src/pages/CurationPage.test.tsx`:
  - Renders the picker for admin role; `useUser()` mocked to admin.
  - Settings button navigates to `/admin`.
  - Selecting a pool navigates to `/play?new=true&size=...&mode=...&flow=curation`.
- Update `frontend/src/pages/LandingPage.test.tsx`:
  - Three Clerk states (signedOut, role=user, role=admin); only admin sees the Curation tile.
  - Daily and Packs tiles render disabled (assert `aria-disabled="true"` or equivalent) for all three roles.
  - Click on disabled Daily/Packs tile → no navigation.
- Update `GLOSSARY.md`:
  - Add `Verdict`, `Verdict Summary`, `Verdict Surface` in the Puzzle Lifecycle section per GT-01 / GT-02 / GT-04.
  - Add `Rater` in the Users & Access section per GT-03.
  - No changes to existing entries.
- Update `ROADMAP.md`:
  - Flip the R-7-02 checkbox from `[ ]` to `[x]` in the Phase 7 block. (R-081's row was flipped during its own PR and stays `[x]`; R-7-03 stays `[ ]` for the next slice.)
  - Sanity grep: no leftover "Phase 6: Verdict" references (Phase 6 is now Admin Auth).
- Update `PROJECT_STRUCTURE.md`:
  - Add `frontend/src/components/game/VerdictSurface.tsx` to the frontend tree.
  - Add `frontend/src/services/verdictService.ts` to the services tree.
- Term consistency sweep (GT-06): grep new code for synonyms (`Judgment`, `Rating`, `Vote`, `Score`) — none should appear unless explicitly defined.
- E2E smoke (manual, not Playwright this phase): on a deployed environment with a Clerk admin session, complete a puzzle → verdict surface visible → click Good → row appears in DynamoDB. Sign out → complete a puzzle → no verdict surface.
- Flip the R-7-02 row in this `tasks.md` from `[ ]` to `[x]`.

**Gate**

- `npx tsc -b` passes.
- `npm test` green (new tests + existing tests still pass).
- Grep sweep: no `Judgment` / `Rating` / `Vote` / `Score` synonyms in new code.
- Glossary contains all four new terms with the wording from `specs/glossary-terms.md`.
- Manual: `task dev:up` → sign in as admin → complete a puzzle → click "Good puzzle" → "Thanks — recorded." renders → DynamoDB has the row. Sign in as a non-admin user → complete a puzzle → no verdict UI visible.

**Files touched**

- `frontend/src/services/verdictService.ts` (new)
- `frontend/src/services/verdictService.test.ts` (new)
- `frontend/src/components/game/VerdictSurface.tsx` (new)
- `frontend/src/components/game/VerdictSurface.test.tsx` (new)
- `frontend/src/pages/GamePage.tsx` (update — render VerdictSurface conditional on admin role + add explicit Skip button)
- `frontend/src/pages/GamePage.test.tsx` (update — three-state visibility coverage + Skip button + skip modal flow)
- `frontend/src/pages/LandingPage.tsx` (update — replace PuzzleSelector landing with three top-level tiles)
- `frontend/src/pages/LandingPage.test.tsx` (update — tile visibility + disabled-tile behaviour across three Clerk states)
- `frontend/src/pages/CurationPage.tsx` (new — picker + settings link, behind ProtectedAdminRoute)
- `frontend/src/pages/CurationPage.test.tsx` (new — settings link + pool selection navigation)
- `frontend/src/App.tsx` (update — register `/curation` route)
- `GLOSSARY.md` (update — add four terms)
- `ROADMAP.md` (update — flip R-7-02 to `[x]`; R-081 already `[x]` from its own PR)
- `PROJECT_STRUCTURE.md` (update — add new frontend files)
- `openspec/changes/phase-7-verdict-system/tasks.md` (update — flip R-7-02 row to `[x]`)

**Dependencies:** R-081 (the API the frontend posts to must be live).

**Commit after completion.**

---

### R-7-03: Per-flow IndexedDB storage (no implicit skips)

- **Roadmap:** R-7-03
- **Agent:** frontend-dev
- **OpenSpec:** `specs/storage.md` (ST-01 through ST-12)

**Context — R-7-02's spec drift.** R-7-02's task spec (line 167 above) said the curation flow would pass a `flow=curation` query param to `/play`. That wiring shipped only partially: the backend / IDB plumbing was deferred and the URL parameter was never added. R-7-03 is therefore the slice that *actually* implements the URL contract envisioned in R-7-02. No history rewrite — call out in the PR body.

**Locked decisions** (from `design-grill-summary.md`):

- D1 — composite-string key `'<flowType>:<flowId>'` on the existing `gameState` store (not compound IDB key, not store-per-flowType).
- D2/D3 — bump `DB_VERSION` to 2 + clear the `gameState` store on upgrade. Graceful drop, no row-level migration.
- D4 — URL specifies the flow + flow-identifying params; `loadState` decides resume vs. fetch. The `?new=true` URL contract is removed entirely.
- D6 — typed `FlowType = 'curation' | 'daily' | 'pack'` union (`'daily'` and `'pack'` pre-declared even though only `'curation'` is wired).
- D9 — clear-on-solve.

**Work**

- Update `frontend/src/storage/types.ts`:
  - Add `export type FlowType = 'curation' | 'daily' | 'pack'` (ST-02).
  - Replace `id: 'current'` literal on `GameState` with `id: string`. Add `flowType: FlowType` and `flowId: string` fields (ST-03).
- Update `frontend/src/storage/db.ts`:
  - Bump `DB_VERSION` from `1` to `2`.
  - In `onupgradeneeded`, when `event.oldVersion < 2`, clear the `gameState` store (preserve `completions`) (ST-04).
  - Add an internal helper `idFor(flowType, flowId): string` returning `${flowType}:${flowId}` — the only place the `':'` separator appears in production code (ST-09).
- Update `frontend/src/storage/utils.ts`:
  - `createFreshGameState(flowType: FlowType, flowId: string, puzzle: PuzzleData): GameState` — construct `id = "${flowType}:${flowId}"`, set `flowType` / `flowId` on the row body. Update tests in `storage/utils.test.ts` accordingly.
- Update `frontend/src/hooks/useGameStorage.ts` (ST-09):
  - `saveState(state: GameState)` unchanged signature — `state.id` already encodes the slot.
  - `loadState(flowType: FlowType, flowId: string): Promise<GameState | null>` — `store.get(idFor(flowType, flowId))`.
  - `clearState(flowType: FlowType, flowId: string): Promise<void>` — `store.delete(idFor(flowType, flowId))`.
- Add `parseFlowType(raw: string | null): FlowType | null` helper next to the type (ST-11). Returns null for unknown / missing values.
- Update `frontend/src/pages/GamePage.tsx`:
  - Drop the `searchParams.get('new') === 'true'` branch entirely (ST-08).
  - Read `flow` / `size` / `mode` (and future `date` / `id`) from URL. Use `parseFlowType` for the `flow` value; null → redirect to `/`.
  - Build `flowId` per flow type. Phase 7 only wires curation → `flowId = "${size}x${size}-${mode}"`. Document the convention in a single helper (`buildCurationFlowId(size, mode)`) so daily / pack producers extend the same module later.
  - Effect logic (ST-06): call `loadState(flowType, flowId)`. Hit + `status !== 'solved'` → resume. Else → `fetchNextPuzzle(size, mode)`, then `saveState(createFreshGameState(flowType, flowId, puzzle))`.
  - Completion handler (ST-07): after `addCompletion`, call `clearState(flowType, flowId)`.
  - Update the doc comment on `GamePage` (currently mentions `?new=true`).
  - "Play Again" button: navigate to the same `/play?flow=curation&size=N&mode=M` URL — the slot was just cleared by ST-07, so the next render fetches fresh.
- Update `frontend/src/pages/CurationPage.tsx`:
  - Drop `new=true` from the navigation. Add `flow=curation`. Final URL: `/play?flow=curation&size=${size}&mode=${mode}`.
- **Sweep `?new=true` from the codebase** (ST-08):
  - `grep -rn "new=true\\|searchParams.*['\\\"]new['\\\"]" frontend/src` — every hit must be removed or converted to a negative-assertion test. Includes `CurationPage.tsx`, any leftover landing-page references, and the `GamePage` test file.
- Update `frontend/src/pages/GamePage.test.tsx`:
  - Drop the `new=true` test cases; convert to per-flow URL `/play?flow=curation&size=...&mode=...`.
  - Add: mount with no slot → fetch path runs and saves. Mount with pre-seeded matching slot (status `in-progress`) → resume path renders persisted cells. Mount with pre-seeded slot but `status: 'solved'` → fetch path runs (defensive, ST-06). Mount with `?flow=junk` → redirect to `/` (ST-11).
  - Solve a puzzle → assert `clearState` was called for the right `(flowType, flowId)`.
  - Switching pools (mount A, then mount B with different size+mode) → A's slot is preserved on disk after B's fetch.
- Add `frontend/src/storage/db.test.ts` (or expand existing test file if present):
  - Round-trip per-slot writes: two slots, two rows.
  - Upsert: three writes to same slot, one row, last wins (ST-05).
  - V1 → V2 upgrade: seed a v1 `{id: 'current', ...}` row, bump version, verify `gameState` is empty and `completions` is preserved (ST-04).
- Add `frontend/src/storage/types.test.ts` (or fold into existing utils test):
  - `parseFlowType('curation')` → `'curation'`. `parseFlowType('daily')` → `'daily'`. `parseFlowType('pack')` → `'pack'`. `parseFlowType('curations')` → `null`. `parseFlowType(null)` → `null`.
- Update `frontend/src/pages/CurationPage.test.tsx`:
  - Selecting a pool navigates to `/play?flow=curation&size=...&mode=...` (no `new=true`).
- Update `GLOSSARY.md`:
  - Already done in glossary commit on this branch — verify `Flow` and `Flow Slot` entries are present and the wording matches the storage spec.
- Update `PROJECT_STRUCTURE.md`:
  - No new files (everything edits existing files), but add a one-line note in the storage section describing the per-flow shape if the existing description claims a single-slot pattern.
- Update `ROADMAP.md`:
  - Flip the R-7-03 checkbox from `[ ]` to `[x]` in the Phase 7 block.
- Flip the R-7-03 row in this `tasks.md` from `[ ]` to `[x]`.

**Gate**

- `npx tsc -b` passes.
- `npm test` green (new tests + existing tests still pass).
- `npm run build` succeeds.
- Grep sweep clean (ST-08): `grep -rn "new=true" frontend/src` returns zero matches in production code.
- Grep sweep clean (ST-09): `grep -rn "':'\\|\":\"" frontend/src/storage` shows the separator only in `idFor`.
- Manual: `task dev:up` → sign in as admin → Curation tile → 5×5 standard → make a few moves. Reload page → puzzle resumes. Navigate back to Curation → 7×7 standard → fresh puzzle; the 5×5 slot is preserved (verifiable via DevTools → Application → IndexedDB → reign-game → gameState; two rows). Solve the 5×5 → that slot is removed.
- Manual upgrade test: deploy the slice to dev. An admin who had a v1 in-progress puzzle pre-deploy first navigates after deploy → no resume, fresh fetch (ST-04 wipe behavior verified by user feedback / direct DevTools inspection).

**Files touched**

- `frontend/src/storage/types.ts` (update — `FlowType` union, `GameState` shape, `parseFlowType` helper)
- `frontend/src/storage/db.ts` (update — `DB_VERSION=2`, upgrade clear, `idFor` helper)
- `frontend/src/storage/utils.ts` (update — `createFreshGameState` takes `(flowType, flowId, puzzle)`)
- `frontend/src/storage/utils.test.ts` (update — new signature)
- `frontend/src/storage/db.test.ts` (new — round-trip + upsert + v1→v2 upgrade)
- `frontend/src/storage/types.test.ts` (new or folded — `parseFlowType` cases)
- `frontend/src/hooks/useGameStorage.ts` (update — `loadState` / `clearState` take `(flowType, flowId)`)
- `frontend/src/pages/GamePage.tsx` (update — URL parsing, resume vs. fetch branch, clear-on-solve, drop `new=true`)
- `frontend/src/pages/GamePage.test.tsx` (update — per-flow URL cases, resume + fetch + solved-defensive + invalid-flow + clear-on-solve + pool-switch isolation)
- `frontend/src/pages/CurationPage.tsx` (update — emit `flow=curation`, drop `new=true`)
- `frontend/src/pages/CurationPage.test.tsx` (update — assert new URL shape)
- `GLOSSARY.md` (already updated on this branch — verify only)
- `PROJECT_STRUCTURE.md` (update if storage description mentions single-slot)
- `ROADMAP.md` (update — flip R-7-03 checkbox)
- `openspec/changes/phase-7-verdict-system/tasks.md` (update — flip R-7-03 row to `[x]`)

**Dependencies:** R-7-02 (the curation route + explicit Skip button must exist; this slice only changes how in-progress state is keyed and how `/play` URLs are shaped).

**Commit after completion. Then archive this OpenSpec change via `/opsx:archive`.**

## Verification Checklist (Phase Close)

Before promoting this epic to main:

- [ ] `PUT /api/admin/puzzles/{id}/verdict` returns 401 for anonymous, 403 for user-role, 200 for admin. Integration test proves it.
- [ ] Verdict rows persist with PK = `"VERDICT#{size}#{mode}#{puzzleId}"`, SK = `"{raterRole}#{raterId}"`. Idempotent overwrite verified.
- [ ] `verdictSummary` projection on `PuzzleRecord` matches the row family after every write.
- [ ] Legacy `verdict: "none"` field removed from `PuzzleRecord`; legacy rows tolerate the schema change without backfill.
- [ ] Frontend verdict surface renders only for admin role; anonymous and User-role play-throughs see no verdict UI.
- [ ] `playTimeMs` captured from `useTimer.elapsed` at action time; row carries the value verbatim.
- [ ] Skip remains the existing `PUT /api/puzzles/{id}/status` flow; the verdict endpoint rejects `value="skip"`.
- [ ] `GLOSSARY.md` contains `Verdict`, `Verdict Summary`, `Rater`, `Verdict Surface` with the wording from `specs/glossary-terms.md`.
- [ ] `ROADMAP.md` Phase 7 block: R-7-02 flipped to `[x]` (R-081 already `[x]`; R-7-03 stays `[ ]` for the next slice).
- [ ] `PROJECT_STRUCTURE.md`: API endpoints table moves verdict from Future to Implemented; new files listed.
- [ ] `tasks.md` status table all `[x]`.
- [ ] No new KIs opened by this phase. (The summary-projection lag is documented in `design.md` Risks but not promoted to a KI — single-admin scale makes it invisible.)
- [ ] Follow 4-axis review-local + security-review before epic→main merge (per CLAUDE.md lesson 13).
