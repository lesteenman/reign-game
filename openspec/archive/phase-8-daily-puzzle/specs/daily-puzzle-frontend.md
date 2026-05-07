# Daily Puzzle — Frontend Spec (R-8-02)

Acceptance criteria for the frontend slice. Cross-references design §5, the per-flow IndexedDB pattern from R-7-03, and the backend contract in `daily-puzzle-backend.md` (DP-01..DP-22). Numbering continues from the backend spec; frontend owns DP-23..DP-32 and OUT-06..OUT-09.

## Routing + landing

**DP-23.** LandingPage's `tile-daily` becomes enabled. `onClick` navigates to `/play?flow=daily`. Tile copy is updated to reflect the live daily flow (no longer "coming soon"). The tile remains visible to Anonymous and User identities; both can play.

**DP-24.** GamePage detects `flow=daily` from the URL search params and switches its behavior:
- Skip the pool-selector / mode-picker UI entirely.
- Fetch via `dailyService.getDaily()` instead of the curation/practice fetcher.
- Use the daily storage adapter (DP-26) for in-progress persistence.
- The daily branch must not bleed into other flows — a `flow=practice` navigation immediately after a daily session resets the GamePage to its standard behavior with no leftover state.

**DP-25.** Post-completion screen shows `serverElapsedMs` from the POST response, the message "Done for today", and a live countdown to the next UTC midnight. On a recycle day (detected via response metadata), the copy explicitly mentions that today's puzzle is a recycle of yesterday's — design §7 residual risk mitigation.

## Storage

**DP-26.** Per-flow IndexedDB slot uses `(flowType: 'daily', flowId: 'YYYY-MM-DD')` — reuses the R-7-03 per-flow store unchanged when its `flowType` union already accepts `'daily'`. If R-7-03's union is closed, this slice extends it (no schema migration of stored data — only the type union). Storage shape lives in `frontend/src/storage/`, not in DailyPage or hooks (Lesson 16).

**DP-27.** Opening daily after solving short-circuits via PLAY history check — DailyPage reads the local daily storage slot first and, if `outcome === 'solved'`, renders the post-completion screen without re-fetching `getDaily()`. The short-circuit is observable via Playwright (no network call to `/api/daily/...` on the second visit).

## Service contract

**DP-28.** `dailyService.getDaily(date?)` calls `GET /api/daily/{date}`. `date` defaults to `todayUTC` derived in the browser. Returns the puzzle payload + `assignedAt` exactly as DP-09 defines. The service sends `X-Device-Id` (read from localStorage; created on first invocation if missing) — Anonymous identity contract per DP-10.

**DP-29.** `dailyService.submitDailyResult({ outcome, playTimeMs, solution })` calls `POST /api/daily/{date}/result` with the exact body shape DP-11 defines. The `date` is derived from the PLAY's `assignedAt` (NOT from `now`) — frontend reads `assignedAt` from the loaded puzzle response and uses its date component. This matches DP-13's cross-midnight contract and avoids a 404 when a player submits at 00:00:02 UTC for an `assignedAt` of 23:55:00 UTC the prior day.

**DP-30.** Error handling — each backend status maps to a meaningful UX state:
- 400 (invalid solution) → "That doesn't match — keep trying" inline message; no navigation.
- 401 (missing auth + missing deviceId) → silently mint a new deviceId and retry once; on second 401, surface a generic error with retry affordance.
- 404 (date out of window / fallback exhausted) → "No daily available right now" full-screen message with a manual retry button.
- 409 (already solved) → short-circuit to post-completion screen, refresh storage from server response.
- 500 → user-facing "Something went wrong, try again" with retry; do NOT auto-retry (avoids hammering a failing transaction).

## State machine

**DP-31.** DailyPage states: `loading` → `playing` → `solved` (or `error`). Transitions:
- `loading` → `playing` on `getDaily()` success.
- `loading` → `solved` when local storage already has `outcome === 'solved'` (DP-27 short-circuit).
- `loading` → `error` on network failure or 4xx/5xx (per DP-30).
- `playing` → `solved` on successful `submitDailyResult()` (200 response).
- `playing` → `error` on submission 4xx/5xx (with retry path back to `playing`).
- All transitions covered by Vitest unit tests with explicit Arrange/Act/Assert blocks.

**DP-32.** Refresh / second device picks up the same puzzle and the same `assignedAt`. Frontend invariant: never recompute or override `assignedAt` locally. The server-side guarantee from DP-19 is what makes this work — frontend's job is simply to display whatever `assignedAt` the server returns and use it for the submission's date derivation. Playwright covers: solve mid-puzzle, refresh, observe progress restored from IndexedDB and `assignedAt` unchanged in the response.

## Out of scope (R-8-02)

**OUT-06.** Leaderboard view (top-N list, player's rank in context, friends filter) — Phase 9. The post-completion screen may show `leaderboardRank` from the POST response if present, but no list view ships in this slice.

**OUT-07.** Streak indicator UI (current streak, longest streak, freeze tokens) — Phase 9. The data model for streaks isn't materialized in R-8-01 (OUT-01).

**OUT-08.** Anonymous "create an account to track your streak" prompt — Phase 9 if at all. R-8-02 lets anonymous players play and finish without any account-creation friction.

**OUT-09.** Pre-fetch tomorrow's daily for offline play — Phase 9 optimization. R-8-02 fetches on demand only.
