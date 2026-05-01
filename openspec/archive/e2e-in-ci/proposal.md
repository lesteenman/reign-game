# E2E in CI — Slice 1 (proposal)

## What

Wire both Playwright projects into GitHub Actions CI so every PR runs:

1. `frontend-integration` — the integration project (mocked `/api/*` via
   `page.route`), no backend, no LocalStack.
2. `frontend-e2e` — the e2e project (real e2e backend on `:5182` proxied
   through Vite on `:5183`, LocalStack-backed DynamoDB, seeded fixture
   pool), launched via the existing `task e2e:up`.

Plumb `VITE_CLERK_PUBLISHABLE_KEY` (and `CLERK_SECRET_KEY` for the e2e
backend) through both Actions and Dependabot scopes so the auth smoke
test (`sign-in button renders…`) runs instead of `test.skip`-ing.

## Why

PR #86 (2026-04-30) caught a documented-but-unrun pipeline: the README
and the playwright config both describe these projects as part of the
gate, but neither runs in CI. The local-only contract has produced
exactly the kind of drift CLAUDE.md lessons 13–14 warn about — code
ships without the spec-described coverage actually being enforced. The
backend `dev:up` lifecycle bug in Phase 4.5 was caught only because
review-local ran the readiness probe; e2e specs that exist on disk but
are silently skipped in CI have no equivalent safety net.

This slice closes the loop: every PR that lands has been observed
booting the full stack and exercising it through the browser. From day
one, both jobs are required-checks — no soak window. If a spec flakes,
mark it `.skip` with a tracking ticket; do not soften the gate.

## Scope

**In scope (slice 1):**

- Two new CI jobs in `.github/workflows/ci.yml`:
  `frontend-integration` (Shape α) and `frontend-e2e` (Shape α).
- LocalStack started via the existing `docker compose` config + init
  script (decision 4 / L2). No new container definition.
- `task e2e:up` reused as the lifecycle entry point (decision 6).
- Caching for Go modules, npm, and Playwright browsers (decision 7).
- Failure-only artifact upload, 7-day retention (decision 8).
- `concurrency.cancel-in-progress` true on PRs, never on `main`
  (decision 9).
- Both jobs marked required from day one (decision 10).
- Add `VITE_CLERK_PUBLISHABLE_KEY` + `CLERK_SECRET_KEY` to the Actions
  + Dependabot secret scopes (decision 5). Unskip the
  `sign-in button renders in the header when Clerk is configured`
  test in `frontend/playwright/e2e/auth.spec.ts` by virtue of the env
  var now being present (no code change to that spec — the existing
  `test.skip(needsClerk, …)` predicate flips automatically).

**Out of scope (deferred to later slices):**

- Slice 2: admin/non-admin Clerk session injection via
  `@clerk/testing` and a dedicated test account. Tracks the two
  remaining skipped tests.
- Slice 3: path-filter optimization (decision 3 — current plan is to
  run on every PR for the first month, then revisit).
- Performance tuning (sharding, parallel projects in one job, etc.).

## Slice ID

This slice has no R-ID. Per the corrected ID scheme in CLAUDE.md, only
the current numbered phase carries `R-<phase>-<slice>` IDs; this is a
backlog item being pulled forward. Tracked solely by the change name
`e2e-in-ci` and a single status row in `tasks.md`.

## Motivation: cite the drift

The triggering observation, captured during PR #86 review:

> The repo ships a Playwright config with two projects (integration
> and e2e) and a fully wired `task e2e:up` lifecycle. README points
> at it. CLAUDE.md references it. Neither project runs in any CI
> workflow. Local-only enforcement is not enforcement.

Spec-implementation drift of this exact shape is what CLAUDE.md
lesson 13 ("Run review-local before `gh pr create`") and lesson 14
("Path/URL/env renames need a full-repo grep") were written to
prevent. The ROADMAP backlog item exists; this slice promotes it.
