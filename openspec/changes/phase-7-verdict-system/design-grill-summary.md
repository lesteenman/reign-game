# Design Grill Summary: Phase 7 — Verdict System

This walk was driven autonomously by the design-flow agent while the human was offline. Where a branch produced an unambiguous answer, the answer is locked. Where the choice would commit something hard to reverse without explicit human sign-off, the branch is flagged **OPEN — needs human override** with the conservative default chosen.

## Final Design

Admin-only puzzle rating: an authenticated admin who has just completed or skipped a played puzzle submits an upvote / downvote on the rendered verdict surface inside the gameplay flow. Verdicts are **per-rater records** (one row per `(puzzleId, raterId)` pair) stored alongside puzzles and configs in the existing `puzzle-pool` table; a verdict-summary projection is denormalized onto the `PuzzleRecord` so the admin pool view, the analysis agent (Phase 9), and the audit loop (R-083 / R-084) can read counts without a fanout query. Skip stays a **status** (the existing `PUT /api/puzzles/{id}/status` flow); the verdict surface is up/down only. Play-time and a small amount of attempt context are captured at verdict-submission time so R-084's blind calibration test gets the signal it needs from day one. The route lives under `/api/admin/puzzles/{id}/verdict` so it inherits the existing `RequireAuth` + `RequireAdmin` middleware chain wired in Phase 6 (R-08A). Frontend buttons render only for users with `publicMetadata.role === 'admin'`; anonymous and User-role players see nothing — no buttons, no API calls.

## Decisions

### Voter identity model — locked by the human (B for now, extensible later)

- **Locked: admin-only voting.** The human explicitly said "B for now, I might add an extra role for people who can also do so later, but not yet." Only signed-in admins submit verdicts.
- **Implication 1 — route placement.** Mount the endpoint under `/api/admin/*` so it picks up `RequireAuth` + `RequireAdmin` from the Phase 6 admin group by construction (BM-05). The ROADMAP entry text "PUT /puzzles/:id/verdict" is overridden — the actual path is `/api/admin/puzzles/{id}/verdict`. The deviation is called out in `proposal.md`.
- **Implication 2 — schema must be multi-rater-ready.** The existing `Verdict string` field on `PuzzleRecord` (an artefact of an earlier design pass — see `backend/internal/repository/puzzle.go` line 64, default `"none"` set by `worker/generator.go` line 145) cannot become the canonical store, because it pre-commits to a single-voter model. Storing the canonical record per-rater means the schema is already correctly shaped when the second rater role lands — no destructive migration.

### Storage shape — per-rater records in `puzzle-pool` table, with a verdict-summary projection on PuzzleRecord

This was the single most consequential decision in the slice. Five candidate shapes were stress-tested. The synthesized result:

**Locked: per-rater verdict records, single-table, denormalized summary on PuzzleRecord.**

Schema details (full surface is spelled out in `specs/repository.md`):

- New row family: `PK = "VERDICT#{size}#{mode}#{puzzleId}"`, `SK = "{raterRole}#{raterId}"`. One row per rater per puzzle.
- Idempotent upsert: re-submission by the same `(puzzleId, raterId)` pair overwrites the row, with `submittedAt` updated. PUT semantics — same admin re-clicks downvote after upvoting → row's `value` changes from `up` to `down`, `submittedAt` advances. No rows accumulate.
- Verdict-summary projection on `PuzzleRecord`: a small JSON-marshalled object (`verdictSummary`) carries `{up, down, lastUpdatedAt}` per puzzle, recomputed on every verdict write. The existing `Verdict string` field is retired in this slice (renamed away in the same migration; the field is essentially unused at read time today, only set to `"none"` at generator-write time).
- Read path for admin pool / analysis agent: read `verdictSummary` from `PuzzleRecord`. No fanout query needed for the common case.
- Read path for audit ("show me all puzzles where verdict==down"): scan-with-filter on the puzzle partition, filtered by `verdictSummary.down > 0`, OR (later) a GSI on `(verdictBucket, createdAt)` if the scan becomes slow. The analysis agent (Phase 9, R-087) decides whether to add the GSI based on observed query latency. Out of scope for this phase.

#### Why this shape (the five-approach synthesis)

| Approach | Multi-rater ready? | Read cost (admin pool) | Write cost | Migration cost from current state | Verdict-vs-status conflict |
|----------|--------------------|------------------------|------------|------------------------------------|----------------------------|
| **A. Single field on PuzzleRecord (current)** | No — blocks future raters | 0 (in record) | 1 UpdateItem | None (already there) | High — overlaps with status `skipped` |
| **B. Per-rater records, no summary** | Yes | Fanout per puzzle (N puzzles → N partition queries) | 1 PutItem | Drop the Verdict field; new row family | Resolved by removing skip from verdict |
| **C. Per-rater records + summary on PuzzleRecord** ← **CHOSEN** | Yes | 0 (in record) | 1 PutItem + 1 UpdateItem (summary) | Drop the Verdict field; new row family + summary projection | Resolved by removing skip from verdict |
| **D. Separate `verdict-pool` table** | Yes | 0 if denormalized; fanout if not | 1 PutItem | New table in Terraform; new repository file | Same as C |
| **E. Append-only verdict log + materialized summary** | Yes — audit-perfect | 0 (summary) | 1 PutItem + 1 UpdateItem | New row family + projection + audit retention policy | Same as C |

The decision matrix:

- **A is out** because the locked human decision says "extensible to additional rater roles later, without a destructive migration." A would require a destructive migration the moment a second rater role lands.
- **B is out** because the analysis agent (Phase 9) and the admin pool view both want quick "what's this puzzle's score" answers. Fanout is acceptable for one-off analyst queries but dies at admin-pool render time.
- **D is out** because CLAUDE.md explicitly prefers "DynamoDB single-table design where practical." A new table buys nothing structurally — the access patterns (read summary by puzzle, read all verdicts by puzzle) are already partition-aligned. The repository pattern in `backend/internal/repository/puzzle.go` uses prefixed PKs (`{size}#{mode}` for puzzles, `CONFIG` for configs); adding `VERDICT#{size}#{mode}#{puzzleId}` slots in cleanly.
- **E is overkill** for two raters and a verdict-summary read pattern. The append-only log is the right shape if we wanted to track "admin upvoted then downvoted then upvoted again" — but the slice's idempotency contract is "re-submission overwrites." E adds a retention question we don't need to answer. If Phase 9's analysis agent decides we want the audit trail, E is additive: convert the per-rater row from "current state" to "log of states" by adding a `submittedAt` partition key. Cheap upgrade path.
- **C wins** because it solves multi-rater extensibility, keeps the hot read path zero-fanout, fits the single-table convention, and leaves a clean upgrade path to E if audit history ever matters.

Open considerations captured in `design.md` Risks: scan-with-filter for the audit query is acceptable today (puzzle pool is a few hundred rows per (size, mode)); when it isn't, add a GSI. Documenting now so the analyst slice picks it up.

### Verdict values — up / down only; skip stays a status

**Locked: the verdict surface offers only `upvote` and `downvote`. Skip stays on the existing `PUT /api/puzzles/{id}/status` flow.**

The ROADMAP entry text says "upvote/downvote/skip", but reading `backend/internal/handler/status.go` line 79 reveals the puzzle status DTO already accepts `"solved"` and `"skipped"` and writes them via `repository.UpdateStatus`. Two parallel concepts named "skip" would create:

- A user-visible question ("does the skip button mean 'I'm not going to play this' or 'this puzzle is bad'?"). The first is a status; the second is a verdict.
- A backend conflict: skipping a puzzle today flips its status to `skipped` but leaves the verdict alone. With three verdict values one of which is `skip`, an admin who skips would be doing both at once — unless we wire one to fire the other, which is dead-weight coupling.

The cleaner read: **status is "what happened during the play attempt" (solved / skipped). Verdict is "what does the rater think of this puzzle as a piece of content" (up / down).** They are orthogonal. An admin who plays a puzzle, finds it boring, skips it, and downvotes it is performing two operations: status=skipped (already wired) and verdict=down (new).

The verdict surface buttons in R-082 render after a play attempt completes — whether by completion (`isSolved`) or by a skip action. The buttons are the same in both places. The skip action does not pre-fill or coerce a verdict — the admin makes both calls.

### Idempotency — PUT semantics, last-write-wins per (puzzleId, raterId)

- **Locked: PUT semantics.** Re-submission by the same `(puzzleId, raterId)` overwrites. No accumulation.
- The summary recompute is safe under repeat writes because it is deterministic from the row set: the write path is "PutItem on the verdict row, then UpdateItem on the PuzzleRecord with the recomputed summary." If the second step crashes (write skew), a follow-up read from the analyst sees a slightly stale summary but no inconsistency in the source-of-truth row family. A post-write reconciliation cron (out of scope for this phase) is the catch-all.
- **Race condition: two admins voting on the same puzzle within a few-ms window.** Two PutItem calls land independently (different SKs — different rater IDs). Two UpdateItem calls on the same PuzzleRecord race on the summary projection. Worst case: the summary lags one vote behind for a few milliseconds. Acceptable — the source of truth is the row family, the summary is a cached projection. Phase 9's analysis agent recomputes from the rows when correctness matters.
- A conditional update on the summary (`SET ... WHERE summary.lastUpdatedAt < :now`) would close the race deterministically but adds a write retry loop that is unjustified at single-admin scale. Locked: skip the conditional check this phase. Document for Phase 9 if the lag becomes visible.

### Play-time and attempt-context capture — yes, capture from day one

R-084 wants "play-time and user-rated difficulty across a labeled corpus." Today the verdict comes from an admin who has just played the puzzle, so the play-time signal is available locally on the frontend at verdict-submission time. **Locked: capture play-time and a few attempt-context fields on the verdict row from day one, even though R-084 is gated on a future public-rater role.**

Captured per verdict:

- `playTimeMs` — wall-clock elapsed time on the attempt that produced this verdict (sourced from `useTimer.elapsed` at completion / skip moment).
- `outcome` — `"solved"` or `"skipped"` (mirrors the puzzle status flow result; named differently to avoid confusing a verdict's outcome with a puzzle's lifecycle status).
- `clientVersion` — git SHA or app version string baked into the frontend bundle. Lets us correlate a wave of downvotes with a UI regression.
- `submittedAt` — server-side timestamp on write.

Why now and not when R-084 lands: capturing later means the verdict corpus split into "before R-084" (no play-time, useless for calibration) and "after R-084" (signal-bearing). The cost is four extra fields on the request body and the verdict row — negligible. The benefit is that R-084 can be done against the verdicts already on file when admin voting has been live for any length of time.

### Migration of the existing `Verdict string` field

The current `PuzzleRecord` ships a `Verdict string \`dynamodbav:"verdict"\`` field defaulted to `"none"`. It is set at generation time and never read by any handler. The R-081 work:

- Removes the `Verdict` field from `PuzzleRecord` and stops writing `verdict: "none"` on new puzzle generation.
- Replaces it with `VerdictSummary` (typed struct, nested under `verdictSummary` JSON key) carrying `{up int, down int, lastUpdatedAt string}`.
- Tolerates legacy rows: old puzzle records still have `verdict: "none"` as a top-level attribute. The new `attributevalue.UnmarshalMap` ignores unknown attributes by default, so reads still work; the field is invisible at the type layer. New writes do not include it.
- No backfill required. The verdict-summary on legacy rows is zero-valued (read on first access produces `{up: 0, down: 0, lastUpdatedAt: ""}`); admin votes after the slice ships populate it forward.

This is the only schema change that touches existing rows, and it's read-tolerant — no destructive migration.

### Frontend — admin-only verdict surface, gated identically to the admin link

- **Locked: the verdict buttons render only for `publicMetadata.role === 'admin'`.** Anonymous and User-role players see no verdict UI. No API calls. No path leakage.
- The gating uses the same `getClerkUserRole(user.publicMetadata) === 'admin'` check that `ProtectedAdminRoute.tsx` uses (`frontend/src/components/auth/role.ts`). Wrapping the verdict surface in a `<Show when="signed-in">` and an admin-role check inside is the convention.
- The buttons render in two places, both inside `GamePage.tsx`:
  1. **Completion overlay** (when `isSolved` flips true). Two buttons next to "Play Again" and "Home": "Good puzzle" / "Bad puzzle." Submitting either advances to the same post-vote state — buttons disable, a small "Thanks — recorded" confirmation shows, then the existing post-completion flow continues.
  2. **Skip action** (when an admin chooses to skip a puzzle they're playing). Skip itself stays on the existing flow; immediately after the status is flipped to `skipped`, the same two buttons render in a small overlay before the user is returned to the landing page. The verdict is optional — skipping without voting is fine.
- Submission posts `PUT /api/admin/puzzles/{id}/verdict` with the request body `{ value: "up" | "down", playTimeMs, outcome, clientVersion }`. On 401 / 403 the buttons hide silently — defense in depth in case role state has shifted mid-session. On 5xx the UI shows a one-line error and lets the user retry; the gameplay flow is not blocked.
- A local in-memory cache of "I've already voted on this puzzle in this session" prevents the buttons from showing again if the same admin re-completes a re-served puzzle in the same tab. The cache is best-effort UX; the backend's idempotent overwrite is the source of truth.

### Verdict on a skipped puzzle when the admin returns later — defer to Phase 8 replay

The locked verdict-on-completion / verdict-on-skip surface only covers verdicts at the moment a play attempt ends. An admin who wants to revisit a puzzle they skipped weeks ago and downvote it would need a UI surface that lists previously-played puzzles — which is exactly what Phase 8's R-086 ships.

**Locked: this phase does not surface a "vote on a previously-played puzzle" UI.** The endpoint accepts the verdict (it's idempotent and the admin's `raterId` is enough to attribute the vote), so Phase 8's replay UI just adds a verdict surface to the puzzle-history page and reuses the existing endpoint. No backend rework required when Phase 8 ships.

### Removed `Verdict` field — a brief callout for the analyst slice

The existing field is unused at read time, but `cmd/genfixtures/main.go` (line 166) and `worker/generator_test.go` (line 140) reference it. Both are updated in R-081 alongside the field removal. The grep sweep is documented in tasks.md. There are no DynamoDB consumers outside the Go code (no Lambda triggers, no Kinesis stream, no analytics export); the change is self-contained.

## Deferred

- **Public-rater role.** Locked decision says "I might add an extra role later." The schema is already shaped for it (per-rater rows keyed by `raterRole`), but no UI, no role assignment, no public-write rate limits ship this phase.
- **Verdict comments / reasons.** Future phase if needed. The verdict row schema can absorb an optional `reason` string without migration.
- **Verdict on previously-played puzzles outside the play loop.** Surfaces in Phase 8 (R-086 replay UI).
- **Audit-trail history (append-only verdict log).** Approach E above. Add when Phase 9 (analysis agent) needs "verdict change over time" signal. Today's PUT-overwrite semantics are sufficient.
- **GSI on `(verdictBucket, createdAt)` for fast "find all downvoted puzzles" scans.** Add when scan-with-filter latency exceeds analyst tolerance. R-087 / R-088 own this decision.
- **Backfill of `verdictSummary` for legacy `verdict: "none"` rows.** Not needed — zero-value summary is correct and indistinguishable from a never-voted puzzle.
- **Conditional-write race protection on summary updates.** Single-admin scale doesn't need it. Add if multi-rater + visible lag becomes a real problem.

## Constraints & Assumptions

- **The Phase 6 admin auth middleware ships before this phase.** Confirmed: R-089 / R-08A / R-08B / R-08C are all `[x]`. The verdict route mounts inside the existing `/api/admin` group with no middleware changes.
- **The puzzle-pool DynamoDB table can absorb a new row family without a Terraform change.** Confirmed: the table uses on-demand billing and a generic schema (`PK STRING, SK STRING`); no GSI changes are required for the verdict row family.
- **Frontend admin role is determined exclusively by `user.publicMetadata.role`.** Confirmed: `frontend/src/components/auth/role.ts` is the canonical reader. The verdict surface uses the same helper.
- **`useTimer` exposes `elapsed` at completion time.** Confirmed: `frontend/src/hooks/useTimer.ts` and the `GameBoard` component use it for the completion overlay's time display. Reusing the same value for `playTimeMs` is a no-op data plumbing change.
- **Skipping a puzzle is the existing `PUT /api/puzzles/{id}/status` flow.** Confirmed: `frontend/src/services/puzzleService.ts` `updatePuzzleStatus` and `backend/internal/handler/status.go`. Verdict capture on skip happens after the status PUT succeeds, not as part of it.
- **Admin role is rare (one or two people).** Confirmed: Phase 6 design grill called this out. The single-table verdict scheme handles up to thousands of admins per puzzle without breaking partition limits, but that scale is irrelevant — the row family will have at most a few rows per puzzle for the foreseeable future.
- **Play-time on the verdict row is the admin's elapsed time on the attempt that produced the verdict, not their fastest run on the puzzle ever.** This matters for R-084 — the calibration test wants per-attempt signal, not per-puzzle aggregates. The frontend hands `useTimer.elapsed` to the API at vote time and that's what gets stored.

## Open Questions (flagged for human override on return)

The autonomous walk landed without genuinely high-stakes opens — the locked decision on voter identity drove most of the choices. Two items are flagged anyway because they have low-cost reversibility windows:

1. **Should the verdict on a skipped puzzle be a separate (smaller) UI surface from the verdict on a completed puzzle, or identical?** The default chosen is **identical** — same two buttons, same flow, same labels — because the admin is making the same judgment ("is this puzzle worth keeping?") in both cases. If the human wants to differentiate ("downvote on skip is a stronger signal than downvote on completion"), the schema already supports it via the `outcome` field — no rework needed; it's a UI question only.

2. **Should the existing `Verdict string` field on `PuzzleRecord` be removed in this slice, or kept as a deprecated alias for one more phase to ease rollback?** The default chosen is **remove now** because (a) the field is unused at read time today (zero handler reads it), (b) DynamoDB tolerates the leftover attribute on legacy rows without intervention, and (c) keeping a deprecated alias forces every new generator-write path to keep populating it. The conservative alternative is "leave the field on `PuzzleRecord` as `verdict_legacy` for one phase, remove in Phase 8" — that's the human's call if reversal cost matters more than schema cleanliness.

## Roadmap Effects

- **Phase 7** keeps its current header (`Verdict System`) and slice IDs (`R-081`, `R-082`).
- **No phase renumbering required.** Phases 7b / 8 / 9 / 10 / 11+ unchanged.
- **No new slice IDs claimed.** R-08D is reserved for the custom-domain follow-up; R-08E and R-08F are next free in the R-08x family but neither is needed — the R-082 PR carries the docs / glossary sweep that Phase 6 split out as R-08C. Per `tasks.md`, the docs sweep is folded into R-082's PR (a third slice would be wasted ceremony for two-slice work).
- **KI catalog update.** No new known issues opened by this phase. The verdict-summary projection's race-condition lag is documented in `design.md` Risks but not promoted to a KI — single-admin scale makes it invisible.
