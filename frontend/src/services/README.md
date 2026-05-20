# `src/services/`

Legacy backend client modules. New service code does NOT land here — the BR-correct home is `features/<feature>/services/` (or wired directly via TanStack `useQuery` / `useMutation`). What remains here is awaiting its own #176 slice.

Four production files plus four unit tests (api.test.ts is new in #176; previously the apiFetch helper's tests were co-located with puzzleService's, and were split out when fetchNextPuzzle moved to `features/curation/`).

## Responsibility

Wrap `/api/*` calls in typed functions. The shared `api.ts` exposes a thin fetch base; per-resource services compose request URLs, headers, and error semantics on top. Today services are called directly from page-level `useEffect`s and from one leaf component (`VerdictSurface`) — that latter call is the known leaf-I/O violation.

## Data flow

- **In:** Called by `pages/*` and `shared/game/` hooks and components.
- **Out:** `fetch()` to `/api/*`. Base URL is `VITE_API_URL || window.location.origin`; in dev, Vite's proxy forwards `/api/*` to `localhost:5181`.

## Files

- **`api.ts`** — Shared fetch base. Exports `apiFetch<T>(path, params?)`, `apiPut<T>(path, body, params?)`, `apiPost<T>(path, body, params?)`, and an `ApiError` class carrying `status`. The three helpers are near-identical (URL construction + headers + ok-check + body parse) — duplication called out in `frontend/FINDINGS.md`.
- **`puzzleService.ts`** — `updatePuzzleStatus(puzzleId, size, mode, status)` (PUT /api/puzzles/{id}/status). Used cross-feature by `shared/game/hooks/useUpdatePuzzleStatus` (which itself serves both curation flow's solve-and-rate and daily flow's completion path). `fetchNextPuzzle` + `NoPuzzlesAvailableError` moved to `features/curation/services/fetch-next-puzzle-service.ts` in #176.
- **`adminService.ts`** — Pool / config CRUD: `fetchPoolStatus`, `updateConfig`, `createConfig`, `triggerReplenish`. Also re-exports `MODES` / `isMode` / `Mode` from `engine/types` (architectural smell — drive-by indirection).
- **`dailyService.ts`** — `getDaily(date?)` and `submitDailyResult(args)`. Carries `X-Device-Id` (DP-10) and on 401 silently rotates the deviceId and retries once. Routes through `api.ts` (header injection added to `api.ts` in Track 3). Uses `ApiError` from `api.ts`; `DailyApiError` removed.

(`landingService.ts` moved to `features/curation/services/enabled-modes-service.ts` in #176; was always vestigially named — only CurationPage consumed it. `verdictService.ts` moved to `features/curation/services/` in #176 PR #202.)

## State management

None. All functions are stateless.

## Rules specific to this directory

- **`ApiError` is the shared error type.** `api.ts` exports `ApiError` (carries `message` and `status`). All services use `ApiError`; `DailyApiError` was removed in Track 3.
- **URL construction goes through `new URL(path, API_BASE_URL || window.location.origin)`.** The fallback to `window.location.origin` is what lets the same code work in dev (Vite proxy) and prod (same-domain API Gateway). New services with custom headers inject them via `api.ts` parameters rather than bypassing the base.
- **Lesson 5 (Playwright cookie jars).** The `request` standalone fixture and `page.request` have separate cookie jars. Auth-gated tests must use `page.request.X(...)`. This is enforced by convention in `playwright/e2e/*.spec.ts` (see comments in `admin-config-flow.spec.ts`).
