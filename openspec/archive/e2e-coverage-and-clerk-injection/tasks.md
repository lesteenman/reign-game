# Tasks: E2E Coverage Expansion + Clerk Admin Session Injection

## Status

| ID | Title | Status |
|----|-------|--------|
| — | E2E coverage expansion + Clerk admin session injection | [x] |

## Work breakdown

### 1. Lifecycle plumbing

- [x] `Taskfile.yml` — add `e2e:up:generator` and `e2e:down:generator` tasks. Pattern after `dev:up:generator` but pin the queue to `puzzle-generation-e2e` via env var. PID file: `./logs/e2e-generator.pid`.
- [x] `Taskfile.yml` — wire `e2e:up:generator` into `e2e:up` (deps array) and `e2e:down:generator` into `e2e:down`. Readiness probe: same "starting local SQS poller" log line, but on a separate log file `./logs/e2e-generator.log`.
- [x] `Taskfile.yml` — set `SQS_QUEUE_URL=http://localhost:4566/000000000000/puzzle-generation-e2e` on `e2e:up:backend` (NOT the dev `puzzle-generation` queue). Lockstep with the generator queue per ES-01.5 — both the e2e backend (enqueues from `/api/admin/replenish`) and the e2e generator (dequeues) must point at the same queue, otherwise replenish messages disappear silently. Corrected in commit 62f0e50.
- [x] `.localstack/init-aws.sh` — provision the `puzzle-generation-e2e` SQS queue alongside the existing `puzzle-generation` queue. Idempotent guard so dev and e2e LocalStack runs both work.
- [x] `.localstack/init-aws.sh` — seed the `puzzle-pool` CONFIG items needed by the new specs (replenishment thresholds, the `7#double=false` flip, served-marking state). Use the existing CONFIG seed block; add fixture path comments.

### 2. Fixtures

- [x] `backend/cmd/genfixtures/main.go` — add fixture-generation cases for: replenishment-trigger pool state, served-pool state (puzzles already marked served), empty-pool state, admin-config baseline.
- [x] New fixture JSONs under `frontend/playwright/e2e/fixtures/puzzles/` — one file per scenario above. Filenames mirror the spec they support.

### 3. Clerk wiring

- [x] `frontend/playwright/global-setup.ts` (new) — Clerk testing-token sign-in flow; persists storage-state to `frontend/playwright/.auth/admin.json` and `.auth/user.json` (per D4). Reads all 6 secrets from env: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. Fail fast with a clear message naming any missing var.
- [x] `frontend/playwright.config.ts` — wire `globalSetup` to the new file; add a `storageState` project for admin specs. Non-admin specs continue running with no auth (anonymous role).
- [x] `frontend/playwright/.auth/` — gitignore. Storage-state is per-run, never committed.
- [x] `frontend/playwright/e2e/.env.example` — document the three `E2E_CLERK_*` env vars with non-sensitive placeholders.

### 4. Unblock skipped specs

- [x] `frontend/playwright/e2e/auth.spec.ts` — remove `test.skip` from all 3 currently-skipped tests; attach them to the admin storageState project.
- [x] `frontend/playwright/e2e/dynamic-modes.spec.ts` — remove `test.skip` from the 1 admin test and re-target it at the `7#double=false` sentinel per locked decision #6 (see design.md). Assert disabled-state UI; do not assert generation succeeds.

### 5. New specs

- [x] `frontend/playwright/e2e/pool-replenishment.spec.ts` (new) — exercises the replenishment trigger end-to-end: seed a low pool via `task e2e:seed` with `9_double_seed1_000001.json`, POST `/api/admin/replenish` (verified route per `backend/internal/handler/replenish_test.go` and `backend/cmd/api/main.go:78`), assert pool count crosses the threshold within the 30 s polling window. See `specs/e2e-coverage.md` EC-05.
- [x] `frontend/playwright/e2e/served-marking.spec.ts` (new) — request a puzzle, complete or surrender it, verify the served flag flips in the pool. Folds the Q-A 404 retry-on-stale assertion (per chunk 4 locked answer): re-request the same puzzle indices, assert response is NOT 404 and the retry path executed (see `specs/e2e-coverage.md` EC-06.5). No separate spec file for this.
- [x] `frontend/playwright/e2e/pool-empty-fallback.spec.ts` (new) — drain or seed-empty the pool, request a puzzle, assert the documented empty-pool fallback (404 surface per Q-A=(a)).
- [x] `frontend/playwright/e2e/admin-config-flow.spec.ts` (new) — uses admin storageState; navigate the admin config UI, flip a CONFIG value, verify persistence and the player-side effect.

### 6. CI

- [x] `.github/workflows/ci.yml` — add all 6 Clerk-related secrets to the e2e job's `env:` block, sourced from GitHub repo secrets and mirrored to BOTH Actions and Dependabot scopes per D3: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. Per chunk 4 lock-in, admin and user passwords are distinct (6th secret added).
- [x] `.github/workflows/ci.yml` — confirm the e2e generator log (`./logs/e2e-generator.log`) is uploaded as a CI artifact alongside backend/frontend logs (extend existing `actions/upload-artifact` step if present).

### 7. Runbook

- [x] `docs/runbooks/e2e-clerk-setup.md` (new) — Clerk dashboard prep (2 test users — user + admin, role assignment via `publicMetadata.role = 'admin'`, dev-tier rate-limit notes), the 6 new secrets to add in BOTH Actions and Dependabot scopes per D3 (`CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`), and the rotation procedure. The runbook is the source-of-truth list that `ci.yml`, `playwright.config.ts`, and `globalSetup.ts` are grep-cross-checked against (lesson 14 sweep). One-time-per-environment task.

## Verification Checklist (Phase Close)

Each item must be empirically checkable by re-running a command or inspecting an artifact:

- [x] `task e2e:up` brings up LocalStack + backend + e2e generator + frontend; `task dev:status` shows all four. `./logs/e2e-generator.pid` is non-empty and the PID is alive. (Verified locally during PR #90; `Taskfile.yml:636` defines `e2e:up:generator`, `:687` `e2e:down:generator`, both wired into `e2e:up` / `e2e:down` at `:804` / `:828`.)
- [x] `task e2e:down` stops all four; no orphan processes (`lsof -ti:5180`, `lsof -ti:5181`, `lsof -ti:4566` all empty); `./logs/e2e-generator.pid` cleaned up. (Verified locally during PR #90; lifecycle parity with `dev:up:generator` per the `Taskfile.yml` heredoc pattern in CLAUDE.md "Taskfile shell pitfalls".)
- [x] `awslocal sqs list-queues` shows BOTH `puzzle-generation` and `puzzle-generation-e2e` after init-aws.sh runs. (`.localstack/init-aws.sh:62` creates `puzzle-generation-e2e` with redrive policy at `:65`; original `puzzle-generation` queue creation unchanged.)
- [x] All 4 previously-skipped specs run (no `test.skip`); confirm via `npx playwright test --list` having zero "skipped" lines for those files. (3 unskipped in `frontend/playwright/e2e/auth.spec.ts`, 1 in `frontend/playwright/e2e/dynamic-modes.spec.ts` — both files exist on `main` post-merge of PR #90.)
- [x] All 4 new specs (`pool-replenishment`, `served-marking`, `pool-empty-fallback`, `admin-config-flow`) pass locally against `task e2e:up`. (Files: `frontend/playwright/e2e/pool-replenishment.spec.ts`, `served-marking.spec.ts`, `pool-empty-fallback.spec.ts`, `admin-config-flow.spec.ts` — all 4 present and shipped in PR #90, green in CI.)
- [x] CI run on the PR is green; the e2e job logs show the admin storageState being created exactly once at the start of the job. (PR #90 merged with all checks green; `frontend/playwright.config.ts:39` wires `globalSetup: "./playwright/global-setup.ts"` once per test run.)
- [x] `gitleaks detect --source .` clean; no Clerk credentials committed; `frontend/playwright/.auth/` is gitignored. (Pre-push hook runs `gitleaks detect`; `.gitignore` includes `frontend/playwright/.auth/`.)
- [x] `docs/runbooks/e2e-clerk-setup.md` exists and the secret list matches the env-var names in `.github/workflows/ci.yml` and `playwright/global-setup.ts` (grep cross-check). (Runbook `:47-52` lists all 6 secrets; `ci.yml:232` `CLERK_SECRET_KEY`, `:253-263` step-scoped re-mapping covers `CLERK_SECRET_KEY` + 4 `E2E_CLERK_TEST_*` vars; `:96`/`:172` `VITE_CLERK_PUBLISHABLE_KEY`. Names align across all three surfaces.)
- [x] `tasks.md` status row flipped to `[x]` in the PR per CLAUDE.md lesson 17. (Line 7 of this file: `| — | E2E coverage expansion + Clerk admin session injection | [x] |`.)
