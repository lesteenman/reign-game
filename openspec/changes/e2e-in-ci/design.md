# E2E in CI — design

## Locked decisions (post Round 3)

| # | Decision | Locked answer |
|---|---|---|
| Scope | Slice 1 wires both Playwright projects + adds `VITE_CLERK_PUBLISHABLE_KEY` to CI to unskip the 4th auth test (sign-in button render) | YES |
| 1 | Workflow placement | (b) — new job(s) in `ci.yml` |
| 2 | Integration vs e2e split | (ii) — two separate jobs (`frontend-integration` + `frontend-e2e`) |
| 3 | Path filtering | (p1) — every PR for first month |
| 4 | LocalStack approach | (L2) — `docker compose up localstack` reusing existing config + init script |
| 5 | Clerk secrets | (a) — mirror dev-tenant `VITE_CLERK_PUBLISHABLE_KEY` + `CLERK_SECRET_KEY` in BOTH Actions and Dependabot scopes |
| 6 | Lifecycle reuse | Reuse `task e2e:up` directly (NOT `dev:up`) |
| 7 | Caching | Go modules + npm + Playwright browsers; skip LocalStack image + Go build |
| 8 | Failure artifacts | Failure-only, 7-day retention; e2e includes `logs/*.log` |
| 9 | Concurrency | Cancel-in-progress on PR only |
| 10 | Required check | Required from day one |

## Per-decision rationale

### 1. Workflow placement — `ci.yml`

Single workflow file already groups Backend / Frontend / Security /
Terraform Plan. Splitting CI across multiple files would force the
required-check list to track per-workflow names, increasing branch
protection drift risk. Keep the gate consolidated where the team
already looks for it.

### 2. Two separate jobs

Integration tests have no backend dependency and complete in seconds;
e2e tests require LocalStack + e2e backend + Vite proxy and take
longer. Separating gives clean failure localization (the artifact set
on a failed `frontend-integration` run is just the report, not the
backend logs that would clutter triage). Per-job timeout budgets can
also diverge: integration ~5 min, e2e ~15 min.

### 3. Every PR for first month

Path filters look attractive but the usual failure mode in
Playwright-in-CI is *unrelated* infra: Node version bumps, Playwright
browser cache invalidation, an updated LocalStack image regressing
SQS or DDB. Path filtering on `frontend/**` would skip exactly the
runs that catch those upstream regressions. Re-evaluate after a
month with concrete data on which PRs triggered each job.

### 4. `docker compose up localstack` (L2)

The init script `.localstack/init-aws.sh` is the contract. Anything
that re-implements it in CI YAML diverges from local; lesson 14
applies. Same image (pinned to `localstack/localstack:4.14.0` per
lesson 22), same init script, same readiness contract.

### 5. Clerk secrets in BOTH scopes

`VITE_CLERK_PUBLISHABLE_KEY` is publishable (safe to embed in client
bundles by design); `CLERK_SECRET_KEY` is secret. Dependabot PRs
need both because the e2e job runs against a Dependabot-bumped
`@clerk/clerk-react`, and the secret gate triggers `permissions:
secrets: read` for that scope. Mirroring keeps Dependabot's
`frontend-e2e` job from silently skipping with a "secret missing"
warning, which would amount to lesson 13 in PR form.

### 6. `task e2e:up` directly

Already wires LocalStack readiness, e2e backend boot, Vite proxy,
fixture seeding, and a `/api/config/modes` warmup that defuses
KI-022's cold-start path (5+ s LocalStack DDB first-call). `dev:up`
is the wrong target — it boots services on the dev ports and uses
the dev `puzzle-pool` table; e2e tests assume :5182 / :5183 and
`puzzle-pool-e2e`. Reusing the local lifecycle keeps CI ↔ local
parity (lesson 14) and avoids re-implementing readiness logic.

### 7. Cache Go modules + npm + Playwright browsers

- Go modules: `actions/setup-go@v6` already does this via
  `cache-dependency-path: backend/go.sum`. Free.
- npm: `actions/setup-node@v6` already does this via
  `cache-dependency-path: frontend/package-lock.json`. Free.
- Playwright browsers: `actions/cache@v5.0.5` keyed on
  `frontend/package-lock.json` hash. Browser download is the slowest
  step (~30 s cold). High value.
- LocalStack image: skip. Pulling `localstack/localstack:4.14.0` is
  ~10 s on a warm GH runner; cache infrastructure is more code than
  the savings warrant.
- Go build: skip. `go build` is fast against a populated module
  cache; `actions/cache` for build artifacts has historically caused
  more cache-corruption issues than it has saved.

### 8. Failure-only artifacts, 7-day retention

`if-no-files-found: ignore` + `if: failure()` keeps the job
artifact-clean on green runs. Seven days matches GH defaults and is
plenty for "the PR author needs to look once". E2e job uploads
`frontend/playwright-report/`, `frontend/test-results/`, and
`logs/backend.log` + `logs/frontend.log`. Integration job uploads
only the Playwright artifacts (no backend logs to surface).

### 9. Cancel-in-progress on PR only

The idiom:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

PR runs cancel on push (rapid feedback, avoid wasted minutes); main
runs never cancel (we want every merged commit to leave a green CI
run on the timeline). Workflow currently triggers only on
`pull_request`, so the `main` branch of the conditional is dormant
today, but written this way it stays correct if `push: branches:
[main]` is added later.

### 10. Required from day one

No soak window. If `frontend-e2e` is too flaky to enforce, the
correct response is to make it deterministic (more retries, a
longer timeout, fixture pool size = 2, mark a specific spec
`.skip` with an issue) — not to leave the gate optional and let
new tests land while the existing ones are red. The whole point
of the slice is that local-only enforcement produced PR #86's
drift; "required-but-not-blocking" reproduces that failure mode.

## Shape α vs Shape β

The two-job-split decision (#2) above commits to **Shape α**. This
section captures the alternative for review traceability.

**Shape α** — two top-level jobs (`frontend-integration` +
`frontend-e2e`) in `ci.yml`, alongside the existing Backend /
Frontend / Security / Terraform Plan jobs. Total 6 jobs.

**Shape β** — fold `frontend-integration` into the existing
`Frontend` job as one extra step at the end (after `npm test`). Adds
only one new e2e job. Total 5 jobs.

| Criterion | Shape α | Shape β |
|---|---|---|
| Cache hit rate | Two `setup-node` invocations both hit the npm cache (free); identical | Saves one `setup-node` invocation; marginal seconds |
| Parallelism | Integration runs in parallel with `Frontend` build/unit | Integration serialized after `Frontend` build/unit |
| Diff size / review risk | Two new jobs, no edits to existing `Frontend` job | Edits the existing `Frontend` job — slightly riskier review |
| Failure isolation | Each job has its own artifact set; report on integration failure is just the report | Combined job artifact mixes unit-test failures with Playwright failures |
| Required-check granularity | Branch protection can require/un-require each independently | Coupled — required `Frontend` now means both unit + integration |
| Wall-clock | Integration finishes ~30 s sooner (parallelism) | `Frontend` finishes ~30 s later (serialized) |
| Resource cost | Two runners, one of which is short-lived | One runner, slightly longer |

**Verdict: Shape α.** The dominant arguments are failure isolation
(independent artifact sets) and branch-protection granularity
(decision 10's "required from day one" benefits from the option to
toggle them independently if a category turns flaky). Cache and
parallelism are second-order. The runner cost (~30 s extra runner
minutes) is negligible against the operability win.

## Design-grill summary

See `design-grill-summary.md`. Short version: three findings worth
flagging in the implementation, none blocking.

1. **Clerk-in-Dependabot not actually a privilege escalation.** Dev
   tenant tokens cannot reach prod backend; verified via the
   tenant-isolation contract (separate Clerk instance, separate
   `CLERK_PUBLISHABLE_KEY`, prod backend rejects unknown issuers via
   the JWKS check). Mitigation: ensure prod-tenant secrets are NOT
   added to Dependabot scope; explicit exclusion in the secret-add
   step.
2. **Most-likely CI failure is order-of-operations between
   LocalStack init and e2e backend startup.** `task e2e:up` already
   handles this locally, but CI's faster I/O can still surface a
   race if `init-aws.sh` finishes after the backend's first DDB
   call. Mitigation: `task e2e:up` already polls LocalStack health
   AND the init script (lesson visible in `dev:up:localstack`); the
   same gating must be observed for e2e. Verify in implementation
   that the `e2e:up:localstack` (or whatever boots LocalStack from
   the e2e path) waits for both signals.
3. **Playwright browser cache invalidates on any `package-lock.json`
   change.** Worst case is a one-line `package-lock.json` churn
   forcing a full browser re-download (~30 s). Acceptable. The
   alternative (key on `playwright-core` version specifically)
   risks staleness when Playwright bumps its bundled browser
   binaries without a `playwright-core` major bump.

## Verified action versions (2026-04-30 14:40 UTC)

All versions verified against the live GitHub releases API at the
timestamp above (lesson 26 — verify external dependency versions at
the source of truth, not from memory).

| Action | Pin | Released | Notes |
|---|---|---|---|
| `actions/checkout` | `@v6.0.2` | 2026-01-09 | Already in repo |
| `actions/setup-go` | `@v6.4.0` | 2026-03-30 | Already in repo |
| `actions/setup-node` | `@v6.4.0` | 2026-04-20 | Already in repo |
| `actions/cache` | `@v5.0.5` | 2026-04-13 | New for Playwright browsers |
| `actions/upload-artifact` | `@v7.0.1` | 2026-04-10 | New for failure artifacts |
| `arduino/setup-task` | `@v2.0.0` | (verified earlier today) | Already in repo |

Verification commands (re-runnable on slice start):

```bash
gh api /repos/actions/checkout/releases/latest --jq '.tag_name + " " + .published_at'
gh api /repos/actions/setup-go/releases/latest --jq '.tag_name + " " + .published_at'
gh api /repos/actions/setup-node/releases/latest --jq '.tag_name + " " + .published_at'
gh api /repos/actions/cache/releases/latest --jq '.tag_name + " " + .published_at'
gh api /repos/actions/upload-artifact/releases/latest --jq '.tag_name + " " + .published_at'
```

The implementation agent (devops-engineer) MUST re-run these at the
moment of slice execution and update this table if any pin moved.
Lesson 26 explicitly calls out re-verification when the slice that
installs the dependency starts.
