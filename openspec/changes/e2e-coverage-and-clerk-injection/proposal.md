# Proposal: E2E Coverage Expansion + Clerk Admin Session Injection

## Status

Draft. Between-phase work, no R-ID per CLAUDE.md lesson 18 (only the current phase gets numbered IDs).

## Context

Slice 1 of the e2e-in-CI initiative just merged: Playwright e2e suite now runs in CI against a LocalStack-backed dev stack, with the puzzle generator excluded and admin/auth specs `test.skip`'d. That win established the lifecycle plumbing (`task e2e:up` / `task e2e:down`) and proved CI can host a deterministic backend, but it left two gaps:

1. **Coverage gap.** Pool replenishment, served-marking, pool-empty fallback, and admin config flows have no e2e tests. These are exactly the paths where bugs slipped through unit-only coverage in R-7-02 and the R-8x admin slices. Nothing in CI fails today if a regression breaks them.
2. **Auth gap.** The 4 currently-skipped specs (3 in `auth.spec.ts`, 1 in `dynamic-modes.spec.ts`) need an authenticated admin session to run. Slice 1 deferred this because Clerk session injection in headless Playwright is not trivial.

This change combines what was originally drafted as slices 2 (coverage) and 3 (Clerk injection) of the e2e-in-CI plan. They are merged because the new `admin-config-flow.spec.ts` requires an authenticated session anyway, so splitting the slices would force a no-value intermediate state where coverage exists but half of it can't actually run.

## Why now

- Slice 1's lifecycle plumbing is fresh in everyone's head — adding the generator lifecycle and Clerk wiring on top is cheaper now than after a context switch.
- The 4 skipped specs are silent debt. Each week they stay skipped, the team gets used to "skipped is normal" and the failure mode of accidental skips becomes harder to spot in PR review.
- Pool replenishment and served-marking are load-bearing for the player experience. The longer they go without e2e coverage, the higher the chance the next puzzle-pool change ships a regression that only surfaces in production.

## What changes

This slice delivers, in one branch:

- **Lifecycle.** Add `task e2e:up:generator` / `task e2e:down:generator` and wire them into `task e2e:up` / `task e2e:down`. Update `.localstack/init-aws.sh` to provision the e2e SQS queue (`puzzle-generation-e2e`) and the e2e CONFIG flips needed for replenishment specs.
- **Fixtures.** Extend `backend/cmd/genfixtures/main.go` with the seed shapes the new specs need (replenishment trigger states, served-pool states, empty-pool states, admin-config baseline).
- **Clerk wiring.** A Playwright `globalSetup` script signs in a dedicated test admin user via Clerk's testing-token flow and persists the session to a storage-state file that all admin specs consume. Six new env vars (locked in chunk 4 — admin and user passwords are distinct): `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `E2E_CLERK_TEST_USER_EMAIL`, `E2E_CLERK_TEST_USER_PASSWORD`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. CI secrets and a runbook (`docs/runbooks/e2e-clerk-setup.md`) cover the manual dashboard prep.
- **Skipped tests unblocked.** All 3 skipped tests in `auth.spec.ts` plus 1 in `dynamic-modes.spec.ts` have `test.skip` removed and run against the injected admin session. The `dynamic-modes` test is re-targeted at the `7#double=false` sentinel per locked decision Q-A=(a) — see design.md.
- **Four new specs.** `pool-replenishment.spec.ts`, `served-marking.spec.ts`, `pool-empty-fallback.spec.ts`, `admin-config-flow.spec.ts`.
- **CI.** `.github/workflows/ci.yml` injects the new secrets into the e2e job.

## Locked decisions

The design-grill pass produced 14 locked decisions plus the Q-A resolution (`(a)` — keep `7#double=false` as the always-disabled sentinel). The full table lives in `design.md`. Highlights:

- Per the design's locked decisions, the dynamic-modes admin spec retargets at `7#double=false` rather than re-enabling it, which would require the underlying generator infeasibility (per KI-007) to be fixed first.
- Generator runs in the e2e stack on a separate queue (`puzzle-generation-e2e`) so dev and e2e never cross-contaminate.
- Clerk session is injected via storage-state, not API-mocked, so the auth path stays exercised end-to-end.
- Two dedicated test users (admin + non-admin) live in the same Clerk dev tenant as local development; production tenant is untouched. See design.md D2.

## Out of scope

- Production Clerk tenant rotation (post-launch operational task) — stays a "click in a third-party UI" task per CLAUDE.md lesson 21. The dev-tenant scoping in this slice is what makes the eventual rotation safe.
- Re-enabling 7×7 Double in the generator — KI-007 still applies; this slice routes around it.
- Visual regression coverage — different problem class, separate slice when prioritized.
