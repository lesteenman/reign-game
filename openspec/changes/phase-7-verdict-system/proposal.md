# Phase 7: Verdict System

## What

Ship admin-only puzzle rating: an authenticated admin who has just played a puzzle (completion or skip) submits an upvote / downvote on a small in-game UI surface. Verdicts are stored as **per-rater records** in the existing `puzzle-pool` DynamoDB table (`PK = "VERDICT#{size}#{mode}#{puzzleId}"`, `SK = "{raterRole}#{raterId}"`). A verdict-summary projection (`{up, down, lastUpdatedAt}`) is denormalized onto `PuzzleRecord` so the admin pool view, the analysis agent (Phase 9), and the audit loop (R-083 / R-084) can read counts without a fanout query. Skip stays a status — the verdict surface is up / down only. Play-time and a small set of attempt-context fields are captured on the verdict row from day one so R-084's blind calibration test gets the signal it needs the moment it starts.

The endpoint is `PUT /api/admin/puzzles/{id}/verdict`, mounted inside the Phase 6 admin route group so it inherits `RequireAuth` + `RequireAdmin` by construction. Anonymous and User-role players never see verdict UI and never call the endpoint.

The existing unused `Verdict string` field on `PuzzleRecord` (set to `"none"` at generation time, never read) is retired as part of R-081's schema work. Legacy rows tolerate the change without backfill.

## Why

- **The audit loop is gated on this phase.** Phase 7b (R-083 dead-rule investigation, R-084 Medium / Hard blind calibration), Phase 8 (replay), and Phase 9 (analysis agent) all need a "what did the rater think of this puzzle" signal. Without verdicts, every downstream calibration-and-tuning slice runs blind.
- **The schema must be multi-rater-ready from day one.** The locked decision is "admin-only for now, public rater role later." A single-field-on-PuzzleRecord shape (which the codebase already has, vestigially) blocks the second rater role behind a destructive migration. A per-rater row family ships the right shape now, avoids future rework, and reuses the single-table convention CLAUDE.md asks for.
- **Play-time signal is cheap to capture now and expensive to retrofit.** R-084 wants per-attempt play-time across a labeled corpus. Capturing on every verdict — even when only admins vote — costs four extra fields and gives the calibration test a signal-bearing corpus the moment it lands.
- **The endpoint placement (`/api/admin/*` rather than the public `/api/puzzles/*`) is the simplest way to satisfy admin-only voting.** It picks up the Phase 6 middleware chain by construction; no new auth wiring.

## Summary of Locked Decisions

Decisions from `design-grill-summary.md`. One-line form:

1. **Voter identity:** Admin-only (locked by human). Schema is multi-rater-ready — keyed by `(puzzleId, raterId, raterRole)`.
2. **Route:** `PUT /api/admin/puzzles/{id}/verdict` — mounted inside the Phase 6 `/api/admin` group, inherits `RequireAuth` + `RequireAdmin`. Note the deviation from the ROADMAP wording ("PUT /puzzles/:id/verdict"): admin-only voting drives the path under `/api/admin/*`.
3. **Storage shape:** Per-rater rows in the existing `puzzle-pool` table, with a `verdictSummary` projection on `PuzzleRecord` for zero-fanout reads. New row family `PK = VERDICT#{size}#{mode}#{puzzleId}`, `SK = {raterRole}#{raterId}`.
4. **Verdict values:** `up` and `down` only. Skip stays a status (existing `PUT /api/puzzles/{id}/status` flow). Status and verdict are orthogonal concepts.
5. **Idempotency:** PUT semantics — same `(puzzleId, raterId)` overwrites the row. Last-write-wins on the summary projection.
6. **Play-time capture:** Yes, from day one. Verdict row stores `playTimeMs`, `outcome` (`solved` / `skipped`), `clientVersion`, `submittedAt`.
7. **Frontend gating:** Verdict UI renders only for `publicMetadata.role === 'admin'`. Anonymous and User-role players see nothing — no buttons, no API calls, no path leakage. Same role helper as `ProtectedAdminRoute`.
8. **Existing `Verdict string` field on `PuzzleRecord`:** Removed in this slice. Field is unused at read time today; legacy rows tolerate the removal (DynamoDB ignores unknown attributes on unmarshal).
9. **Glossary:** Add `Verdict`, `Verdict Summary`, `Rater`, `Verdict Surface`. Document `Verdict` distinct from `Status`.

## Acceptance Criteria

A single cycle of Phase 7 is done when:

- **AC-1. Endpoint shipped.** `PUT /api/admin/puzzles/{id}/verdict` accepts `{ value: "up" | "down", playTimeMs: number, outcome: "solved" | "skipped", clientVersion: string }` and persists a verdict row with the body fields plus `submittedAt` and `raterId` (from the Clerk session). Returns `200` on success, `400` on a malformed body, `404` if the puzzle doesn't exist, `401` for anonymous, `403` for non-admin.
- **AC-2. Verdict row family in DynamoDB.** Rows write to `PK = "VERDICT#{size}#{mode}#{puzzleId}"`, `SK = "{raterRole}#{raterId}"`. Re-submission overwrites — at most one row per (puzzle, rater) pair.
- **AC-3. Summary projection on PuzzleRecord.** Every successful verdict write is followed by a recompute of `verdictSummary` on the puzzle row: `{up: int, down: int, lastUpdatedAt: ISO 8601}`. The summary is read directly by handlers; no fanout query on the read path.
- **AC-4. Skip is not a verdict.** The `PUT /api/admin/puzzles/{id}/verdict` endpoint rejects any value other than `up` or `down` with 400. Skipping a puzzle remains a status flow (`PUT /api/puzzles/{id}/status` with `"skipped"`); the two flows do not coerce one another.
- **AC-5. Frontend verdict surface — admin only.** The verdict buttons render in two places inside the gameplay flow: (a) the completion overlay, (b) the post-skip transient state. Both surfaces are wrapped in a check for `getClerkUserRole(user.publicMetadata) === 'admin'`. Anonymous and User-role play-throughs render no verdict UI.
- **AC-6. Play-time captured.** The frontend hands `useTimer.elapsed` (in milliseconds) to the verdict request body. The generated verdict row carries the value verbatim.
- **AC-7. Backend auth tests prove gating.** New integration tests in the handler package iterate the existing 3-state `adminAuthMatrix` (anonymous → 401, user → 403, admin → 200) on the verdict endpoint, plus per-value validation tests (invalid `value` → 400, missing puzzle → 404).
- **AC-8. Existing `Verdict string` field retired.** The field is removed from `PuzzleRecord`; references in `cmd/genfixtures/main.go` and `worker/generator.go` are removed; tests no longer assert on it. A grep sweep confirms no remaining read-side references.
- **AC-9. Frontend tests.** New Vitest specs cover (a) admin role → verdict surface visible, (b) user role → verdict surface absent, (c) anonymous → verdict surface absent, (d) submit success → "Thanks — recorded" confirmation, (e) submit failure (5xx) → retry-able error message, no flow-block.
- **AC-10. Glossary updated.** `Verdict`, `Verdict Summary`, `Rater`, `Verdict Surface` defined. `Status` and `Verdict` are explicitly contrasted.
- **AC-11. ROADMAP updated.** R-081 and R-082 flipped to `[x]` in their respective PRs. KI catalog unchanged (no new KIs from this phase).
- **AC-12. PROJECT_STRUCTURE.md updated.** API endpoints table moves the verdict row from "Future" to "Implemented" with auth=Admin. New backend handler file and frontend component listed.

## Scope

### In Scope

- New backend handler `PUT /api/admin/puzzles/{id}/verdict` mounted inside the Phase 6 admin route group.
- New repository methods on `PuzzleRepository`: `PutVerdict`, `GetVerdictSummary` (used by the admin pool view in a follow-up if needed; the summary on `PuzzleRecord` covers the immediate read path), and `RecomputeVerdictSummary` (called from the handler after a successful row write).
- Schema changes on `PuzzleRecord`: remove `Verdict string`, add `VerdictSummary VerdictSummary` (typed struct).
- Frontend verdict surface inside `GamePage.tsx` — two buttons on the completion overlay and on the post-skip transient state.
- Frontend service `submitVerdict(puzzleId, value, playTimeMs, outcome)` in a new `frontend/src/services/verdictService.ts`.
- Backend tests: handler-level auth matrix, request validation, idempotency proof, summary recompute proof.
- Frontend tests: role-gated visibility, submit success / failure handling.
- Glossary additions.
- ROADMAP and PROJECT_STRUCTURE.md updates.
- Removal of legacy `Verdict string` field with a grep sweep.

### Out of Scope (Deferred)

- **Public-rater role.** Schema is shaped for it; no UI, no rate limits, no role assignment in this phase.
- **Verdict comments / written reasons.** Add when needed; row family can absorb an optional `reason string` without migration.
- **"Vote on a previously-played puzzle" UI.** Surfaces in Phase 8 (R-086 replay UI). The backend endpoint accepts the verdict regardless; only the UI surface is missing.
- **Audit-trail history (append-only verdict log).** Today's PUT-overwrite is sufficient. Add if Phase 9 wants change-over-time signal.
- **GSI on `(verdictBucket, createdAt)` for "show me all downvoted puzzles" queries.** Phase 9's analysis agent decides if it's worth the cost based on observed scan latency.
- **Conditional-write race protection on summary updates.** Single-admin scale doesn't see lag; add if multi-rater + visible lag becomes a real problem.
- **Backfill of `verdictSummary` for legacy puzzle rows.** Not needed — zero-value summary is correct.

### Non-Goals

- Replacing the existing `PUT /api/puzzles/{id}/status` flow. Status and verdict are orthogonal; both flows remain.
- Building a verdict-aware admin curation UI. Phase 8 / 9 own the curation surface.
- Per-puzzle rate limiting on verdict submission. Single-admin use does not need it.
- Verdict submission from anonymous or User-role users. Locked: admin-only.

## Dependencies

- **Phase 6 admin auth shipped (`R-089`, `R-08A`, `R-08B`, `R-08C`).** Confirmed `[x]` in ROADMAP. The verdict endpoint inherits the middleware chain.
- **`puzzle-pool` DynamoDB table.** No Terraform change required — the table accepts arbitrary `PK / SK` row families.
- **Clerk publicMetadata.role.** Already wired via Phase 6.
- **`useTimer.elapsed` accessible at completion / skip time.** Confirmed in `frontend/src/pages/GamePage.tsx` and `frontend/src/hooks/useTimer.ts`.
- **No new npm packages.** Frontend uses the existing `apiPut` helper.
- **No new Go packages.** Backend reuses chi, Clerk SDK, and the AWS SDK already on the dependency tree.

## Risks

| Risk | Mitigation |
|------|------------|
| Verdict-summary projection lags the row family by milliseconds under concurrent admin votes | Single-admin scale makes the lag invisible. Document the source-of-truth contract: the row family is canonical, the summary is a cached projection. Phase 9 recomputes from rows when correctness matters. |
| An admin re-votes after their role is revoked mid-session | Backend middleware fails closed (403 on the next request). Frontend cache of "I voted on this puzzle" is best-effort UX, not a security boundary. |
| Removing the legacy `Verdict string` field breaks an unseen consumer | Grep sweep is mandatory in R-081's PR. There are no external consumers (no Lambda triggers, no analytics export) — the change is self-contained inside the Go module + one frontend test fixture. |
| The `verdictSummary` summary diverges from the row family after a partial-failure write | Acceptable for single-admin use. A reconciliation cron is documented in Phase 9 deferred items. The row family remains canonical. |
| Verdict captured on a puzzle that was never marked `served` (synthetic / replayed puzzle) | The endpoint validates that the puzzle exists in DynamoDB by reading the `PuzzleRecord`. If absent → 404. The UI today only surfaces verdict buttons after a play attempt, which means the puzzle was served — but defending the endpoint is cheap. |
| The verdict surface confuses a non-admin who somehow sees it (cosmetic-gating bypass) | Backend middleware is the source of truth. A user who reaches the endpoint via direct API call gets 403 regardless of UI state. AC-7 proves it. |
| Schema migration leaves orphan `verdict: "none"` attributes on legacy rows | Acceptable. DynamoDB tolerates unknown attributes; the Go struct ignores them on unmarshal. No backfill required; future writes don't set the legacy field. |

## Migration

- **Schema migration:** `PuzzleRecord` loses the `Verdict string` field and gains `VerdictSummary VerdictSummary`. The change ships with R-081's repository update.
- **Data migration:** None. Legacy rows keep their `verdict: "none"` attribute (ignored on unmarshal). New writes do not set it. Verdict-summary on legacy rows is zero-valued on first read, populated forward by admin votes after the slice ships.
- **Rollback:** If the slice produces a regression, revert the handler + repository changes and the PuzzleRecord schema. The verdict row family persists in DynamoDB but is unreferenced — no read or write code touches it. Cleanup is a one-line scan-and-delete script if desired.
- **Forward compat:** When the public-rater role lands, the schema is already correct. Adding the new role is (a) an authn change (Clerk role), (b) a public-write rate limit on the endpoint, (c) a new UI surface for non-admin players. No schema rework.
