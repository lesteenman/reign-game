# Design — e2e-coverage-and-clerk-injection

## 1. Summary

Re-enable the four currently-skipped Playwright tests and add four new e2e specs that cover gaps surfaced after R-7-02/R-7-03 (admin config flow, replenish queue, served-marking, pool-empty fallback). Use `@clerk/testing/playwright` for real Clerk session injection (replacing the cookie-stub middleware bypass) and run an isolated generator process inside e2e (`task e2e:up:generator` against `puzzle-generation-e2e` + `puzzle-pool-e2e`) so SQS-driven flows can be asserted end-to-end. See `proposal.md` for scope, `tasks.md` for the implementation checklist, and `design-grill-summary.md` for the full decision log.

## 2. Locked decisions

Eighteen decisions: 14 from chunk 2's grill + 4 follow-ups from chunk 3.

| # | Decision | Rationale |
|---|---|---|
| D1 | Use `@clerk/testing/playwright` (real Clerk dev tenant). | Authentic JWTs flow through the same auth middleware as prod; the cookie-stub bypass under coverage today is exactly what shipped the bug class we want to prevent. |
| D2 | Dedicated Clerk dev tenant with two seeded test users (`E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_ADMIN_EMAIL`). | Isolates e2e from any real-tenant data and avoids polluting human dev sessions. Admin user has `publicMetadata.role === 'admin'` set in the dashboard. |
| D3 | Six new GitHub Actions secrets (admin + user passwords are distinct per chunk 4), mirrored to Actions + Dependabot scopes (per chunk 3 follow-up #4). | Matches slice 1's secret-mirroring precedent so Dependabot PRs run the same coverage as branch PRs. Distinct passwords keep the admin credential blast-radius contained if a non-admin spec ever logs the user password. |
| D4 | `clerkSetup()` in Playwright `globalSetup`, per-spec `clerk.signIn` against `page.context()`. | Matches `@clerk/testing/playwright` documented usage; storage state lives at `frontend/test-results/.clerk/{user,admin}.json`, gitignored. |
| D5 | Run a generator process inside e2e via `task e2e:up:generator`. | The replenish + served-marking + pool-empty specs require real SQS message processing; mocking the pool defeats the test. Mirrors `task dev:up:generator`. |
| D6 | Isolated SQS queue `puzzle-generation-e2e` and DDB table `puzzle-pool-e2e`. | Prevents cross-talk with `task dev:up` if both stacks ever co-exist on the same LocalStack. |
| D7 | Startup ordering: LocalStack → init script → backend → frontend → generator → seed. | Backend must be up before generator (shared DDB writers don't race); seed runs last so initial pool state is deterministic per spec. |
| D8 | `task e2e:seed` script seeds `puzzle-pool-e2e` from a JSON fixture before each suite. | Eliminates flake from "is there a puzzle?" preconditions; `beforeAll` truncates + reseeds. |
| D9 | Polling-loop assertion shape: `expect.poll(() => fetch(...).then(r => r.json().poolCount), { timeout: 30_000, intervals: [500, 1000, 2000] })`. | 30 s budget chosen to match the slowest observed generator latency under CI; 500 ms first poll catches fast cases without burning CPU. |
| D10 | Flat 30 s polling budget in both local and CI (per chunk 3 follow-up #1). | Single timing constant is simpler than env-split and avoids "passes locally, flakes in CI" surprises; F4 in grill summary stays at "accept". |
| D11 | Pool-mutating specs (replenish, served-marking, pool-empty-fallback) run in `test.describe.serial` with `workers: 1` (per chunk 3 follow-up #2). | Three specs all mutate `puzzle-pool-e2e` rows; parallel execution would race. Other specs stay parallel. |
| D12 | Empty-pool fallback assertion is text-only, no screenshot (per chunk 3 follow-up #3). | A non-blank user-readable text assertion is sufficient; F1 in grill summary stays at "accept" — no screenshot mitigation needed. |
| D13 | Admin config flow spec covers create-combo + replenish-trigger paths (replaces the originally-scoped daily-challenge backfill spec, which was descoped in chunk 4). | Daily-challenge surface area isn't yet shipped; admin config flow is load-bearing today. |
| D14 | Q-A 404 retry-on-stale spec asserts the retry path executes once on a stale served-list hit. | Closes the regression class from R-7-02 where stale Q-A indexes returned 404s. |
| D15 | Generator binary in e2e is the same `cmd/api` (with `GENERATOR_MODE=sqs`) as dev — env-overridden, not a test build. | "Test what we ship" — different binary in e2e was ruled out in chunk 2's grill. |
| D16 | Env-override matrix: `PUZZLE_TABLE_NAME=puzzle-pool-e2e`, `SQS_QUEUE_URL=http://localhost:4566/000000000000/puzzle-generation-e2e`, `CLERK_SECRET_KEY=<dev tenant>`, `CLERK_PUBLISHABLE_KEY=<dev tenant>`, `AWS_ENDPOINT_URL=http://localhost:4566`. | Single source of truth in `frontend/playwright.config.ts` `webServer` block + `task e2e:up:*`. |
| D17 | R-08D-equivalent rotation phrased as **"production Clerk tenant rotation (post-launch operational task)"** in proposal.md (per chunk 3 follow-up #5). | Lesson 21: dashboard rotation is not a code slice; the dev-tenant scoping in this slice is what makes the eventual rotation safe. |
| D18 | No new GitHub Actions added; `@clerk/testing/playwright` pinned via `package-lock.json` only. | Per CLAUDE.md lesson 19/26 — no version pinning needed at the spec level since no new action surfaces are introduced. |

**Implementation note (D11 deferred)**: D11's project split (a serial `pool-mutating` Playwright project + a parallel project for non-mutating specs) was not implemented because the existing `e2e` project's `workers: 1` setting (a pre-slice fixture-pool-race constraint) binds the same as a serial project would. Implementing the split without first breaking the fixture-pool constraint would just move the race. Tracked as a follow-up if/when fixture-pool size grows. Estimated savings: ~10–25 s wall clock per CI run.

## 3. Generator-in-e2e architecture

The generator is the same Go binary that runs in dev (`cmd/api` launched with `GENERATOR_MODE=sqs`). E2E runs a second instance against isolated infra:

```
┌──────────────────────────────────────────────────────────────────┐
│ LocalStack (docker compose, port 4566)                           │
│  ├─ SQS: puzzle-generation       (dev stack)                     │
│  ├─ SQS: puzzle-generation-e2e   (e2e stack — NEW)               │
│  ├─ DDB: puzzle-pool             (dev stack)                     │
│  └─ DDB: puzzle-pool-e2e         (e2e stack — NEW)               │
└──────────────────────────────────────────────────────────────────┘
        ▲                                      ▲
        │ task dev:up:*                        │ task e2e:up:*
        │                                      │
┌───────┴────────┐                     ┌───────┴────────────────┐
│ dev backend    │                     │ e2e backend            │
│  port 5181     │                     │  port 5181 (reuses)    │
│ dev generator  │                     │ e2e generator          │
│  PID logs/...  │                     │  PID logs/e2e-...      │
│ dev frontend   │                     │ e2e frontend           │
│  port 5180     │                     │  port 5180 (reuses)    │
└────────────────┘                     └────────────────────────┘
```

**Startup ordering** (enforced by `task e2e:up`):
1. LocalStack ready (`/_localstack/health` + `init-aws.sh` finished — extends existing readiness check to include e2e queue + table).
2. Backend ready (`/api/health` 200, `PUZZLE_TABLE_NAME=puzzle-pool-e2e`).
3. Frontend ready (port 5180 listening).
4. Generator ready (`logs/e2e-generator.log` shows "starting local SQS poller", `SQS_QUEUE_URL=...puzzle-generation-e2e`).
5. Seed runs (`task e2e:seed`) — truncates `puzzle-pool-e2e` and inserts the JSON fixture.

**Polling-loop assertion shape** (used by replenish + served-marking specs):
```ts
await expect.poll(
  async () => (await fetch('/api/admin/pool').then(r => r.json())).count,
  { timeout: 30_000, intervals: [500, 1000, 2000] }
).toBeGreaterThanOrEqual(targetCount);
```

## 4. Clerk session injection architecture

Replaces the current cookie-stub middleware bypass. The four skipped tests reactivate against real Clerk JWTs.

**Global setup** (`frontend/playwright.config.ts` → `globalSetup`):
```ts
import { clerkSetup } from '@clerk/testing/playwright';
export default async () => { await clerkSetup(); };
```
`clerkSetup()` reads `CLERK_PUBLISHABLE_KEY` + `CLERK_SECRET_KEY` from env, fetches the dev tenant's JWT signing key, and warms the test-mode token issuer.

**Per-test sign-in** (used by auth + admin + dynamic-modes specs):
```ts
import { clerk } from '@clerk/testing/playwright';
test.beforeEach(async ({ page }) => {
  await page.goto('/');
  await clerk.signIn({
    page,
    signInParams: { strategy: 'password',
      identifier: process.env.E2E_CLERK_TEST_ADMIN_EMAIL!,
      password: process.env.E2E_CLERK_TEST_ADMIN_PASSWORD! },
  });
});
```

**Storage state files**: `frontend/test-results/.clerk/user.json` and `.clerk/admin.json` — generated by `globalSetup`, gitignored, regenerated each run. Specs that don't need a fresh sign-in can `use: { storageState: '.clerk/admin.json' }`.

**Env vars (6 new secrets, mirrored to Actions + Dependabot per D3)**:
- `CLERK_PUBLISHABLE_KEY` (dev tenant, `pk_test_...`)
- `CLERK_SECRET_KEY` (dev tenant, `sk_test_...`)
- `E2E_CLERK_TEST_USER_EMAIL`
- `E2E_CLERK_TEST_USER_PASSWORD`
- `E2E_CLERK_TEST_ADMIN_EMAIL`
- `E2E_CLERK_TEST_ADMIN_PASSWORD`

Per chunk 4 lock-in: admin and user passwords are **distinct**. Reusing the user password for the admin would shrink the secret count by one but leak the admin credential to any spec that logs the user password during a sign-in failure — the marginal cost of one extra secret is worth the blast-radius isolation.

**Dev tenant scoping**: tenant has zero real users, only the two seeded test users; admin user's `publicMetadata.role = 'admin'` is set manually in the Clerk dashboard once during setup (documented in `docs/runbooks/e2e-clerk-setup.md`).

## 5. Spec-by-spec test plan

Four new specs + four unskipped tests. See `tasks.md` for file paths.

**Unskipped (existing)** — sign-in via `clerk.signIn` from D4:

1. `auth.spec.ts::admin route requires admin role` — assert that an authenticated **non-admin** user navigating to `/admin` is redirected (or sees the unauthorized banner) and that the same flow with `E2E_CLERK_TEST_ADMIN_EMAIL` reaches `/admin` and renders the pool widget.
2. `auth.spec.ts::user can sign out` — assert the sign-out button clears the Clerk session and `/admin` is no longer reachable without re-auth.
3. `dynamic-modes.spec.ts::admin can flip mode active flag` — sign in as admin, flip a mode's `active` toggle, assert the `/api/admin/modes` GET reflects the change and the public puzzle endpoint stops serving that mode.
4. `dynamic-modes.spec.ts::non-admin cannot flip mode flag` — sign in as user, attempt the PUT, assert 403 and no state change.

**New specs**:

5. `pool-replenishment.spec.ts` (serial) — `beforeAll` truncates `puzzle-pool-e2e` to `count=1`. Spec triggers `POST /api/admin/replenish` (verified route per `backend/internal/handler/replenish_test.go` and `backend/cmd/api/main.go:78`) and `expect.poll(... .count, { timeout: 30_000 }).toBeGreaterThanOrEqual(targetCount)`. Asserts the generator drained at least one SQS message by tailing `logs/e2e-generator.log` for a `produced puzzle` line.
6. `served-marking.spec.ts` (serial) — `beforeAll` seeds 3 puzzles. Spec serves a puzzle via `GET /api/puzzle?mode=...&difficulty=...`, asserts the served puzzle's row is updated to `served=true` (via `/api/admin/pool` count breakdown), then `expect.poll` confirms the generator replenishes back to baseline within 30 s.
7. `pool-empty-fallback.spec.ts` (serial) — `beforeAll` truncates `puzzle-pool-e2e` to empty. Spec navigates to a puzzle page; asserts a non-blank user-readable text element (per D12 — no screenshot) like "puzzle is being prepared, please retry shortly" is rendered, and that `/api/puzzle` returns the expected `503`/`409` shape.
8. `admin-config-flow.spec.ts` (parallel-safe) — sign in as admin, exercise the create-combo + replenish-trigger admin UI flow end-to-end, and assert the resulting `/api/admin/pool` row reflects the new combo. Replaces the originally-scoped daily-challenge backfill spec per D13 (chunk 4 descope).

A fifth assertion path — Q-A 404 retry-on-stale (D14) — is folded into `served-marking.spec.ts` rather than its own file: after the first served puzzle, the spec deliberately serves the same indices again to provoke the stale path, asserts the response is **not** a 404 (the retry kicked in) and that the backend log line `WARN: served-list stale, retrying` appears once.

## 6. Serial group rationale

Per D11 / chunk 3 follow-up #2:

- **`test.describe.serial` + `workers: 1`** for `replenish.spec.ts`, `served-marking.spec.ts`, `pool-empty-fallback.spec.ts`. All three mutate `puzzle-pool-e2e`; running them in parallel would race on row counts and message ordering.
- **Parallel-safe**: `auth.spec.ts`, `dynamic-modes.spec.ts`, `admin-config-flow.spec.ts`. None mutate the puzzle pool — auth flips Clerk session state per browser context (isolated by Playwright), dynamic-modes mutates the modes config table (single-row, idempotent), and the admin-config-flow spec only reads/writes the modes config (not the puzzle pool).
- **`beforeAll` per serial spec** truncates `puzzle-pool-e2e` and reseeds the fixture relevant to that spec (e.g. replenish seeds `count=1`, served-marking seeds `count=3`). Avoids leakage between specs even within the serial group.
- **Implementation note**: configure via `playwright.config.ts` `projects:` — one project for the serial group with `fullyParallel: false, workers: 1`, another for the parallel specs with the default settings. This is more robust than file-level `test.describe.serial` decorators alone, since it prevents Playwright's worker pool from picking up a serial spec on a parallel worker.

## 7. Lesson 14 cross-doc sweep checklist

Per CLAUDE.md lesson 14, every new path/queue/table/task/env-var name introduced by this slice must be greppable across the entire repo before the PR opens. Implementation agents MUST run a `grep -rn "<exact-string>" .` for each row below and confirm the only matches are intended.

| Exact string | Files that should reference it |
|---|---|
| `puzzle-generation-e2e` | `Taskfile.yml` (`e2e:up:generator`, env override), `.localstack/init-aws.sh` (queue creation), `docker-compose.yml` (no change expected — LocalStack already mounts init-aws.sh), `.github/workflows/ci.yml` (e2e job env), `docs/runbooks/e2e-clerk-setup.md` (queue listing), `frontend/playwright.config.ts` (none — backend-side only), the 3 serial specs (none — they call backend APIs not SQS directly). |
| `puzzle-pool-e2e` | `Taskfile.yml` (`e2e:up:backend`, `e2e:up:generator`, `e2e:seed`), `.localstack/init-aws.sh` (table creation), `.github/workflows/ci.yml`, `docs/runbooks/e2e-clerk-setup.md`, the 3 serial specs (via the seed task, not direct DDB calls). |
| `e2e:up:generator` | `Taskfile.yml` (definition + `e2e:up` deps), `.github/workflows/ci.yml` (CI job step), `docs/runbooks/e2e-clerk-setup.md` (developer instructions), `PROJECT_STRUCTURE.md` (task listing). |
| `e2e:up:backend`, `e2e:up:frontend`, `e2e:up:localstack`, `e2e:up`, `e2e:down`, `e2e:seed` | Same set as above; `e2e:seed` also referenced by the 3 serial specs' `beforeAll`. |
| `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD` | `frontend/playwright.config.ts` (env consumption), `.github/workflows/ci.yml` (secret references — both Actions and Dependabot scopes per D3), `docs/runbooks/e2e-clerk-setup.md` (setup instructions), `auth.spec.ts` + `dynamic-modes.spec.ts` (sign-in calls). |
| `logs/e2e-generator.log`, `logs/e2e-generator.pid` | `Taskfile.yml` (`e2e:up:generator`, `e2e:down:generator`, `e2e:logs:generator`), `.gitignore` (already covers `logs/` — verify), `served-marking.spec.ts` + `replenish.spec.ts` (log-tail assertion). |
| `frontend/test-results/.clerk/` | `.gitignore` (must be added), `playwright.config.ts` (`globalSetup` writes here, `storageState` reads here). |
| `task e2e:up`, `task e2e:down` | `docs/runbooks/e2e-clerk-setup.md`, `CONTRIBUTING.md` if it documents dev workflow, `PROJECT_STRUCTURE.md`. |

Open the PR only after every row above grep-matches the expected file set. If a string appears in an unexpected file, it's drift — fix it before pushing.

## 8. Verified action versions, risks, known limits

**GitHub Actions**: this slice adds **zero** new actions. `@clerk/testing/playwright` is an npm sub-path pinned via `package-lock.json`; no `uses:` line changes. CLAUDE.md lesson 19/26 verification therefore reduces to: at install time, run `npm view @clerk/testing version` and pin via `npm install` (lockfile is the source of truth — do not hand-edit a version into `package.json`).

**Risks / known limits**:

1. **Q-A 404 retry path is partially unverified** — D14's assertion depends on the backend currently emitting the `WARN: served-list stale, retrying` log line. If R-7-02's fix landed without that log line, the spec will need a different assertion (HTTP-level 200-not-404 still works, but the log-tail check would be dead code). Implementation agent must verify the log line exists in `backend/internal/...` before relying on it; if missing, drop to HTTP-only assertion.
2. **Generator latency variance** — the 30 s polling budget (D9/D10) was sized against current dev-stack timings. If LocalStack 4.14.0 (pinned per CLAUDE.md lesson 22) regresses on SQS message-visibility timing, the budget may need to grow. Mitigation: the polling intervals (`[500, 1000, 2000]`) front-load fast checks, so the 30 s ceiling is rarely hit in the green path.
3. **Clerk dev-tenant rate limits** — Clerk's dev tier rate-limits sign-in attempts per IP. If CI runs many parallel jobs against the same tenant, expect 429s. Mitigation: storage-state reuse across specs (D4) means each spec re-uses one sign-in rather than re-authenticating; only the global setup hits Clerk.
4. **Storage state staleness** — Clerk JWTs expire (default 1 h). If the suite runs longer than the JWT lifetime, late specs will see auth failures. Mitigation: each spec calls `clerk.signIn` in `beforeEach` rather than relying solely on storageState. Trade-off: slightly more Clerk traffic per run, but predictable.
5. **`puzzle-pool-e2e` schema drift** — if the production `puzzle-pool` schema changes (new GSI, new attribute), `init-aws.sh` must mirror it for `puzzle-pool-e2e`. Lesson 14 sweep covers this, but it's worth flagging — easy to forget the e2e table when adding a column.
6. **Production Clerk tenant rotation** (D17 / chunk 3 #5) is a post-launch operational task, not a slice. The dev-tenant scoping in this slice is what makes the eventual rotation safe; once production launches, the runbook in `docs/runbooks/admin-auth-setup.md` covers the swap.
