# Design Grill Summary

A second-pass stress test of the 14 locked decisions and the 4 new specs. Each finding lists severity (info / risk / blocker) and the resolution (accept / mitigate / defer).

## Findings

### F1. Q-A=(a) — empty-pool 404 behavior is unverified

**Severity:** risk.
**The grill.** We locked Q-A=(a): document the current 404 surface as the empty-pool fallback. Nobody on this team has actually exercised that code path in production conditions. The 404 might render as a blank screen, a generic browser error page, or an unstyled crash — any of which is worse for a player than a polite "no puzzles right now" message. The spec is asserting a behavior we haven't confirmed is acceptable.
**Resolution:** mitigate. The new `pool-empty-fallback.spec.ts` does NOT just assert "404 returned"; it also screenshots the player-facing surface and asserts the page contains *some* user-readable text (not blank, not a stack trace). If the screenshot review during PR shows the surface is genuinely user-hostile, we open a follow-up slice for a styled empty state — but we do not block this slice on it. Documenting current behavior is itself a win because it makes the next regression visible.

### F2. Generator-in-e2e first-poll race

**Severity:** risk.
**The grill.** `task e2e:up:generator` starts the generator after LocalStack is "ready", but the readiness probe only confirms the queue exists, not that the generator's first SQS long-poll has connected. A spec that sends a replenishment trigger immediately after `e2e:up` returns may fire before the generator is actually listening, miss the message, and time out. Symptom in CI: flaky `pool-replenishment.spec.ts`.
**Resolution:** mitigate. The generator readiness probe waits for the existing `"starting local SQS poller"` log line (same gate as `dev:up:generator`). That log line is emitted *after* the SQS client opens the long-poll, so it's a true readiness signal, not just a process-started signal. Spec-side, `pool-replenishment.spec.ts` also seeds the trigger via DDB write rather than SQS so the test isn't sensitive to message-delivery ordering. Belt + braces.

### F3. Test-admin credentials and Dependabot scope

**Severity:** info.
**The grill.** Three new repo secrets land: `E2E_CLERK_PUBLISHABLE_KEY`, `E2E_CLERK_TEST_ADMIN_EMAIL`, `E2E_CLERK_TEST_ADMIN_PASSWORD`. GitHub by default does NOT expose secrets to PRs from forks, but Dependabot PRs run with a separate `dependabot` secret scope. If the e2e job is configured to run on Dependabot PRs and we forget to mirror the secrets to that scope, Dependabot e2e runs will fail mysteriously. Conversely, if we DO mirror, the credentials are slightly more exposed than necessary.
**Resolution:** accept (with explicit choice). The runbook (`docs/runbooks/e2e-clerk-setup.md`) names this trade-off and recommends NOT mirroring to Dependabot — Dependabot PRs run the unit suite but the e2e job is configured to skip on `actor == 'dependabot[bot]'`. Blast radius of credential exposure is dev-tenant-only (separate Clerk instance from prod), but smaller surface is still better. Manual e2e re-run on a Dependabot PR is a one-click "Re-run jobs" by a maintainer if needed.

### F4. 30 s polling timeout vs CI generation latency

**Severity:** risk.
**The grill.** Locked decision used 30 s as the spec-side polling budget for the generator to top up a low pool. Local generation is sub-second; CI cold-start with LocalStack is materially slower, and we have no measurement of p95 generation latency in the CI environment specifically. A 30 s budget that's fine 95% of the time and times out 5% of runs is the worst kind of flakiness — frequent enough to erode trust, rare enough to dismiss as "transient".
**Resolution:** mitigate. Set the polling budget at 60 s for the CI run and 15 s for local. Measure actual p95 in CI over the first 10 runs after merge; if p95 < 10 s, drop CI to 30 s in a follow-up. If p95 > 30 s, the slice produced data we wouldn't otherwise have, and we can decide whether to optimize the generator or accept the longer budget. Either way, the polling budget lives in one constant in `playwright.config.ts` so future tuning is one diff.

### F5. Spec ordering — state mutation across specs

**Severity:** risk.
**The grill.** Three of the four new specs mutate the puzzle-pool state: replenishment seeds a low pool, served-marking flips a flag, empty-pool drains the pool. If Playwright runs them in parallel (default) or in the wrong serial order, one spec's setup is another's teardown. Symptom: order-dependent flakiness that's nearly impossible to debug from CI logs alone.
**Resolution:** mitigate. Group all pool-mutating specs into a Playwright `test.describe.serial` block (or assign them to a single project that runs `workers: 1`). Each spec's `beforeEach` re-seeds the pool from a fixture so the spec is independent of the previous spec's final state. Costs ~2 s per spec for re-seed; buys back a class of flakiness we're better off never having.

### F6. 7×7 Double sentinel — what if KI-007 regresses to feasible?

**Severity:** info.
**The grill.** Decision #6 retargets the dynamic-modes admin spec at `7#double=false` because KI-007 currently rules generation infeasible. If the generator is later improved (or the difficulty constraints relaxed) so 7×7 Double becomes feasible, the spec asserting "this combo is disabled" becomes a *false negative* — the test still passes, but for the wrong reason, and we lose confidence that admin enable/disable actually works for that combo.
**Resolution:** accept. The spec's assertion is "this specific combo renders disabled and rejects activation"; the *general* admin enable/disable flow is covered separately in `admin-config-flow.spec.ts` against a different combo. Even if 7×7 Double becomes feasible later, the assertion remains literally true (we still ship it disabled by sentinel) until KI-007 is closed and the sentinel is removed in a follow-up. At that point the test gets retargeted, and that retargeting is a single-line diff.

## Summary

- 1 finding accepted as documented trade-off (F3, F6).
- 4 findings mitigated within this slice (F1, F2, F4, F5).
- 0 blockers.

Proceed to spec generation.
