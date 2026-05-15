# Playwright suites

Two test projects live here, each matching one of the categories in the project glossary (`GLOSSARY.md` → Testing).

| Folder            | Project       | What it talks to                                            |
|-------------------|---------------|-------------------------------------------------------------|
| `integration/`    | `integration` | Frontend with `page.route` mocks — no backend required.     |
| `e2e/`            | `e2e`         | Real backend + real LocalStack, against seeded fixtures.    |

Playwright is the tool in both cases; the category comes from what the test actually exercises.

## Project configuration

Both projects are defined in `frontend/playwright.config.ts`:

- **`integration`** — Runs against the normal Vite dev server on `:5180` (`baseURL: http://localhost:5180`). Specs intercept `/api/*` via `page.route(...)` and return mocked JSON, so the dev server is the only running dependency. Defaults to chromium, parallel.
- **`e2e`** — Runs against the **e2e Vite on `:5183`** (`baseURL: http://localhost:5183`), which proxies `/api/*` to the **e2e backend on `:5182`** reading **`puzzle-pool-e2e`** in LocalStack. Serial (`fullyParallel: false, workers: 1`), no retries. The fixture pool holds one puzzle per `(size, mode)`, so parallel workers would race for the single fixture.

The shared `webServer` block (top-level, not per-project) spawns `npm run dev` on `:5180` for the integration project; the e2e npm script sets `PLAYWRIGHT_SKIP_WEBSERVER=1` so Playwright does NOT spawn a redundant Vite for the e2e job (`task e2e:up` already provides Vite on `:5183`).

The integration job in CI intentionally omits `CLERK_SECRET_KEY` to minimize supply-chain blast radius — `global-setup.ts` accommodates this by skipping `clerkSetup()` when the secret is absent (the integration specs never call live Clerk APIs).

## Running the integration suite

No setup beyond `npm install`. The existing dev server on `:5180` is reused if running; otherwise Playwright's `webServer` config starts one.

```bash
npm run test:integration
```

## Running the e2e suite

The e2e project drives a **separate Vite on `:5183`** (proxying `/api/*` to the **separate backend on `:5182`** reading **`puzzle-pool-e2e`**). This keeps the dev stack on `:5180`/`:5181` untouched.

```bash
task e2e:up           # brings up the e2e backend + frontend and seeds fixtures
npm run test:e2e
task e2e:down         # tear down when done
```

LocalStack is shared with the dev stack — `task dev:up:localstack` works for both. The e2e isolation lives at the DynamoDB-table and backend-instance boundary.

## Running both

```bash
npm run test:playwright
```

Runs `integration` in parallel (the default), then `e2e` serially (one worker — fixture pool has one puzzle per combo).

## Test helpers

| File | Role |
|---|---|
| `test-helpers/clerk.ts` | `signInAsAdmin(page)` and `signInAsUser(page)`. Both navigate to `/` first (Clerk requires the client to be loaded before `clerk.signIn` attaches) and sign in via email-ticket (no password). Each helper validates its own env vars on first call with a fail-fast error message naming the missing var. |
| `test-helpers/admin-pool.ts` | `readCombo(request, size, mode)` — reads GET /api/admin/pool and finds the matching combo. Throws loudly when missing so fixture/seed drift fails the test rather than treating absence as `readyCount = 0`. Defines `ConfigBody`, `ComboStatus`, `AdminPoolResponse` types. |
| `test-helpers/modes.ts` | Types + `containsCombo(modes, size, mode)` predicate for GET /api/config/modes. Used by both `admin-config-flow.spec.ts` and `dynamic-modes.spec.ts`. |
| `global-setup.ts` | Runs `clerkSetup()` (from `@clerk/testing/playwright`) once per Playwright invocation when `CLERK_SECRET_KEY` is present. Validates `VITE_CLERK_PUBLISHABLE_KEY` upfront with a fail-fast error message. Test-user creds are validated lazily inside the clerk helpers — keeps the integration job runnable without the test-user vars while still surfacing missing vars loudly on the e2e job. |

## Auth flow in tests

Three things to know:

1. **Per-spec sign-in beats long-lived `storageState`.** Clerk JWTs expire after ~1h. Each admin/user-gated test calls `clerk.signIn(...)` against its own browser context (design D4 of the e2e-coverage-and-clerk-injection slice). The cost of re-signing-in is preferable to debugging late-run auth flakes.
2. **Email-ticket, not password.** `clerk.signIn({ page, emailAddress })` uses Clerk's backend SDK to mint a sign-in ticket and evaluates a ticket-strategy signIn on the page. Per `@clerk/testing`'s source, this branch waits for `window.Clerk.user` to populate before returning — unlike the password branch, which races. The `E2E_CLERK_TEST_*_PASSWORD` env vars exist but are NOT consumed by the helpers today.
3. **Cookie-jar gotcha (lesson 5 in `frontend/CLAUDE.md`).** Playwright's standalone `request` fixture and `page.request` have separate cookie jars. When a test authenticates via the browser, session cookies attach to the `page`'s `BrowserContext`. The standalone `request` fixture is a separate `APIRequestContext` and would 401 on auth-gated endpoints. **Always use `page.request.X(...)` for admin API calls**, or alias `const request = page.request` at the top of the test (see `admin-config-flow.spec.ts:98` for the documented pattern).

## Lifecycle

The e2e stack is orchestrated by Taskfile targets (run from the repo root):

```
task e2e:up              # LocalStack + e2e backend (:5182) + e2e frontend (:5183) + seeded fixtures
task e2e:down            # tear down (LocalStack stays up — shared with dev)
task e2e:seed            # re-seed committed fixture puzzles (idempotent)
task e2e:status          # show e2e backend status + fixture count
task e2e:genfixtures     # regenerate committed fixture puzzles (deterministic from fixed seeds)
task e2e:up:generator    # start e2e generator worker
task e2e:down:generator  # stop e2e generator worker
```

`task e2e:up` waits until each service is healthy before returning. Backend readiness is `/api/health`; frontend readiness is `:5183` listening; generator readiness is the "starting local SQS poller" log line.

## Fixtures

Committed under `e2e/fixtures/puzzles/*.json` as DynamoDB-Item JSON. Two fixtures with the same region map + different SKs are committed for the 7×7 Standard combo — a workaround for React StrictMode's dev-mode double-mount of `GamePage`, where the first mount's cancelled fetch still triggers a server-side `status=served` update, leaving the second mount needing a fresh fixture to avoid 404.

Regenerate after intentional generator changes:

```bash
task e2e:genfixtures
```

Deterministic: the same seed + size + k produces byte-identical output.

## Known caveats

- **First `/api/config/modes` call can be slow.** LocalStack's DynamoDB cold-path occasionally sits at 5–10 s on the first request (KI-022). `task e2e:up` warms the backend; the e2e specs bump `toHaveCount` / `toBeVisible` timeouts to 15 s as belt-and-suspenders.
- **Don't re-run `npm run test:e2e` without re-seeding.** Each run serves the fixture (two rows → `status=served` after play-to-completion). Re-run `task e2e:seed` before another pass, or `task e2e:up` which re-seeds as part of its warmup.
- **No retries on the e2e project.** A retry consumes fixture 2/2 (the StrictMode spare); a second retry hits the same no-puzzles-available path the project-level config is designed to avoid. Failures here need a real look, not a rerun.

## Current test coverage

### Integration

- **`integration/grid-interaction.spec.ts`** (303 LOC) — Mocked-backend grid mechanics: cell three-tap, drag, undo/redo, conflict highlighting, completion overlay, reset, keyboard shortcuts (Ctrl+Z / Ctrl+Shift+Z), solved-state interaction lockout.

### e2e

- **`e2e/play-to-completion.spec.ts`** — Drives a 7×7 Standard puzzle to solved against the real backend. Exercises `/api/config/modes`, `/api/puzzles/next`, cell-click, undo, redo, completion overlay.
- **`e2e/dynamic-modes.spec.ts`** — Asserts `/api/config/modes` filters by `enabled`. The e2e pool pins `9#double` to disabled; the test confirms the button does not render.
- **`e2e/auth.spec.ts`** — Anonymous header smoke + admin-route access matrix (EC-01: admin sees pool widget, non-admin sees forbidden landing) + sign-out (EC-02) + session-persists-across-reload (EC-03).
- **`e2e/admin-config-flow.spec.ts`** — Admin signs in → PUT /api/admin/config/{size}/{mode} flips the `enabled` flag on `7#double` → GET /api/config/modes reflects the change → cleanup flips it back via try/finally inside the test body.
- **`e2e/daily-flow.spec.ts`** — Daily-flow happy path (anonymous user lands on `/play?flow=daily` from the landing tile, GameBoard mounts) + KI-025 (timer does not reset on navigate-back, asserts wall-clock progress) + DP-27 short-circuit (pre-seeded IndexedDB solved row → PostCompletionScreen renders with NO `/api/daily/*` request — negative-network-call assertion).
- **`e2e/pool-empty-fallback.spec.ts`** — Empty-pool fallback UX.
- **`e2e/pool-replenishment.spec.ts`** — Admin-triggered replenishment flow.
- **`e2e/served-marking.spec.ts`** — Served-status row marking after a puzzle is fetched.
