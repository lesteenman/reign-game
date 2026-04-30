# E2E in CI — tasks (slice 1)

## Status

| ID | Slice | Status |
|---|---|---|
| — | E2E in CI (Playwright integration + e2e jobs) | [ ] |

Flip to `[x]` in the same branch as the implementation per CLAUDE.md
lesson 17.

## Implementation agent

**`devops-engineer`** owns this slice (workflow + Taskfile changes).
The agent must read its own definition and the lessons in CLAUDE.md
for CI / workflow rules.

## Per-file work items

### `.github/workflows/ci.yml`

1. Add a top-level `concurrency:` block on the workflow (above
   `permissions:` is the conventional spot):

   ```yaml
   concurrency:
     group: ${{ github.workflow }}-${{ github.ref }}
     cancel-in-progress: ${{ github.event_name == 'pull_request' }}
   ```

   Implements decision 9. Note the conditional shape — direct boolean
   `true` would also cancel hypothetical future `main`-branch runs.

2. Add a new job `frontend-integration`:
   - `runs-on: ubuntu-latest`
   - `defaults: { run: { working-directory: frontend } }`
   - `actions/checkout@v6`
   - `actions/setup-node@v6` with `node-version: "24"`, `cache: npm`,
     `cache-dependency-path: frontend/package-lock.json`
   - `actions/cache@v5.0.5` for Playwright browsers, key
     `playwright-${{ runner.os }}-${{ hashFiles('frontend/package-lock.json') }}`,
     path `~/.cache/ms-playwright`
   - `npm ci`
   - `npx playwright install --with-deps chromium` (only chromium —
     matches the project config's `browserName: 'chromium'`)
   - `npm run test:integration`
   - On failure, `actions/upload-artifact@v7.0.1` with
     `if-no-files-found: ignore`, `retention-days: 7`,
     `name: playwright-integration-report`,
     `path: |\n        frontend/playwright-report/\n        frontend/test-results/`,
     `if: failure()`

3. Add a new job `frontend-e2e`:
   - Same runs-on + checkout + setup-node + cache + Playwright cache
     steps as `frontend-integration`.
   - Also `actions/setup-go@v6` with `go-version: "1.26"` and
     `cache-dependency-path: backend/go.sum` (Go is needed for the
     e2e backend binary that `task e2e:up:backend` builds + runs).
   - `arduino/setup-task@v2` (`version: 3.x`,
     `repo-token: ${{ secrets.GITHUB_TOKEN }}`).
   - Start LocalStack via `docker compose up -d localstack` in the
     repo root. The existing `docker-compose.yml` mounts
     `.localstack/init-aws.sh`. Reference: decision 4 / lesson 14.
   - Wait for LocalStack readiness — script should poll
     `/_localstack/health` AND wait for the init script's completion
     marker (e.g. the `puzzle-pool` table in ACTIVE state and the
     puzzle-generation queue existing). Reuse whatever `dev:up`
     does; do not re-implement.
   - Run `task e2e:up`. This boots e2e backend on :5182, e2e
     frontend (Vite proxy) on :5183, seeds fixtures, warms the
     `/api/config/modes` cold path. Per lesson 25, do not split out
     warmup or seeding into the workflow — keep it inside the task.
   - Env vars on the job: `VITE_CLERK_PUBLISHABLE_KEY` and
     `CLERK_SECRET_KEY` from `${{ secrets.* }}` (decision 5). Both
     are needed: the frontend bundle reads `VITE_CLERK_*`, the
     backend reads `CLERK_SECRET_KEY` (verify against
     `backend/cmd/api/main.go`'s clerk SDK init).
   - `npm run test:e2e` (working-directory: frontend).
   - On failure, `actions/upload-artifact@v7.0.1` with
     `if-no-files-found: ignore`, `retention-days: 7`,
     `name: playwright-e2e-report`,
     `path: |\n        frontend/playwright-report/\n        frontend/test-results/\n        logs/backend.log\n        logs/frontend.log`,
     `if: failure()`. Decision 8.
   - Final step (always run): `task e2e:down` and `docker compose
     down localstack`. Use `if: always()` to ensure cleanup runs
     even on failure. Local lifecycle cleanup matters less in CI
     (the runner is ephemeral) but lets future "self-hosted runner"
     migrations work without rework.

4. Verify all action pins match the table in `design.md`. Re-run the
   `gh api .../releases/latest` calls at slice start (lesson 26) and
   bump if any version moved between design and implementation.

### `Taskfile.yml`

5. **Probably no changes required.** Audit `task e2e:up` and its
   children for any assumption that a TTY is attached or that a
   developer-side log tail is running (it shouldn't — they all use
   `nohup ... &` already per the dev-stack contract). If anything
   blocks on user input or tails logs, fix it inline (one-shot
   change with rationale in the commit body, lesson 25).

### `frontend/playwright/e2e/auth.spec.ts`

6. **No code change required.** The skip predicate
   `!process.env.VITE_CLERK_PUBLISHABLE_KEY` becomes false once the
   workflow exports the secret. Verify the unskipped test (line
   41-57: "sign-in button renders in the header when Clerk is
   configured") actually passes against the dev tenant's
   publishable key. If the test fails for a content reason
   (selector drift, button not yet rendered on cold load), file an
   issue and `.skip` *that specific test* with a tracking link;
   do not soften the gate (decision 10).

### `.github/dependabot.yml` (if exists) and Actions repo settings

7. **Out-of-band step (not file edit) — flag in slice handoff.** The
   secrets `VITE_CLERK_PUBLISHABLE_KEY` and `CLERK_SECRET_KEY` must
   be added in BOTH the Actions secret scope AND the Dependabot
   secret scope. This is a click-through in the GitHub repo
   settings → Secrets and variables → Actions / Dependabot.
   Per CLAUDE.md lesson 21, this is *not* a tracked code slice —
   capture it as a one-line callout in the slice's PR description
   and in `docs/runbooks/admin-auth-setup.md` (or create that
   runbook line if it doesn't exist) so the human ops side has a
   record. Do NOT block the PR on it; do test that the secrets are
   present by running the workflow on the PR branch.

## Gate criteria

The slice is done when ALL of the following hold:

- [ ] `frontend-integration` and `frontend-e2e` jobs exist in
      `.github/workflows/ci.yml`.
- [ ] Both jobs are listed in branch protection's required-checks
      list (out-of-band repo settings step — verify via
      `gh api repos/:owner/:repo/branches/main/protection
      --jq '.required_status_checks.contexts'`).
- [ ] A smoke PR (any trivial change, e.g., a one-line
      docs-only diff) shows both jobs running and green.
- [ ] The "sign-in button renders in the header when Clerk is
      configured" test runs (not skipped) in the e2e job's output.
- [ ] Concurrency group is set with `cancel-in-progress` only
      on PR runs.
- [ ] On a deliberately-broken PR (e.g., a temporary commit that
      fails one e2e spec), failure artifacts are uploaded and contain
      `playwright-report/index.html`, `test-results/`, and (e2e job
      only) `logs/backend.log` + `logs/frontend.log`.
- [ ] Both jobs fall under 20 min wall-clock time on a cold cache.

## Verification Checklist (Phase Close)

Per CLAUDE.md lesson 27 — every item empirically checkable.

1. **Workflow file lints clean.**
   `actionlint .github/workflows/ci.yml` returns no errors.

2. **Concurrency block present.**
   `grep -nE "concurrency:|cancel-in-progress:" .github/workflows/ci.yml`
   shows both lines, with `cancel-in-progress: ${{ github.event_name
   == 'pull_request' }}`.

3. **Two new jobs present.**
   `grep -nE "^\s+(frontend-integration|frontend-e2e):"
   .github/workflows/ci.yml` returns two matches.

4. **Action versions pinned to verified values.**
   `grep -nE "uses:.*@v" .github/workflows/ci.yml | grep -E
   "(actions/(checkout|setup-go|setup-node|cache|upload-artifact)|
   arduino/setup-task)"` shows pins matching `design.md`'s table at
   slice-execution time (re-verified per lesson 26).

5. **Secrets referenced, not hardcoded.**
   `grep -nE "VITE_CLERK_PUBLISHABLE_KEY|CLERK_SECRET_KEY"
   .github/workflows/ci.yml` shows only `${{ secrets.* }}`
   references; no plaintext values.

6. **No prod-tenant secrets named.**
   `grep -nE "PROD_CLERK|PRODUCTION_CLERK" .github/workflows/ci.yml`
   returns empty (decision 5 / grill finding 1 mitigation).

7. **Failure-only artifact upload.**
   `grep -nB2 "upload-artifact" .github/workflows/ci.yml | grep -E
   "if: failure"` shows the conditional on every upload-artifact
   step.

8. **`task e2e:up` invoked, not `task dev:up`.**
   `grep -nE "task (e2e|dev):up" .github/workflows/ci.yml` shows
   only e2e variants in the new jobs.

9. **Auth test no longer skipped in CI logs.**
   On the smoke PR, the e2e job's stdout for "sign-in button
   renders in the header when Clerk is configured" shows a passing
   result (not "skipped").

10. **Required-checks updated.**
    `gh api repos/:owner/:repo/branches/main/protection --jq
    '.required_status_checks.contexts'` includes
    `"frontend-integration"` and `"frontend-e2e"` (or the
    workflow-level names GitHub reports — confirm by inspecting
    one finished CI run's check-suite first, then add).

11. **Tasks.md status row flipped.**
    `grep -E "^\| — \| E2E in CI .* \| \[x\] \|$"
    openspec/changes/e2e-in-ci/tasks.md` returns the row with
    `[x]`.

12. **Runbook captures the dashboard step.**
    `grep -nE "VITE_CLERK_PUBLISHABLE_KEY|CLERK_SECRET_KEY"
    docs/runbooks/admin-auth-setup.md` includes a line about
    adding both to Actions + Dependabot scopes (decision 5 +
    lesson 21 — operational detail belongs in the runbook).

## Cross-references

- Spec: `openspec/changes/e2e-in-ci/specs/ci-workflow.md` (acceptance
  criteria CW-01..CW-N).
- Design: `openspec/changes/e2e-in-ci/design.md`.
- Stress-test: `openspec/changes/e2e-in-ci/design-grill-summary.md`.
