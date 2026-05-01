# Spec: E2E Stack (lifecycle, queue, table, fixtures)

## Status

Draft. Acceptance criteria prefixed `ES-` (E2E Stack). See `../design.md` §3 for the generator-in-e2e architecture and `../tasks.md` §1–2, §6–7 for the implementation checklist.

## Scope

Lifecycle additions, LocalStack init, fixture seeds, and CI wiring. Test assertions are out of scope (see `e2e-coverage.md`).

## Acceptance criteria — lifecycle

- **ES-01** `task e2e:up:generator` exists and starts the same `cmd/generator/main.go` binary as dev, with env overrides `SQS_QUEUE_URL=http://localhost:4566/000000000000/puzzle-generation-e2e` and `PUZZLE_TABLE_NAME=puzzle-pool-e2e`. PID written to `./logs/e2e-generator.pid`; stdout+stderr to `./logs/e2e-generator.log`.
- **ES-02** `task e2e:down:generator` stops the e2e generator process by reading `./logs/e2e-generator.pid`, sending SIGTERM, waiting up to 5 s, then SIGKILL on timeout. PID file is removed on success.
- **ES-03** `task e2e:up` includes `e2e:up:generator` in its dependency chain in this order: `e2e:up:localstack` → `e2e:up:backend` → `e2e:up:frontend` → `e2e:up:generator` → `e2e:seed` (per D7). Each step blocks on a readiness check before the next runs.
- **ES-04** Generator readiness probe: `e2e:up:generator` polls `./logs/e2e-generator.log` for the literal substring `starting local SQS poller` with a 30 s timeout. Same readiness contract as `dev:up:generator`.
- **ES-05** `task e2e:down` includes `e2e:down:generator`. After `e2e:down`, `lsof -ti:5180`, `lsof -ti:5181`, `lsof -ti:4566` are all empty AND `./logs/e2e-generator.pid` does not exist.
- **ES-06** Per CLAUDE.md Taskfile-shell pitfall guidance: the e2e generator lifecycle wraps its `nohup ... &` + `echo $! > pid` logic inside a `bash <<'BASH' ... BASH` heredoc, since Task's built-in shell returns goroutine handles for `$!` rather than OS PIDs.

## Acceptance criteria — LocalStack init

- **ES-07** `.localstack/init-aws.sh` creates the SQS queue `puzzle-generation-e2e` alongside the existing `puzzle-generation` queue. Idempotent — re-running the script is a no-op if the queue exists.
- **ES-08** `.localstack/init-aws.sh` creates the DynamoDB table `puzzle-pool-e2e` with the **same schema** as `puzzle-pool` (PK, SK, all GSIs). Schema parity is enforced — adding a column to `puzzle-pool` requires the same column on `puzzle-pool-e2e` (lesson 14 sweep covers this).
- **ES-09** `.localstack/init-aws.sh` seeds the `puzzle-pool-e2e` CONFIG items needed by the new specs:
  - replenishment threshold and target for `9#double` (consumed by EC-05)
  - `7#double=false` flag (consumed by EC-04)
  - default mode/difficulty matrix matching the production seed
- **ES-10** LocalStack readiness check (already present for the dev stack) extends to wait for BOTH queues (`puzzle-generation` AND `puzzle-generation-e2e`) to exist AND the `puzzle-pool-e2e` table to be `ACTIVE` before declaring LocalStack ready. Without this, `e2e:up:generator` starts polling a non-existent queue and logs `NonExistentQueue` errors.

## Acceptance criteria — fixtures

- **ES-11** New fixtures live under `frontend/playwright/e2e/fixtures/puzzles/`:
  - `9_double_seed1_000001.json` — 1 puzzle for the `9#double` shape (seeds EC-05's low-count starting state).
  - `7_standard_seed2_000003.json` — 3 puzzles for the `7#standard` shape (seeds EC-06's served-marking baseline).
  - No fixture for `7#double` — KI-007 documents that 7×7 Double has 0 solutions under our adjacency rules, so the generator cannot produce a puzzle for that shape. EC-04 reads the `7#double=false` CONFIG sentinel directly (CONFIG-only, no puzzle row required).
- **ES-12** `backend/cmd/genfixtures/main.go` is extended to produce the two fixtures above deterministically from a seed. Re-running the generator with the same seed produces byte-identical JSON.
- **ES-13** `task e2e:seed` (new task) loads the fixture path passed as an argument, truncates `puzzle-pool-e2e`, and inserts the fixture rows. Used by each serial spec's `beforeAll`. Truncation is a `Scan` + `BatchWriteItem` delete; e2e table size is bounded by fixture size so this is fast.

## Acceptance criteria — CI wiring

- **ES-14** `.github/workflows/ci.yml` injects all 6 Clerk-related env vars into the e2e job: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. All 6 are repo secrets, mirrored to BOTH the Actions scope AND the Dependabot scope per D3 — Dependabot PRs must run the same coverage as branch PRs.
- **ES-15** `.github/workflows/ci.yml` uploads `./logs/e2e-generator.log` as a CI artifact alongside `backend.log` and `frontend.log`. Without this, debugging a flake in CI requires re-running.
- **ES-16** No new GitHub Actions are introduced. Per the locked answer in chunk 4, this slice does not add any `uses:` lines, so CLAUDE.md lesson 19/26 version-pin verification reduces to a no-op for actions; only the `npm install @clerk/testing` lockfile entry needs registry verification at install time.

## Acceptance criteria — runbook

- **ES-17** `docs/runbooks/e2e-clerk-setup.md` exists and documents:
  - Clerk dashboard prep: dev tenant, two test users (`E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_ADMIN_EMAIL`), admin user's `publicMetadata.role = 'admin'`.
  - The full list of 6 secrets and the exact names — the runbook is the source of truth that `ci.yml`, `playwright.config.ts`, and `globalSetup.ts` cross-check against (lesson 14 sweep).
  - The "mirror to Dependabot scope too" reminder per D3.
  - Rate-limit notes (Clerk dev tier limits sign-in attempts per IP — relevant if many parallel CI jobs hit the same tenant).
  - Rotation procedure for when these secrets need to be rolled.

## Decision links

- D5–D7 (generator architecture, queue/table isolation, startup ordering): ES-01 through ES-10.
- D3 (secret mirroring to Dependabot): ES-14, ES-17.
- D8 (truncate-and-reseed contract): ES-13.
- D15 (same generator binary, env-overridden): ES-01.
- D16 (env-override matrix): ES-01.
- D18 (no new GitHub Actions): ES-16.
