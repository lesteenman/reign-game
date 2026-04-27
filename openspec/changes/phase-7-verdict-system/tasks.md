# Phase 7 Tasks: Verdict System

## Slice Dependency Graph

```
R-081 (Backend handler + repository + schema migration)
    │
    └── R-082 (Frontend verdict surface + glossary + docs sweep + slice close-out)
```

R-081 ships first. R-082 depends on R-081's API being live (the frontend posts to it) and folds the docs / glossary / ROADMAP sweep into the same PR — splitting that out as a third slice would be wasted ceremony for two-slice work. Per CLAUDE.md lesson 18 (grep for ID collisions): R-08D is reserved for the Phase 6 custom-domain follow-up; R-08E and R-08F are next free in the R-08x family but neither is needed.

## Status

All Phase 7 slices are `[ ]` until completed. Per CLAUDE.md lesson 17, each slice's PR must flip its row to `[x]` as a required artifact — no post-hoc sweeps.

| ID    | Slice                                                          | Layer | Status |
|-------|----------------------------------------------------------------|-------|--------|
| R-081 | Backend verdict handler + repository + schema migration        | 1     | [ ]    |
| R-082 | Frontend verdict surface + glossary + docs sweep + close-out   | 2     | [ ]    |

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

### R-082: Frontend verdict surface + glossary + docs sweep + slice close-out

- **Roadmap:** R-082
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
  - Props per FB design.md §5, plus a `variant: 'completion' | 'skip'` prop (FB-02 post-grill resolution).
  - **Variant behavior:** `completion` renders prominent layout (current full-size buttons + "Rate this puzzle?" prompt). `skip` renders de-emphasized layout (smaller buttons, quieter prompt copy like "Quick rate before moving on?", less visual prominence). Branch is a className / size-token swap; no state-machine, label, or API-call differences between variants. Same up/down axis on both.
  - State machine: `idle` → `submitting` → `done` | `error` (FB-06).
  - Buttons labelled "Good puzzle" / "Bad puzzle" (FB-03) — same labels in both variants.
  - Submitted-set `sessionStorage` cache (FB-07): if `puzzleId` already in `reign:verdict:submitted`, return null on mount.
  - Error state: "Couldn't save your verdict." + Retry button.
  - Done state: "Thanks — recorded." text replaces buttons.
- Create `frontend/src/components/game/VerdictSurface.test.tsx`:
  - Idle render: both buttons present.
  - Click "Good puzzle" → service called with `value: 'up'`; click "Bad puzzle" → `value: 'down'` (FB-03).
  - Submitting state disables both buttons (FB-06).
  - Success → `done` state renders, no buttons (FB-06).
  - 500 → `error` state with Retry; clicking Retry re-invokes service with the previously-clicked value.
  - sessionStorage cache: pre-seed key with puzzleId → component renders null (FB-07).
  - Variant rendering: `variant="completion"` produces the prominent layout class in DOM; `variant="skip"` produces the de-emphasized layout class and not the prominent one. Same buttons, same labels, same backend call shape across both variants (FB-02 post-grill).
- Update `frontend/src/pages/GamePage.tsx`:
  - Read `useUser()` from `@clerk/react` inside `GameBoard`.
  - Compute `isAdmin = getClerkUserRole(user?.publicMetadata) === 'admin'` using existing helper from `frontend/src/components/auth/role.ts`.
  - Render `<VerdictSurface variant="completion" outcome="solved" ...>` inside the completion overlay JSX (after Play Again / Home), conditional on `isAdmin && ready && showCompletion` (FB-01, FB-02).
  - Add the post-skip transient state for admins:
    - When an admin clicks an existing skip control (or invokes the skip flow that calls `updatePuzzleStatus(..., 'skipped')`), hold the user on a `<VerdictSurface variant="skip" outcome="skipped" ...>` panel before navigating home.
    - Non-admins continue to navigate home directly — behaviour unchanged.
  - `playTimeMs` prop: `completionTime * 1000` for the completion path, `timer.elapsed * 1000` for the skip path.
- Update `frontend/src/pages/GamePage.test.tsx`:
  - Three Clerk hook stubs (signedOut / role=user / role=admin); only admin stub finds verdict buttons in DOM (FB-01).
  - Admin completion path → buttons visible, click → service called with `outcome: 'solved'`; rendered with `variant="completion"` (prominent layout class in DOM) (FB-02, FB-08).
  - Admin skip path → buttons visible after status PUT, click → service called with `outcome: 'skipped'`; rendered with `variant="skip"` (de-emphasized layout class in DOM, prominent class absent) (FB-02).
  - Submission failure does not block the Play Again / Home buttons (FB-09).
- Update `GLOSSARY.md`:
  - Add `Verdict`, `Verdict Summary`, `Verdict Surface` in the Puzzle Lifecycle section per GT-01 / GT-02 / GT-04.
  - Add `Rater` in the Users & Access section per GT-03.
  - No changes to existing entries.
- Update `ROADMAP.md`:
  - Flip R-081 and R-082 checkboxes from `[ ]` to `[x]` in the Phase 7 block. (R-081's row will already be `[x]` from its own PR; this slice flips R-082's and confirms R-081's stayed `[x]`.)
  - Sanity grep: no leftover "Phase 6: Verdict" references (Phase 6 is now Admin Auth).
- Update `PROJECT_STRUCTURE.md`:
  - Add `frontend/src/components/game/VerdictSurface.tsx` to the frontend tree.
  - Add `frontend/src/services/verdictService.ts` to the services tree.
- Term consistency sweep (GT-06): grep new code for synonyms (`Judgment`, `Rating`, `Vote`, `Score`) — none should appear unless explicitly defined.
- E2E smoke (manual, not Playwright this phase): on a deployed environment with a Clerk admin session, complete a puzzle → verdict surface visible → click Good → row appears in DynamoDB. Sign out → complete a puzzle → no verdict surface.
- Flip R-082 row in this `tasks.md` from `[ ]` to `[x]`.

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
- `frontend/src/pages/GamePage.tsx` (update — render VerdictSurface conditional on admin role)
- `frontend/src/pages/GamePage.test.tsx` (update — three-state visibility coverage)
- `GLOSSARY.md` (update — add four terms)
- `ROADMAP.md` (update — flip R-081 and R-082 to `[x]`)
- `PROJECT_STRUCTURE.md` (update — add new frontend files)
- `openspec/changes/phase-7-verdict-system/tasks.md` (update — flip R-082 row to `[x]`)

**Dependencies:** R-081 (the API the frontend posts to must be live).

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
- [ ] `ROADMAP.md` Phase 7 block: R-081 and R-082 flipped to `[x]`.
- [ ] `PROJECT_STRUCTURE.md`: API endpoints table moves verdict from Future to Implemented; new files listed.
- [ ] `tasks.md` status table all `[x]`.
- [ ] No new KIs opened by this phase. (The summary-projection lag is documented in `design.md` Risks but not promoted to a KI — single-admin scale makes it invisible.)
- [ ] Follow 4-axis review-local + security-review before epic→main merge (per CLAUDE.md lesson 13).
