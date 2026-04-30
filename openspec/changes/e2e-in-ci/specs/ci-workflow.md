# CI Workflow Spec — E2E in CI (slice 1)

Acceptance criteria for `.github/workflows/ci.yml` after this slice
ships. Prefix `CW-` (CI Workflow). Numbered for citation from
`tasks.md`.

## Triggers

- **CW-01.** The workflow triggers on `pull_request` to `main` (no
  change from current state).
- **CW-02.** The workflow declares a `concurrency` block with `group:
  ${{ github.workflow }}-${{ github.ref }}` and `cancel-in-progress:
  ${{ github.event_name == 'pull_request' }}`. PR runs cancel
  in-progress when a new commit lands; non-PR runs do not cancel.

## Jobs

- **CW-03.** Job `frontend-integration` exists with
  `runs-on: ubuntu-latest` and `defaults.run.working-directory:
  frontend`.
- **CW-04.** Job `frontend-e2e` exists with `runs-on: ubuntu-latest`.
  Working directory is the repo root because `task e2e:up` runs
  multiple `dir:`-scoped sub-tasks. Steps that run npm scripts must
  set `working-directory: frontend` per-step.
- **CW-05.** Both jobs are independent (no `needs:` dependency on
  Backend / Frontend / Security / Terraform Plan).

## Services / external setup

- **CW-06.** `frontend-integration` does NOT start LocalStack, the
  e2e backend, or any Docker container. The integration project
  uses `page.route` to mock `/api/*`.
- **CW-07.** `frontend-e2e` starts LocalStack via `docker compose
  up -d localstack` from the repo root. The container uses the
  pinned image referenced in `docker-compose.yml`
  (`localstack/localstack:4.14.0` per CLAUDE.md lesson 22).
- **CW-08.** `frontend-e2e` waits for LocalStack readiness AND the
  init script's completion before any e2e backend boot. Reuses the
  same readiness contract as `task dev:up:localstack` —
  `/_localstack/health` plus the `puzzle-pool` table being ACTIVE
  and the puzzle-generation queue existing.
- **CW-09.** `frontend-e2e` invokes `task e2e:up` once LocalStack is
  ready. Does NOT invoke `task dev:up` (different ports, different
  table — would silently mismatch the playwright config's
  `:5183`/`puzzle-pool-e2e` expectations).

## Environment / secrets

- **CW-10.** `frontend-e2e` exposes `VITE_CLERK_PUBLISHABLE_KEY` and
  `CLERK_SECRET_KEY` as job-level env from `secrets.*`.
- **CW-11.** Both secrets exist in BOTH the Actions secret scope and
  the Dependabot secret scope. Verifiable via repo settings; the
  job behavior on a Dependabot PR confirms it (the auth smoke test
  must run, not skip, on Dependabot PRs that bump
  `@clerk/clerk-react`).
- **CW-12.** Prod-tenant Clerk keys are NOT added to Dependabot
  scope. Only the dev-tenant keys are mirrored.
- **CW-13.** No Clerk secrets appear in plaintext in any workflow
  file or `.github/dependabot.yml`. Grep
  `VITE_CLERK_PUBLISHABLE_KEY|CLERK_SECRET_KEY` returns only
  `${{ secrets.* }}` references.

## Caching

- **CW-14.** Go modules cached via `actions/setup-go`'s built-in
  cache, key `backend/go.sum`.
- **CW-15.** npm packages cached via `actions/setup-node`'s built-in
  cache, key `frontend/package-lock.json`.
- **CW-16.** Playwright browsers cached via `actions/cache@v5.0.5`,
  path `~/.cache/ms-playwright`, key
  `playwright-${{ runner.os }}-${{ hashFiles('frontend/package-lock.json') }}`.
- **CW-17.** `npx playwright install --with-deps chromium` runs only
  the chromium browser (matches project config). Skipping firefox
  and webkit avoids ~60 s of unnecessary download.
- **CW-18.** LocalStack image is NOT cached separately — pulling
  4.14.0 on a warm runner is acceptable per design decision 7.
- **CW-19.** Go build artifacts are NOT cached separately.

## Test invocation

- **CW-20.** `frontend-integration` runs `npm run test:integration`
  (Playwright project filter `--project=integration`).
- **CW-21.** `frontend-e2e` runs `npm run test:e2e` (Playwright
  project filter `--project=e2e`).
- **CW-22.** Neither job runs `npm run test:playwright` (which
  invokes both projects in one run) — the split is the whole point.
- **CW-23.** The auth test "sign-in button renders in the header
  when Clerk is configured" is not skipped in `frontend-e2e`. Visible
  in the job stdout.

## Artifacts

- **CW-24.** Both jobs upload artifacts only on failure
  (`if: failure()`).
- **CW-25.** Retention is 7 days (`retention-days: 7`).
- **CW-26.** `if-no-files-found: ignore` prevents the upload step
  from failing when the failure happened before reports were
  written.
- **CW-27.** `frontend-integration` artifact name is
  `playwright-integration-report` and includes
  `frontend/playwright-report/` and `frontend/test-results/`.
- **CW-28.** `frontend-e2e` artifact name is `playwright-e2e-report`
  and includes `frontend/playwright-report/`,
  `frontend/test-results/`, `logs/backend.log`, and
  `logs/frontend.log`.

## Cleanup

- **CW-29.** `frontend-e2e` runs `task e2e:down` and
  `docker compose down localstack` in an `if: always()` step at the
  end of the job.

## Required checks

- **CW-30.** `frontend-integration` and `frontend-e2e` are added to
  branch protection's required-checks list for `main`. Both jobs
  block merge if red. No soak window — required from day one.
- **CW-31.** Branch protection is verified via
  `gh api repos/:owner/:repo/branches/main/protection
  --jq '.required_status_checks.contexts'` on slice close.

## Performance budgets

- **CW-32.** `frontend-integration` finishes within 8 min on cold
  cache, 5 min on warm cache.
- **CW-33.** `frontend-e2e` finishes within 20 min on cold cache,
  15 min on warm cache.
- **CW-34.** If the e2e job exceeds CW-33 in normal operation
  (post-warm), file an issue and review whether to shard. Don't
  raise the budget silently.

## Out of scope (for this slice)

- **CW-OUT-01.** Path filtering (decision 3 — every PR for first
  month).
- **CW-OUT-02.** Sharding the e2e project across multiple parallel
  workers.
- **CW-OUT-03.** Admin/non-admin Clerk session injection. The two
  remaining `test.skip(needsClerk, …)` predicates in
  `auth.spec.ts` (lines 62, 78) stay skipped in this slice.
- **CW-OUT-04.** Self-hosted runners or any non-`ubuntu-latest`
  runner.
