# Design Grill Summary — E2E in CI (slice 1)

Single-pass stress test of the 10 locked decisions. Three areas
prioritized per the Round-3 dispatch: Clerk-in-Dependabot security,
"required from day one" reliability, Playwright browser cache
invalidation. Each finding includes severity (BLOCKING / HIGH /
MEDIUM / LOW / NOTE), the assumption being challenged, the
counter-argument, and the resolution as it lands in the spec.

## Finding 1: Clerk-in-Dependabot — privilege escalation path?

**Severity:** MEDIUM (mitigated; mitigation explicit in spec).

**Assumption challenged:** Decision 5 mirrors `VITE_CLERK_PUBLISHABLE_KEY`
and `CLERK_SECRET_KEY` into the Dependabot scope. Could a malicious
Dependabot PR (or a leaked dev secret) be used to mint user tokens
that hit the prod backend?

**Walk the tree:**

1. Dependabot PRs do NOT have write access to the repo by default —
   they cannot exfiltrate secrets via workflow modification on the
   PR branch. (GitHub explicitly hardens this since 2021.)
2. The dev-tenant `CLERK_SECRET_KEY` cannot be used to authenticate
   against the prod-tenant Clerk instance — they are separate
   tenants with separate JWKS. A token signed with the dev tenant's
   keys fails JWKS verification at the prod backend boundary.
3. The prod backend's auth middleware uses the
   prod `CLERK_SECRET_KEY` for verification (not the dev one), and
   `clerk-sdk-go/v2`'s default `clerk.SetKey` only accepts tokens
   signed by the corresponding tenant's key set. Cross-tenant
   forgery requires breaking Clerk itself, not us.
4. The `VITE_CLERK_PUBLISHABLE_KEY` is publishable by design — it's
   shipped in the client bundle. No secret value to protect.

**Counter:** What if someone adds the *prod* Clerk keys to the
Dependabot scope by mistake when configuring? Then a Dependabot PR's
e2e run could sign tokens that the *prod* backend would accept —
that's not exploitable from the Dependabot PR (no shell access to
the dev backend, only the LocalStack-hosted CI backend), but it does
violate the principle.

**Resolution (lands in spec as CW-12):** Spec explicitly forbids
prod-tenant Clerk keys in Dependabot scope. Verification: the
runbook entry for adding the secrets must call out "dev tenant ONLY
for Dependabot." If a prod key were ever needed for some Dependabot
test (it isn't), it would require its own dedicated review.

**Verification check on slice close:** `gh api
repos/:owner/:repo/dependabot/secrets --jq '.secrets[].name'` lists
only `VITE_CLERK_PUBLISHABLE_KEY` and `CLERK_SECRET_KEY` (no prod
variants).

## Finding 2: "Required from day one" — most likely failure mode?

**Severity:** HIGH (operational; no spec change but an
implementation note).

**Assumption challenged:** Decision 10 makes the e2e job a
required check immediately, no soak window. What's the most likely
reason CI e2e fails in practice during the first week?

**Walk the tree:** Three candidates considered.

1. **Order-of-operations between LocalStack init and backend
   startup.** The init script seeds DDB tables and SQS queues. The
   e2e backend's first DDB call panics if it lands before the
   `puzzle-pool-e2e` table is `ACTIVE`. Locally this is rare
   because dev runs in slow human-time; in CI on a fast SSD runner
   the gap shrinks and races become probable. **This is the most
   likely first-week failure.**

2. **LocalStack 4.14.0-pinned-image quirks.** Lesson 22 documents
   the `:latest` regression that motivated the pin. The pinned
   version is known-good, but its DDB cold-start (~5 s on first
   call, KI-022) is still long. The `task e2e:up` warmup against
   `/api/config/modes` defuses this — but only if the warmup
   actually runs before the test calls hit. Less likely than #1
   because the warmup is already in `e2e:up`.

3. **Playwright `webServer` config conflict.** The current config
   has `webServer.command: "npm run dev"` with `port: 5180` and
   `reuseExistingServer: !process.env.CI`. In CI, `process.env.CI`
   is true, so `reuseExistingServer` is false — Playwright would
   try to spawn its own `npm run dev` on :5180 even though the e2e
   project's baseURL is :5183. Locally this is masked because
   `task e2e:up` already runs the right Vite. In CI this could
   collide if the test runner expects to spawn but
   `npm run test:e2e` doesn't disable the global webServer hook.

**Resolution:** Findings 2 and 3 land as implementation
gotchas, not spec text:

- For #1: tasks.md item 3 explicitly says "Wait for LocalStack
  readiness AND the init script's completion before any e2e
  backend boot. Reuses the same readiness contract as `task
  dev:up:localstack`." The implementation agent must verify the
  `e2e:up` lifecycle gates on both signals (likely already does;
  re-confirm).
- For #3: tasks.md item 5 is an audit checkpoint — read
  `playwright.config.ts` and either disable the `webServer` block
  for the e2e project (per-project override) or set
  `reuseExistingServer: true` unconditionally for the e2e project.
  Local-dev semantics (auto-spawn Vite) and CI semantics
  (pre-spawned by `task e2e:up`) differ; the config must reflect
  that.

**Open question for the implementation agent (not blocking
design):** Does `npm run test:e2e` already work locally because
`task e2e:up` happens to listen on the right ports first, or
because Playwright's `webServer` block is bypassed by some other
mechanism? Confirm before assuming the same will hold in CI.

## Finding 3: Playwright browser cache — worst-case dep bump?

**Severity:** LOW (acceptable risk, no spec change).

**Assumption challenged:** Decision 7 keys the Playwright browser
cache on `hashFiles('frontend/package-lock.json')`. What's the
worst case for an unrelated dep bump?

**Walk the tree:** A Dependabot PR bumping any frontend package
(say, `@types/react` patch release) changes `package-lock.json`'s
hash. The cache key changes. The cache miss forces a fresh
chromium download (~80 MB, ~30 s on a warm GH runner). On every
unrelated bump until the lockfile stabilizes again. Worst case:
Renovate-style PR storm with 5 lockfile changes per day → 5x
30 s = 2.5 min/day of wasted runner time.

**Counter:** Why not key on the `playwright-core` version
specifically? Risk: Playwright bumps its bundled browser binaries
without a `playwright-core` major bump (they do this for security
patches). A version-keyed cache would silently serve a stale
browser binary, masking issues that real users would hit. The
lockfile hash is conservative-correct: any code change that could
plausibly affect browser behavior invalidates the cache.

**Resolution:** No spec change. Accept the ~30 s/dep-bump cost.
If it becomes painful (>5 min/day measured), revisit with a
narrower key. Document the tradeoff in `design.md` (which already
includes this in the per-decision rationale for #7).

## Finding 4: `task e2e:up` lifecycle — TTY assumptions?

**Severity:** LOW (verified clean by inspection; flagged for
re-verification).

**Assumption challenged:** Decision 6 reuses `task e2e:up` as-is.
The dev-stack contract in CLAUDE.md emphasizes `nohup ... &` and
explicit readiness probes. Does any sub-task of `e2e:up` accidentally
require a TTY or assume an interactive shell?

**Walk the tree:** Read of `Taskfile.yml`'s `e2e:up:` block (lines
697-718) shows:

- `e2e:up:backend` (line 566)
- `e2e:up:frontend` (line 614)
- `e2e:seed`
- A curl warmup
- A status echo

No `read`, no interactive prompts, no `docker compose up` (without
`-d`), no `tail -f`. The lifecycle pattern matches `dev:up`, which
is known-CI-clean (the existing Frontend job's `npm test` runs
without the dev stack, so this isn't tested in CI yet, but the
pattern is intact).

**Resolution:** No spec change. tasks.md item 5 already calls for an
audit before assuming clean.

## Finding 5: Required-check name brittleness

**Severity:** MEDIUM (operational; documented in tasks.md).

**Assumption challenged:** Branch protection requires checks by
*display name*, not by `jobs.<job_id>:` key. If a job's `name:`
field is omitted, the displayed name defaults to the job_id —
adding a `name:` later renames it in branch protection's view.

**Resolution:** tasks.md item 2 and 3 specify both `jobs.<id>:` and
the resulting display behavior; verification step CW-31 explicitly
inspects `gh api .../protection`. The implementation agent should
add a `name:` field to each new job to lock the display string —
e.g. `name: Frontend Integration` and `name: Frontend E2E` — and
add those exact strings to the required-checks list. Renaming
later requires a coordinated branch-protection update, which is
visible in repo audit logs.

## Finding 6: `secrets.*` availability on Dependabot PRs

**Severity:** LOW (mitigated by decision 5).

**Assumption challenged:** GitHub split secret scopes specifically
because Dependabot PRs were a vector for exfiltration. Decision 5
explicitly mirrors the Clerk secrets — but we should verify that
the e2e job actually fails fast on Dependabot PRs if mirroring was
forgotten (rather than silently skipping the auth test).

**Resolution:** No spec change. The auth smoke test asserts
visibility of `data-testid="sign-in-button"`. If `VITE_CLERK_PUBLISHABLE_KEY`
is absent on a Dependabot PR (mirroring failure), the test won't
skip — `process.env.VITE_CLERK_PUBLISHABLE_KEY` will be empty
string, which is truthy in `!process.env.VITE_CLERK_PUBLISHABLE_KEY`
... actually, empty string is falsy in JS. The
`needsClerk` flag would be `true` if the env var is unset *or*
empty, and the test would skip. **This is a hole.** If
mirroring fails, the test silently skips, returning us to the
exact drift state PR #86 caught.

**Updated resolution:** Add to tasks.md gate criterion: "On the
smoke PR, the e2e job's stdout for the auth smoke test shows a
PASSING result (not 'skipped')." Already present in tasks.md as
"Auth test no longer skipped in CI logs" (verification item 9).
Reinforced.

## Decisions table — open vs resolved

| # | Decision | Status |
|---|---|---|
| Scope | Wire both projects + add Clerk env | Resolved |
| 1 | Workflow placement (ci.yml) | Resolved |
| 2 | Two separate jobs | Resolved |
| 3 | Path filtering (every PR for 1mo) | Resolved |
| 4 | LocalStack via docker compose | Resolved |
| 5 | Clerk secrets in BOTH scopes (dev only) | Resolved + mitigation captured (CW-12) |
| 6 | `task e2e:up` reuse | Resolved + audit task captured |
| 7 | Caching strategy | Resolved + tradeoff documented |
| 8 | Failure-only artifacts, 7 days | Resolved |
| 9 | Cancel-in-progress on PR only | Resolved |
| 10 | Required from day one | Resolved + first-week reliability flagged in finding 2 |

All ten decisions resolved. No design questions outstanding for
the user. Implementation may proceed.
