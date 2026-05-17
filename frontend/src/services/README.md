# `src/services/`

Backend client modules. Six production files plus five unit tests.

## Responsibility

Wrap `/api/*` calls in typed functions. The shared `api.ts` exposes a thin fetch base; per-resource services compose request URLs, headers, and error semantics on top. Today services are called directly from page-level `useEffect`s and from one leaf component (`VerdictSurface`) — that latter call is the known leaf-I/O violation.

## Data flow

- **In:** Called by `pages/*` and `shared/game/` hooks and components.
- **Out:** `fetch()` to `/api/*`. Base URL is `VITE_API_URL || window.location.origin`; in dev, Vite's proxy forwards `/api/*` to `localhost:5181`.

## Files

- **`api.ts`** — Shared fetch base. Exports `apiFetch<T>(path, params?)`, `apiPut<T>(path, body, params?)`, `apiPost<T>(path, body, params?)`, and an `ApiError` class carrying `status`. The three helpers are near-identical (URL construction + headers + ok-check + body parse) — duplication called out in `frontend/FINDINGS.md`.
- **`puzzleService.ts`** — `fetchNextPuzzle(size, mode)` (GET /api/puzzles/next?size=N&mode=M) and `updatePuzzleStatus(puzzleId, size, mode, status)` (PUT /api/puzzles/{id}/status). Defines `NoPuzzlesAvailableError` thrown when the pool returns 404.
- **`verdictService.ts`** — `submitVerdict({puzzleId, size, mode, value, playTimeMs, outcome, clientVersion?})` (PUT /api/admin/puzzles/{id}/verdict?size=N&mode=M). Silently swallows 401/403 (FB-05) — the role check that rendered the surface should have prevented this, but a revoked-mid-session admin shouldn't see a scary error toast.
- **`adminService.ts`** — Pool / config CRUD: `fetchPoolStatus`, `updateConfig`, `createConfig`, `triggerReplenish`. Also re-exports `MODES` / `isMode` / `Mode` from `engine/types` (architectural smell — drive-by indirection).
- **`landingService.ts`** — `fetchEnabledModes()` (GET /api/config/modes). Public endpoint, no auth.
- **`dailyService.ts`** — `getDaily(date?)` and `submitDailyResult(args)`. Carries `X-Device-Id` (DP-10) and on 401 silently rotates the deviceId and retries once. Routes through `api.ts` (header injection added to `api.ts` in Track 3). Uses `ApiError` from `api.ts`; `DailyApiError` removed.

## State management

None. All functions are stateless.

## Rules specific to this directory

- **`ApiError` is the shared error type.** `api.ts` exports `ApiError` (carries `message` and `status`). All services use `ApiError`; `DailyApiError` was removed in Track 3.
- **URL construction goes through `new URL(path, API_BASE_URL || window.location.origin)`.** The fallback to `window.location.origin` is what lets the same code work in dev (Vite proxy) and prod (same-domain API Gateway). New services with custom headers inject them via `api.ts` parameters rather than bypassing the base.
- **Lesson 5 (Playwright cookie jars).** The `request` standalone fixture and `page.request` have separate cookie jars. Auth-gated tests must use `page.request.X(...)`. This is enforced by convention in `playwright/e2e/*.spec.ts` (see comments in `admin-config-flow.spec.ts`).
