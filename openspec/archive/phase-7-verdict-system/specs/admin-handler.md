# Spec: Verdict Admin Handler

The HTTP contract for the new admin-only verdict endpoint.

## VH-01: Endpoint mounts inside the Phase 6 admin route group

**Rule.** The endpoint is registered as `r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))` inside the existing `r.Route("/api/admin", func(r chi.Router) { r.Use(auth.RequireAuth, auth.RequireAdmin); ... })` block in `backend/cmd/api/main.go`. No new middleware is added. No alternate mount path is created.

**Value.** Phase 6's BM-05 invariant ("admin routes are grouped under `/api/admin` with middleware applied once") is preserved by construction. Anonymous → 401 and signed-in non-admin → 403 are inherited; the verdict handler does not re-implement them.

**Verification.** Code search: the only `r.Method(...)` call ending in `/verdict` lives inside the `/api/admin` group. Integration tests using the existing `mountAdminWithAuth` helper iterate `adminAuthMatrix` and assert 401 / 403 / 200 for each session state.

## VH-02: Path is `PUT /api/admin/puzzles/{id}/verdict?size={size}&mode={mode}`

**Rule.** The full path includes the puzzle ID as a path parameter and the puzzle's `size` and `mode` as required query parameters. Without `size` and `mode` the handler cannot construct the partition key for the puzzle row, so they are mandatory and validated before any DynamoDB call.

**Value.** Mirrors the `PUT /api/puzzles/{id}/status` convention exactly (see `backend/internal/handler/status.go`). One pattern for "address a puzzle row by HTTP" across the surface — no surprise for future contributors.

**Verification.** Test: `PUT /api/admin/puzzles/abc/verdict` (no query params) → 400 with `invalid_params`. `PUT /api/admin/puzzles/abc/verdict?size=5` (missing mode) → 400. Happy path requires both.

## VH-03: Request body is `{ value, playTimeMs, outcome, clientVersion }`

**Rule.** The handler decodes the request body as JSON with exactly four fields:

| Field | Type | Constraint |
|---|---|---|
| `value` | string | `"up"` or `"down"` — case-sensitive equality. |
| `playTimeMs` | int64 | `>= 0`. NaN, Inf, negative, or non-numeric → 400. |
| `outcome` | string | `"solved"` or `"skipped"` — case-sensitive equality. |
| `clientVersion` | string | Free-form. Empty string permitted (frontend defaults to `"dev"` in local). |

Unknown fields are ignored. Missing required fields produce 400.

**Value.** Strict, narrow validation. `value` is the verdict; the other three are the calibration signal R-084 needs from day one.

**Verification.** Per-field test cases: `value="UP"` → 400, `value="upvote"` → 400, `playTimeMs=-1` → 400, `playTimeMs="50"` (string) → 400, `outcome="completed"` → 400, missing `value` → 400. Happy path with all four valid → 200.

## VH-04: Skip is rejected from the verdict surface

**Rule.** `value="skip"` returns 400 `invalid_params`. Skipping a puzzle remains the existing `PUT /api/puzzles/{id}/status` flow with `status="skipped"`. The verdict and status flows do not coerce one another — the handler does not flip puzzle status as a side effect of receiving a verdict, and the status handler does not write a verdict.

**Value.** `Status` and `Verdict` are orthogonal concepts (see GLOSSARY.md). Two parallel "skip" entry points would create a user-visible question ("which one am I doing?") and a backend coupling problem.

**Verification.** Test: `value="skip"` → 400. Test: posting a valid verdict does not change the puzzle's `status` attribute in DynamoDB. Test: posting `PUT /api/puzzles/{id}/status` with `"skipped"` does not create a verdict row.

## VH-05: Puzzle existence is verified before the verdict row is written

**Rule.** The handler calls `repo.GetPuzzle(ctx, size, mode, puzzleID)` first. If the puzzle row is absent, it returns 404 `not_found` without writing a verdict row.

**Value.** Verdict rows for non-existent puzzles are dead data — they would never be read because the read path is "look up the puzzle, read its summary." Defending the endpoint at the boundary keeps the row family clean.

**Verification.** Test: PUT verdict against a `puzzleId` that does not exist in the table → 404. No verdict row is written (assert by `ListVerdictsForPuzzle` returning empty after the failed call).

## VH-06: `raterId` is sourced from the Clerk session, never the request body

**Rule.** The handler reads the authenticated user via `auth.UserFromContext(r.Context())` and uses `user.ID` as the verdict row's `raterId`. The request body has no `raterId` field. Even if a malicious caller adds one to the JSON, it is ignored — the field is not part of the decoded struct.

**Value.** Vote attribution cannot be spoofed. An admin's votes are bound to their Clerk identity for the life of their account.

**Verification.** Test: PUT verdict with `{ value: "up", raterId: "imposter", ... }` in the body → row's `SK` is `"admin#<authenticated-user-id>"`, not `"admin#imposter"`. Inspection of the decoded struct confirms `raterId` is not a field.

## VH-07: `raterRole` is hard-coded to `"admin"` this phase

**Rule.** The handler writes `RaterRole: "admin"` on every verdict row. The `RequireAdmin` middleware has already proven the caller has the admin role — no second check is needed inside the handler.

**Value.** Schema is multi-rater-ready (the SK includes the role) but the slice does not yet need to read the role from `user.publicMetadata`. When the public-rater role lands, this becomes a one-line change: `RaterRole: getClerkRole(user)`.

**Verification.** Code review: the literal `"admin"` is the only `RaterRole` value the handler writes. Grep confirms no read from `user.publicMetadata.role` inside `verdict.go`.

## VH-08: Successful write returns the recomputed summary in the response

**Rule.** On success (after `PutVerdict` and `RecomputeVerdictSummary` both succeed), the handler responds 200 with `{ "summary": { "up": N, "down": M, "lastUpdatedAt": "..." } }`. The frontend does not need a follow-up GET to know the new totals.

**Value.** One round trip per vote. The verdict surface can render the new totals immediately if a future UI wants to ("3 admins agree this is bad").

**Verification.** Test: first vote (`up`) → `{ up: 1, down: 0 }`. Second admin votes `down` → `{ up: 1, down: 1 }`. Same admin overwrites with `down` → `{ up: 0, down: 2 }`.

## VH-09: Summary recompute failure is logged WARN but does not fail the request

**Rule.** If `PutVerdict` succeeds but `RecomputeVerdictSummary` fails (e.g., transient DynamoDB error on the second UpdateItem), the handler logs `WARN: verdict: summary recompute failed puzzle=<id> err=<reason>` and still responds 200. The verdict row family is canonical; the summary is a cached projection that will be re-converged on the next vote.

**Value.** The user-visible action ("I voted") succeeded — the row exists. The cached summary lagging by one vote is not a 5xx-worthy condition. Failing the request would invite the admin to re-click, producing a redundant overwrite of the row they just successfully wrote.

**Verification.** Test with a repository fake whose `RecomputeVerdictSummary` returns an error: handler still responds 200. Log assertion captures the WARN line. The verdict row is present in the fake table.

## VH-10: Idempotent overwrite — same `(puzzleId, raterId)` produces exactly one row

**Rule.** Re-submission by the same authenticated user against the same puzzle overwrites the existing verdict row. The row family contains at most one row per `(puzzleId, raterId)` pair regardless of how many votes the admin submits.

**Value.** PUT semantics. The admin who clicks up, then down, then up again ends up with `value: up` on file — not three rows.

**Verification.** Test: same admin submits up, down, up. After all three writes, `ListVerdictsForPuzzle` returns exactly one row with `value: "up"`. Summary is `{ up: 1, down: 0 }`.

## VH-11: Logs name the puzzle and rater but not the cookie or session token

**Rule.** Successful writes log `verdict: write puzzle=<id> rater=<clerkUserId> value=<up|down>`. Failed writes log the same context plus the reason. Session tokens, cookie values, and the full Clerk user object are never logged. Inherits BM-07 from the auth middleware.

**Value.** The Clerk user ID is a random string — safe to log for correlation. The session token is a credential.

**Verification.** Code review: `log.Printf` calls in `verdict.go` only pass the four fields above. No `%v` / `%+v` on session or user objects.
