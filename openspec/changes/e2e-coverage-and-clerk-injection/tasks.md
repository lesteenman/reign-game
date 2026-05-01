# Tasks: E2E Coverage Expansion + Clerk Admin Session Injection

## Status

| ID | Title | Status |
|----|-------|--------|
| — | E2E coverage expansion + Clerk admin session injection | [x] |

## Work breakdown

### 1. Lifecycle plumbing

- [ ] `Taskfile.yml` — add `e2e:up:generator` and `e2e:down:generator` tasks. Pattern after `dev:up:generator` but pin the queue to `puzzle-generation-e2e` via env var. PID file: `./logs/e2e-generator.pid`.
- [ ] `Taskfile.yml` — wire `e2e:up:generator` into `e2e:up` (deps array) and `e2e:down:generator` into `e2e:down`. Readiness probe: same "starting local SQS poller" log line, but on a separate log file `./logs/e2e-generator.log`.
- [ ] `.localstack/init-aws.sh` — provision the `puzzle-generation-e2e` SQS queue alongside the existing `puzzle-generation` queue. Idempotent guard so dev and e2e LocalStack runs both work.
- [ ] `.localstack/init-aws.sh` — seed the `puzzle-pool` CONFIG items needed by the new specs (replenishment thresholds, the `7#double=false` flip, served-marking state). Use the existing CONFIG seed block; add fixture path comments.

### 2. Fixtures

- [ ] `backend/cmd/genfixtures/main.go` — add fixture-generation cases for: replenishment-trigger pool state, served-pool state (puzzles already marked served), empty-pool state, admin-config baseline.
- [ ] New fixture JSONs under `backend/testdata/fixtures/e2e/` — one file per scenario above. Filenames mirror the spec they support.

### 3. Clerk wiring

- [ ] `frontend/playwright/global-setup.ts` (new) — Clerk testing-token sign-in flow; persists storage-state to `frontend/playwright/.auth/admin.json` and `.auth/user.json` (per D4). Reads all 6 secrets from env: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. Fail fast with a clear message naming any missing var.
- [ ] `frontend/playwright.config.ts` — wire `globalSetup` to the new file; add a `storageState` project for admin specs. Non-admin specs continue running with no auth (anonymous role).
- [ ] `frontend/playwright/.auth/` — gitignore. Storage-state is per-run, never committed.
- [ ] `frontend/playwright/e2e/.env.example` — document the three `E2E_CLERK_*` env vars with non-sensitive placeholders.

### 4. Unblock skipped specs

- [ ] `frontend/playwright/e2e/auth.spec.ts` — remove `test.skip` from all 3 currently-skipped tests; attach them to the admin storageState project.
- [ ] `frontend/playwright/e2e/dynamic-modes.spec.ts` — remove `test.skip` from the 1 admin test and re-target it at the `7#double=false` sentinel per locked decision #6 (see design.md). Assert disabled-state UI; do not assert generation succeeds.

### 5. New specs

- [ ] `frontend/playwright/e2e/pool-replenishment.spec.ts` (new) — exercises the replenishment trigger end-to-end: seed a low pool via `task e2e:seed:pool` with `9_double_seed1_000001.json`, POST `/api/admin/replenish` (verified route per `backend/internal/handler/replenish_test.go` and `backend/cmd/api/main.go:78`), assert pool count crosses the threshold within the 30 s polling window. See `specs/e2e-coverage.md` EC-05.
- [ ] `frontend/playwright/e2e/served-marking.spec.ts` (new) — request a puzzle, complete or surrender it, verify the served flag flips in the pool. Folds the Q-A 404 retry-on-stale assertion (per chunk 4 locked answer): re-request the same puzzle indices, assert response is NOT 404 and the retry path executed (see `specs/e2e-coverage.md` EC-06.5). No separate spec file for this.
- [ ] `frontend/playwright/e2e/pool-empty-fallback.spec.ts` (new) — drain or seed-empty the pool, request a puzzle, assert the documented empty-pool fallback (404 surface per Q-A=(a)).
- [ ] `frontend/playwright/e2e/admin-config-flow.spec.ts` (new) — uses admin storageState; navigate the admin config UI, flip a CONFIG value, verify persistence and the player-side effect.

### 6. CI

- [ ] `.github/workflows/ci.yml` — add all 6 Clerk-related secrets to the e2e job's `env:` block, sourced from GitHub repo secrets and mirrored to BOTH Actions and Dependabot scopes per D3: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. Per chunk 4 lock-in, admin and user passwords are distinct (6th secret added).
- [ ] `.github/workflows/ci.yml` — confirm the e2e generator log (`./logs/e2e-generator.log`) is uploaded as a CI artifact alongside backend/frontend logs (extend existing `actions/upload-artifact` step if present).

### 7. Runbook

- [ ] `docs/runbooks/e2e-clerk-setup.md` (new) — Clerk dashboard prep (2 test users — user + admin, role assignment via `publicMetadata.role = 'admin'`, dev-tier rate-limit notes), the 6 new secrets to add in BOTH Actions and Dependabot scopes per D3 (`CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`), and the rotation procedure. The runbook is the source-of-truth list that `ci.yml`, `playwright.config.ts`, and `globalSetup.ts` are grep-cross-checked against (lesson 14 sweep). One-time-per-environment task.

## Verification Checklist (Phase Close)

Each item must be empirically checkable by re-running a command or inspecting an artifact:

- [ ] `task e2e:up` brings up LocalStack + backend + e2e generator + frontend; `task dev:status` shows all four. `./logs/e2e-generator.pid` is non-empty and the PID is alive.
- [ ] `task e2e:down` stops all four; no orphan processes (`lsof -ti:5180`, `lsof -ti:5181`, `lsof -ti:4566` all empty); `./logs/e2e-generator.pid` cleaned up.
- [ ] `awslocal sqs list-queues` shows BOTH `puzzle-generation` and `puzzle-generation-e2e` after init-aws.sh runs.
- [ ] All 4 previously-skipped specs run (no `test.skip`); confirm via `npx playwright test --list` having zero "skipped" lines for those files.
- [ ] All 4 new specs (`pool-replenishment`, `served-marking`, `pool-empty-fallback`, `admin-config-flow`) pass locally against `task e2e:up`.
- [ ] CI run on the PR is green; the e2e job logs show the admin storageState being created exactly once at the start of the job.
- [ ] `gitleaks detect --source .` clean; no Clerk credentials committed; `frontend/playwright/.auth/` is gitignored.
- [ ] `docs/runbooks/e2e-clerk-setup.md` exists and the secret list matches the env-var names in `.github/workflows/ci.yml` and `playwright/global-setup.ts` (grep cross-check).
- [ ] `tasks.md` status row flipped to `[x]` in the PR per CLAUDE.md lesson 17.
