# `src/services/`

Legacy backend client modules. New service code does NOT land here — the BR-correct home is `features/<feature>/services/` (or wired directly via TanStack `useQuery` / `useMutation`). What remains here is awaiting its own #176 slice.

Three production files (down from four after #120 moved `api.ts` to `shared/api/`) plus three unit tests.

## Responsibility

Wrap `/api/*` calls in typed functions. Each service composes request URLs, headers, and error semantics on top of the shared `@shared/api` fetch client. Today services are called directly from page-level `useEffect`s and from one leaf component (`VerdictSurface`) — that latter call is the known leaf-I/O violation.

## Data flow

- **In:** Called by `pages/*` and `shared/game/` hooks and components.
- **Out:** `apiGet` / `apiPut` / `apiPost` from `@shared/api`, which calls `fetch()` to `/api/*`. Base URL is `VITE_API_URL || window.location.origin`; in dev, Vite's proxy forwards `/api/*` to `localhost:5181`.

## Files

- **`puzzleService.ts`** — `updatePuzzleStatus(puzzleId, size, mode, status)` (PUT /api/puzzles/{id}/status). Used cross-feature by `shared/game/hooks/useUpdatePuzzleStatus` (which itself serves both curation flow's solve-and-rate and daily flow's completion path). `fetchNextPuzzle` + `NoPuzzlesAvailableError` moved to `features/curation/services/fetch-next-puzzle-service.ts` in #176.
- **`adminService.ts`** — Pool / config CRUD: `fetchPoolStatus`, `updateConfig`, `createConfig`, `triggerReplenish`. Also re-exports `MODES` / `isMode` / `Mode` from `engine/types` (architectural smell — drive-by indirection).
- **`dailyService.ts`** — `getDaily(date?)` and `submitDailyResult(args)`. Carries `X-Device-Id` (DP-10) and on 401 silently rotates the deviceId and retries once. Uses `apiGet` / `apiPost` from `@shared/api` with `options.headers` for the device-id injection.

(`api.ts` moved to `shared/api/` in #120 — collapsed `apiFetch` / `apiPut` / `apiPost` into one `apiRequest` + thin wrappers; renamed `apiFetch` to `apiGet`. `landingService.ts` moved to `features/curation/services/enabled-modes-service.ts` in #176; was always vestigially named — only CurationPage consumed it. `verdictService.ts` moved to `features/curation/services/` in #176 PR #202.)

## State management

None. All functions are stateless.

## Rules specific to this directory

- **`ApiError` is the shared error type.** `@shared/api` exports `ApiError` (carries `message` and `status`). All services use it; `DailyApiError` was removed in Track 3.
- **URL construction is centralized in `@shared/api/client.ts`.** Don't re-implement it here. New services with custom headers inject them via `apiGet` / `apiPut` / `apiPost`'s `options.headers` parameter rather than bypassing the base.
- **Lesson 5 (Playwright cookie jars).** The `request` standalone fixture and `page.request` have separate cookie jars. Auth-gated tests must use `page.request.X(...)`. This is enforced by convention in `playwright/e2e/*.spec.ts` (see comments in `admin-config-flow.spec.ts`).
