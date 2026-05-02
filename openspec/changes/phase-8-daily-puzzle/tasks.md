# Phase 8 — Daily Puzzle: Tasks

## Status

| ID | Slice | Layer | Status |
|---|---|---|---|
| R-8-01 | Backend daily resolution + cron + submission | 1 | [ ] |
| R-8-02 | Frontend daily flow UX | 2 | [ ] |

## Slice dependency graph

R-8-01 (backend) → R-8-02 (frontend)

R-8-02 depends on R-8-01's API contract being live. Implementation can overlap if the frontend stubs against the spec in `specs/daily-puzzle-frontend.md`, but R-8-02 cannot ship until R-8-01 is merged.

## Tasks

### R-8-01: Backend daily resolution + cron + submission

**Repository (`backend/internal/repository/`):**
- [ ] New file `daily.go` with `DailyRepository` interface + `PuzzleRepository` implementation (D15 — single-table; row shapes per design §3)
- [ ] Method `GetSchedule(ctx, date)` — GetItem `PK=DAILY#YYYY-MM-DD`, `SK=<single>` (design §3 row 2)
- [ ] Method `GetCandidate(ctx)` / `PutCandidate(ctx, puzzleId, sourcePartition)` / `DeleteCandidate(ctx)` — `PK=DAILY-CANDIDATE`; PutCandidate uses `ConditionExpression: attribute_not_exists(PK) OR puzzleId = :sameId` (Finding 2, design §4 T-6h step 4)
- [ ] Method `FinalizeSchedule(ctx, date, puzzleId, sourcePartition, mode)` where `mode ∈ {confirm, recycle}` — wraps the TransactWriteItems package described in design §4 T=0 step 5; conditional `attribute_not_exists(PK)` on the schedule row (Finding 2 / design §4 step 6)
- [ ] Method `IncrementCounter(ctx, date, field)` for `started`/`solved` — `ADD <field> :one` on the schedule row (D10)
- [ ] Method `ListApprovedPool(ctx, mode, size)` — Query `PK = <size>#<mode>` filtered server-side by `verdictSummary.up >= 1 AND verdictSummary.down == 0 AND (lastDailyDate IS MISSING OR lastDailyDate < :fourteenDaysAgo)` (D3, design §4 T-6h step 2)
- [ ] Method `MarkPuzzleAsDailyOn(ctx, puzzleId, date)` — UpdateItem on PuzzleRecord setting `lastDailyDate = :date` (D11; called inside the schedule TransactWriteItems and on recycle for Finding 6)
- [ ] Method `GetPlay(ctx, playerId, date)` / `CreatePlayStarted(ctx, playerId, date, assignedAt)` — PutItem with `ConditionExpression: attribute_not_exists(PK)` (Finding 3, design §5 GET behavior)
- [ ] Method `SubmitPlay(ctx, playerId, date, payload)` — TransactWriteItems for PLAY update + counter increment + optional leaderboard PutItem (D14, Finding 4, design §5 POST behavior)
- [ ] Method `LeaderboardRank(ctx, date, paddedMs, userId)` — Query `PK = DAILY-LEADERBOARD#YYYY-MM-DD` with `SK <= :playerSK` (design §5 POST response)
- [ ] Tests (TDD, table-driven): row-shape round-trip per access pattern; conditional-update race coverage on PutCandidate, CreatePlayStarted, FinalizeSchedule, SubmitPlay's `outcome = :started` guard (Finding 4)

**Handlers (`backend/internal/handler/`):**
- [ ] New `daily.go` with `DailyGetHandler(repo)` and `DailySubmitHandler(repo)`
- [ ] Date-string validation: parse `{date}` as `YYYY-MM-DD` UTC; allow `todayUTC` and `yesterdayUTC` only; future dates → 404 (Finding 8, design §5)
- [ ] GET: resolve `playerId` (`userId` if signed-in, else `deviceId` from `X-Device-Id` header per Finding 5)
- [ ] GET: GetItem schedule row; if missing AND date == todayUTC, run sync fallback (T=0 finalize algorithm — design §4 sync-fallback contract; Finding 1)
- [ ] GET: GetItem PLAY row; on miss, PutItem `outcome=started`+`assignedAt=now` with `attribute_not_exists(PK)` guard. On condition failure, GetItem and use the winner's `assignedAt` — refresh / second device must NOT reset `assignedAt` (Finding 3)
- [ ] GET: response shape per design §5 table — includes puzzle payload + player state
- [ ] POST: validate submitted `solution` against the puzzle's expected solution; invalid → 400
- [ ] POST: read PLAY row; compute `serverElapsedMs = now - assignedAt`; counter date keys off PLAY's `assignedAt` date, NOT `now` (Finding 7, design §5 leg 2)
- [ ] POST: TransactWriteItems with three legs (PLAY update with `outcome = :started` guard for idempotency, schedule counter `ADD solved :one`, leaderboard PutItem for signed-in only) (D13, D14, Finding 4)
- [ ] POST: 409 on `outcome` already `solved`; 400 on invalid solution; 200 with `serverElapsedMs` and (signed-in only) `leaderboardRank`
- [ ] Auth wiring: GET and POST both Anonymous-or-User (D13 — anonymous play, no leaderboard row)
- [ ] Per-step timing logs per CLAUDE.md "Backend logging" — both handlers issue 2+ DDB calls so they qualify for the default `total_ms=N step1_ms=...` instrumentation
- [ ] Tests: auth matrix (anon/user), date-window validation (today/yesterday/future/malformed), sync fallback engagement on missing schedule row, idempotent submission (409 on retry), counter date for cross-midnight submissions (Finding 7)

**Cron / worker (`backend/cmd/daily-cron/` + EventBridge):**
- [ ] New Lambda entry point `backend/cmd/daily-cron/main.go` with two subcommands or a single dispatch keyed by `event.DetailType` (`t-6h-ensure` vs `t-0-finalize`)
- [ ] T-6h handler: implement design §4 T-6h cron — GetItem candidate, skip if fresh, else Query approved pool, deterministic pick, conditional PutCandidate, reserve `lastDailyDate` on PuzzleRecord with conditional update; empty pool → log + clean exit (Finding 9)
- [ ] T=0 handler: implement design §4 T=0 cron — `attribute_not_exists` GetItem guard for today, GetItem yesterday for solved count (treat missing as 0), GetItem candidate, decision tree (recycle if candidate empty OR yesterday.solved == 0; else confirm), TransactWriteItems for the chosen path (D6, Finding 9)
- [ ] Both handlers idempotent on duplicate firings — verified by integration test that runs each cron twice
- [ ] Structured log events on each cron path so observability hooks (deferred infra slice per design §7) have stable names
- [ ] Tests: cron fires twice (idempotency), pool empty → recycle path, missing candidate → recycle path, normal path → confirm + DeleteCandidate, recycle advances `lastDailyDate` to today (Finding 6)

**Wiring (`backend/cmd/api/main.go`):**
- [ ] Register routes: `GET /api/daily/{date}`, `POST /api/daily/{date}/result`
- [ ] Anonymous-or-User middleware variant for the daily endpoints (D13)
- [ ] Inject `DailyRepository` via constructor dependencies (no globals, per CLAUDE.md backend conventions)

**Infrastructure (`infra/`):**
- [ ] New EventBridge schedule rules (cron expressions `0 18 * * ? *` and `0 0 * * ? *`) targeting the new daily-cron Lambda (D5)
- [ ] IAM role: read/write on `puzzle-pool` table only; no GSIs needed (D15)
- [ ] Terraform module wiring + `terraform fmt -recursive` before commit
- [ ] Add `daily-cron` Lambda to the existing deploy pipeline (mirrors generator setup)

**OpenSpec + sweep:**
- [ ] Cross-doc sweep per design §6 checklist — `ROADMAP.md`, `GLOSSARY.md`, `PROJECT_STRUCTURE.md`, `CLAUDE.md` (logging note only)
- [ ] Grep `DAILY#`, `DAILY-CANDIDATE`, `DAILY-LEADERBOARD#`, `PLAY#`, `lastDailyDate` across `*.md`, `*.go`, `*.ts`, `*.tsx`, `*.tf`, `*.yml`, shell scripts (Lesson 14)
- [ ] Flip the R-8-01 row to `[x]` in this `tasks.md` as part of the slice's PR (Lesson 17)

### R-8-02: Frontend daily flow UX

**Routing (`frontend/src/App.tsx` + `frontend/src/pages/`):**
- [ ] New `DailyPage.tsx` mounted at `/play?flow=daily` — wraps the existing GamePage and injects daily-specific service + storage adapters
- [ ] LandingPage's `tile-daily` becomes enabled with `onClick → navigate('/play?flow=daily')`; copy update for the tile

**Service (`frontend/src/services/`):**
- [ ] New `dailyService.ts`: `getDaily(date?)` (defaults to today UTC) and `submitDailyResult({ outcome, playTimeMs, solution })`
- [ ] Reads/writes `X-Device-Id` header for anonymous identity (Finding 5 / design §5 anonymous identity contract)
- [ ] Type definitions imported from a shared location (Lesson 16 — DTOs live in service module, not in components)
- [ ] Vitest unit tests with explicit Arrange/Act/Assert sections per CLAUDE.md frontend conventions

**Storage (`frontend/src/storage/`):**
- [ ] Reuse the per-flow IndexedDB pattern from R-7-03; key with `flowType: 'daily'`, `flowId: 'YYYY-MM-DD'`
- [ ] Storage shape definitions live in `storage/`, not in DailyPage or hooks (Lesson 16)
- [ ] No new schema migration needed if R-7-03's per-flow store already accepts arbitrary `flowType`; otherwise extend the type union

**UX states (`frontend/src/pages/DailyPage.tsx`):**
- [ ] **Initial load**: fetch today's daily via `getDaily()`, show puzzle on success
- [ ] **Mid-play**: standard GamePage interactions; persist progress under daily storage key
- [ ] **Solved (this session)**: post-completion screen with `serverElapsedMs` from server response, "Done for today" message, countdown to next UTC midnight
- [ ] **Already-solved (prior session)**: on landing, read PLAY storage; if solved, short-circuit to "Done for today" without re-fetching
- [ ] **Recycle day callout**: post-completion copy must explicitly mention recycle days when applicable (design §7 residual risk — recycle UX surprise)
- [ ] **Error states**: 404 (date out of window), network failure, invalid solution → user-facing error with retry affordance

**Tests:**
- [ ] Vitest unit tests on `dailyService.ts` (mocked fetch) and on DailyPage state machine
- [ ] Playwright e2e: `/play?flow=daily` happy path against `task dev:up` — load, play, submit, see "Done for today"
- [ ] Playwright e2e: already-solved short-circuit on second visit
- [ ] Per CLAUDE.md lesson 3, write the Playwright e2e BEFORE Vitest unit tests for any touch/pointer interaction added in this slice

**OpenSpec + sweep:**
- [ ] Update `PROJECT_STRUCTURE.md` if new component / service / storage files are added under previously-undocumented trees
- [ ] Flip the R-8-02 row to `[x]` in this `tasks.md` as part of the slice's PR (Lesson 17)

## Gate criteria

**R-8-01 done when:**
- All routes return correct status codes for the auth + validation matrix (anon/user × today/yesterday/future/malformed)
- Both cron jobs run idempotently (integration test: each cron fires twice without double-write)
- Submission writes are atomic via `TransactWriteItems` (verified by integration test that asserts all three legs commit or none do)
- Sync fallback engages on missing schedule row and converges on the same row both crons would have written
- Counter date for cross-midnight submissions keys off `assignedAt`, not `now` (Finding 7 integration test)
- `lastDailyDate` advances on recycle (Finding 6 integration test)
- review-local + security-review on the slice's PR (Lesson 13)

**R-8-02 done when:**
- `/play?flow=daily` renders today's puzzle for both signed-in and anonymous players
- Submitted plays show "Done for today" with countdown to next UTC midnight
- Already-solved short-circuit avoids a second `getDaily()` round trip
- `flowType: 'daily'` IndexedDB slot saves and resumes mid-play correctly
- Playwright e2e covers happy path + already-solved short-circuit
- review-local + security-review on the slice's PR (Lesson 13)

## Verification Checklist (Phase Close)

- [ ] `GET /api/daily/{today}` returns the scheduled puzzle; `{tomorrow}` returns 404; malformed date returns 400
- [ ] First GET creates a PLAY row with `outcome: 'started'` + `assignedAt`; second GET / second device reuses the same `assignedAt`
- [ ] `POST /api/daily/{date}/result` with valid body atomically updates PLAY → solved, increments schedule `solved` counter, and writes a leaderboard row only for signed-in players
- [ ] T=0 cron at 00:00 UTC reads yesterday's `solved` count and either confirms candidate or recycles yesterday's puzzle
- [ ] T-6h cron at 18:00 UTC ensures the `DAILY-CANDIDATE` slot has a fresh approved puzzle, or logs and exits cleanly when the pool is empty
- [ ] Anonymous players' PLAY rows key by `deviceId`; no leaderboard row is written for them
- [ ] Pool filter `verdictSummary.up >= 1 AND verdictSummary.down == 0` is the only approval gate (no admin UI)
- [ ] `lastDailyDate` is set on confirm AND advanced on recycle (Finding 6)
- [ ] LandingPage `tile-daily` is enabled and routes to `/play?flow=daily`
- [ ] `tasks.md` status table all `[x]` after both slices ship (Lesson 17)
- [ ] No new KIs opened by this phase
- [ ] review-local + security-review on every slice's PR (Lesson 13)

## Cross-references

- Specs: `openspec/changes/phase-8-daily-puzzle/specs/daily-puzzle-backend.md`, `openspec/changes/phase-8-daily-puzzle/specs/daily-puzzle-frontend.md`
- Design: `openspec/changes/phase-8-daily-puzzle/design.md`
- Stress-test: `openspec/changes/phase-8-daily-puzzle/design-grill-summary.md`
- Proposal: `openspec/changes/phase-8-daily-puzzle/proposal.md`
