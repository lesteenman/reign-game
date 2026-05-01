# Spec: E2E Coverage (4 new specs + 4 unskipped tests)

## Status

Draft. Acceptance criteria prefixed `EC-` (E2E Coverage). See `../design.md` §5 for the architectural rationale and `../tasks.md` §4–5 for the implementation checklist.

## Scope

This spec captures the assertion shape and serial-group placement for every Playwright test added or unskipped by the slice. It does NOT define lifecycle, fixtures, or queue/table provisioning — those belong to `e2e-stack.md`.

## Serial group membership

Per `../design.md` D11: pool-mutating specs run in a Playwright project with `fullyParallel: false, workers: 1`. Other specs stay parallel.

| Spec | Group | Why |
|---|---|---|
| `auth.spec.ts` (4 tests, 3 unskipped + 1 existing) | parallel | Mutates Clerk session per browser context only — Playwright isolates contexts already. |
| `dynamic-modes.spec.ts` (2 tests, 1 unskipped + 1 existing) | parallel | Mutates the `modes` config row, which is single-row idempotent. |
| `pool-replenishment.spec.ts` (new) | **serial** | Mutates `puzzle-pool-e2e` rows; races on count if parallel. |
| `served-marking.spec.ts` (new) | **serial** | Same as above; also folds the Q-A 404 retry assertion (per locked answer for Q-A). |
| `pool-empty-fallback.spec.ts` (new) | **serial** | Truncates `puzzle-pool-e2e` to empty before assertion — destructive. |
| `admin-config-flow.spec.ts` (new) | parallel | Mutates a single config row; idempotent under retry. |

Total spec files added by this slice: **4**. Total tests unskipped: **4** (3 in `auth.spec.ts` + 1 in `dynamic-modes.spec.ts`).

## Acceptance criteria — unskipped tests

For each test below, the previous `test.skip` predicate was "no Clerk session injection in headless e2e." That predicate is removed because `clerkSetup()` + `clerk.signIn()` from `@clerk/testing/playwright` are now wired in `globalSetup` and `beforeEach` (see `../design.md` §4).

- **EC-01** `auth.spec.ts::admin route requires admin role` — Given an authenticated **non-admin** user, when they navigate to `/admin`, then they are redirected away or shown an unauthorized banner. Given an authenticated **admin** user (signed in with `E2E_CLERK_TEST_ADMIN_EMAIL` + `E2E_CLERK_TEST_ADMIN_PASSWORD`), when they navigate to `/admin`, then the admin pool widget renders.
- **EC-02** `auth.spec.ts::user can sign out` — Given an authenticated session, when the sign-out button is clicked, then the Clerk session is cleared and a subsequent navigation to `/admin` redirects to the sign-in page.
- **EC-03** `auth.spec.ts::admin session persists across reload` — Given an authenticated admin session, when the page reloads, then `/admin` still renders without re-authentication (validates storage-state survives reload).
- **EC-04** `dynamic-modes.spec.ts::admin can flip mode active flag` — Re-targeted per D6 at the `7#double=false` sentinel (KI-007 still infeasible). Given an authenticated admin, when they GET `/api/admin/modes`, then `7#double` shows `active=false`. The spec asserts the disabled-state UI; it does NOT attempt to enable `7#double` (would fail at generator level — see `../design.md` D6).

## Acceptance criteria — new specs

### EC-05 — `pool-replenishment.spec.ts` (serial)

- **EC-05.1** `beforeAll` calls `task e2e:seed:pool` with the low-count fixture (`9_double_seed1_000001.json`) so `puzzle-pool-e2e` starts at `count=1` for the `9#double` shape.
- **EC-05.2** Spec POSTs `POST /api/admin/replenish` (verified route per `backend/internal/handler/replenish_test.go` and `backend/cmd/api/main.go:78`) as the authenticated admin.
- **EC-05.3** Spec then asserts: `expect.poll(() => fetch('/api/admin/pool').then(r => r.json()).count, { timeout: 30_000, intervals: [500, 1000, 2000] }).toBeGreaterThanOrEqual(targetCount)` where `targetCount` is the configured replenishment target for `9#double`.
- **EC-05.4** Spec tails `./logs/e2e-generator.log` and asserts at least one line matching `/generator: produced puzzle X.*9#double/` was emitted during the polling window. This proves the SQS path was exercised, not just the DDB count.

### EC-06 — `served-marking.spec.ts` (serial, includes Q-A 404 retry assertion)

- **EC-06.1** `beforeAll` seeds 2 puzzles into `puzzle-pool-e2e` for the `7#standard` shape (`7_standard_seed1_000001.json` + `7_standard_seed2_000003.json`).
- **EC-06.2** Spec serves a puzzle via `GET /api/puzzles/next?size=7&mode=standard`, captures the returned puzzle's index.
- **EC-06.3** Spec asserts the served puzzle's row is updated to `served=true` by hitting `GET /api/admin/pool` and inspecting the count breakdown — the served-status delta must be exactly +1.
- **EC-06.4** `expect.poll` confirms the generator replenishes back to baseline (`count >= 3` for `7#standard`) within the 30 s polling budget.
- **EC-06.5 (Q-A 404 retry)** After EC-06.2, the spec deliberately re-requests `GET /api/puzzles/next?size=7&mode=standard` with the same client state to provoke the stale served-list code path. Assertion: response is **NOT** 404 (the retry-on-stale path executed and returned a fresh puzzle). If the backend currently emits `WARN: served-list stale, retrying`, the spec also tails the backend log for that line — but the HTTP-level assertion is the load-bearing one (per `../design.md` §8 risk #1).

### EC-07 — `pool-empty-fallback.spec.ts` (serial)

- **EC-07.1** `beforeAll` truncates `puzzle-pool-e2e` to empty (no fixture seed).
- **EC-07.2** Spec navigates to a puzzle page (`/play?flow=curation&size=9&mode=double`).
- **EC-07.3** Spec asserts a non-blank user-readable text element is rendered. Per D12, this is text-only — no screenshot. Acceptable selectors: `[data-testid="empty-pool-message"]`, or text matching `/preparing|please retry|temporarily unavailable/i`. Implementation agent picks the exact selector based on what the UI ships.
- **EC-07.4** Spec asserts `GET /api/puzzles/next?size=7&mode=standard` returns the documented empty-pool surface — HTTP `404` with a JSON body containing `{ "error": "no_puzzles_available" }` (verified against `backend/internal/httperr/httperr.go` and the `served-marking.spec.ts` assertion landed in chunk 3).

### EC-08 — `admin-config-flow.spec.ts` (parallel)

- **EC-08.1** Spec uses admin `storageState` from `frontend/test-results/.clerk/admin.json`.
- **EC-08.2** Spec navigates to `/admin`, clicks into the modes/config widget, flips a non-load-bearing toggle (NOT `7#double`, which is the disabled sentinel), saves.
- **EC-08.3** Spec asserts `GET /api/config/modes` reflects the new value.
- **EC-08.4** Spec asserts the player-side effect: a public puzzle endpoint either serves or refuses the toggled mode according to the new flag. Concrete assertion shape: `GET /api/puzzle?mode=<toggled>` returns 200 if enabled, 404 if disabled.
- **EC-08.5** Cleanup in `afterAll`: flip the toggle back to its original value to keep the parallel-safe contract intact.

## Decision links

- Q-A (404 retry placement): folded into EC-06 — no separate spec file. Per locked answer in chunk 4.
- D6 (`7#double` retarget): EC-04.
- D9–D10 (polling budget): EC-05.3, EC-06.4 use `timeout: 30_000`.
- D11 (serial group): the 3 pool-mutating specs use a Playwright `project` with `fullyParallel: false, workers: 1`.
- D12 (text-only fallback assertion): EC-07.3.
- D13 (clock override): not used by EC-01..08; D13 was scoped to a daily-challenge spec that did not survive into the final 4-spec list.
- D14 (Q-A 404 retry): folded into EC-06.5.
