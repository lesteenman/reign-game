# Phase 8 — Daily Puzzle: Design

## 1. Summary

Phase 8 introduces Reign's first player-facing scheduled flow: one canonical 9×9 Standard puzzle per UTC day, drawn from the Phase 7 approved pool. Signed-in players land on a daily leaderboard; anonymous players play and see their own time but do not appear on the leaderboard. See `proposal.md` for goals and scope; this document locks the technical contract.

The phase ships in two slices:
- **R-8-01** — backend schema, repository, handler, two cron jobs, and sync fallback.
- **R-8-02** — frontend landing-tile state machine, GamePage Daily variant, and post-completion screen.

R-8-03 (admin curation UI) is dropped — approval is derived from existing Phase 7 verdicts (`up >= 1 && down == 0`), so no new admin surface is required. All design-grill findings (10 total: 4 MAJOR, 5 MINOR, 1 INFO) are resolved with concrete mechanisms; see section 7 for the cross-reference and `design-grill-summary.md` for the full grill.

## 2. Locked decisions

| # | Decision | Locked answer | Rationale |
|---|---|---|---|
| D1 | Daily scope | Single canonical 9×9 Standard puzzle for everyone | Avoids GSI / cross-partition complexity; matches NYT/Wordle convention |
| D2 | Date semantics | UTC midnight rollover | Recycle invariant trivially holds; clock-tamper hardening via server-stamped `assignedAt` |
| D3 | Approval semantics | Derived: `verdictSummary.up >= 1 && verdictSummary.down == 0` | Zero new admin UI; leverages existing Phase 7 verdict corpus |
| D4 | Phase shape | 2 slices (R-8-01 backend, R-8-02 frontend) | R-8-03 admin-UI dropped because approval is derived |
| D5 | Generation strategy | Two-phase cron: T-6h candidate ensure + T=0 finalize, with sync fallback | Optimal pool conservation; sync fallback handles cron failure |
| D6 | Recycle threshold | Zero `solved` records on yesterday | Started-but-bounced does not taint; matches user intuition |
| D7 | Candidate slot | Singleton `DAILY-CANDIDATE` row, persists across days when not consumed | Recycle never burns the queued fresh puzzle |
| D8 | Server-issued timer | `assignedAt` stamped on first GET; `serverElapsedMs` is source of truth | Avoids breaking retrofit when leaderboard ships in Phase 9 |
| D9 | Outcome model | `'started' \| 'solved'` (no skip on dailies/packs) | Curation has Skip; daily/packs do not surface that action |
| D10 | Per-day counter | `{ started: N, solved: M }` on schedule row | Powers recycle decision (`solved == 0`) + telemetry |
| D11 | PuzzleRecord backref | Single field `lastDailyDate` (NOT array) | Smaller writes; sufficient for cross-feature exclusion |
| D12 | Source partition | Keep `sourcePartition` field on schedule rows | Future-proofs combo rotation without locking it in |
| D13 | Anonymous play | PLAY row keyed by `deviceId`; NO leaderboard row | Inclusive play, exclusive ranking |
| D14 | Submission atomicity | `TransactWriteItems` (PLAY update + counter increment + LEADERBOARD if signed-in) | Submission is atomic across all 2-3 mutations |
| D15 | Schema partition strategy | 5 row shapes in `puzzle-pool` table (single-table principle) | No GSIs; each hot path is single GetItem or Query |

## 3. Schema

Five row shapes in the existing `puzzle-pool` table. No new tables, no GSIs.

```
1. PK = 9#standard                           → PuzzleRecord (existing) + new field: lastDailyDate
   SK = <puzzleId>

2. PK = DAILY#YYYY-MM-DD                     → Schedule row
   SK = <single>                                fields: puzzleId, assignedAt (cron stamp),
                                                        sourcePartition, started, solved

3. PK = DAILY-CANDIDATE                      → Singleton candidate slot
   SK = <single>                                fields: puzzleId, queuedAt, sourcePartition

4. PK = PLAY#<playerId>                      → Per-player play history
   SK = DAILY#YYYY-MM-DD                        fields: outcome, assignedAt, submittedAt,
                                                        serverElapsedMs, playTimeMs (advisory)

5. PK = DAILY-LEADERBOARD#YYYY-MM-DD         → Sorted leaderboard (signed-in only)
   SK = <paddedMs>#<userId>                     fields: userId, serverElapsedMs, submittedAt
```

**Row 1 — PuzzleRecord (extended).** Access pattern: existing curation reads. **Write triggers:** new `lastDailyDate` field is set in the same transaction that writes the schedule row, AND advanced on recycle (Finding 6). **Read triggers:** future cross-feature exclusion checks; not read during the daily hot path.

**Row 2 — Schedule.** Access pattern: single GetItem per request keyed by date. **Write triggers:** T=0 cron (or sync fallback) creates the row; submission TransactWriteItems increments `started` / `solved`. **Read triggers:** every `GET /api/daily/{date}` to resolve the puzzle; T=0 cron reads yesterday's row for the recycle decision.

**Row 3 — Candidate slot.** Access pattern: single GetItem from the cron / fallback path. **Write triggers:** T-6h cron writes with `ConditionExpression: attribute_not_exists(PK) OR puzzleId = :sameId` (Finding 2); T=0 cron deletes after consuming on confirm; recycle leaves it untouched (D7). **Read triggers:** T=0 cron, sync fallback.

**Row 4 — PLAY.** Access pattern: GetItem by `playerId` + date. `playerId` = `userId` for signed-in, `deviceId` for anonymous. **Write triggers:** first GET creates with `ConditionExpression: attribute_not_exists(PK)` (Finding 3); submission updates with `ConditionExpression: outcome = :started` (Finding 4); subsequent GETs touch only `lastSeenAt` (advisory). **Read triggers:** every GET to retrieve `assignedAt` and current state.

**Row 5 — Leaderboard.** Access pattern: Query `PK = DAILY-LEADERBOARD#YYYY-MM-DD` ordered by SK ascending → fastest first. **Write triggers:** signed-in submission TransactWriteItems leg. **Read triggers:** post-completion screen; future Phase 9 leaderboard surface. SK uses zero-padded `serverElapsedMs` (8 digits → max ~27.7 hours) followed by `#userId` to disambiguate ties.

**Known limitation (Finding 10):** dangling `started` PLAY rows for players who never return. Accepted; ~$0.02/year storage at projected scale. TTL attribute reserved for Phase 9+ if growth surprises.

## 4. Cron + sync-fallback algorithm

Two scheduled jobs plus an idempotent fallback inside the GET handler. All writes use conditional expressions; every job is safe to retry.

### T-6h cron — candidate ensure (freshness)

Runs at 18:00 UTC. Goal: ensure `DAILY-CANDIDATE` holds a fresh approved puzzle ahead of the T=0 swap.

1. GetItem `DAILY-CANDIDATE`. If present and `queuedAt` < 24h old → done (idempotent on duplicate firings).
2. Query approved pool: `PK = 9#standard`, filter `verdictSummary.up >= 1 AND verdictSummary.down == 0 AND (lastDailyDate IS MISSING OR lastDailyDate < today - 14d)`.
3. Pick deterministically (lowest puzzleId among eligible; deterministic so duplicate firings select the same row).
4. PutItem `DAILY-CANDIDATE` with `ConditionExpression: attribute_not_exists(PK) OR puzzleId = :pickedId` (Finding 2). On condition failure → another invocation won; exit.
5. If approved pool is empty → log and exit cleanly (Finding 9). T=0 will recycle.

`PuzzleRecord.lastDailyDate` is NOT touched at T-6h — the candidate slot's conditional PutItem is the sole guard against duplicate-cron races, and `lastDailyDate` is set only at T=0 when the puzzle is actually finalized as today's daily.

T-6h is a freshness optimization, not a correctness gate (Finding 1).

### T=0 cron — finalize (correctness-critical)

Runs at 00:00 UTC. Goal: write today's schedule row.

1. Conditional check: GetItem `DAILY#today`. If present → another invocation won; exit.
2. GetItem `DAILY#yesterday`. Read `solved` count. If yesterday's row missing (cold start) → treat as `solved == 0`.
3. GetItem `DAILY-CANDIDATE`.
4. Decision tree:
   - **Candidate empty** → recycle. (Finding 9)
   - **`yesterday.solved == 0`** → recycle. (D6)
   - **else** → confirm candidate.
5. Build TransactWriteItems:
   - **Recycle path:** PutItem `DAILY#today` with `puzzleId = yesterday.puzzleId`, `assignedAt = now`, `sourcePartition = yesterday.sourcePartition`, counters zeroed; UpdateItem on the PuzzleRecord advancing `lastDailyDate = today` (Finding 6); leave `DAILY-CANDIDATE` untouched (D7).
   - **Confirm path:** PutItem `DAILY#today` with the candidate's `puzzleId`; UpdateItem on the PuzzleRecord setting `lastDailyDate = today`; DeleteItem `DAILY-CANDIDATE`.
6. All PutItem operations on `DAILY#today` use `ConditionExpression: attribute_not_exists(PK)` for at-least-once safety.

### Sync fallback (inside GET handler)

Runs when `GET /api/daily/{date}` arrives and the schedule row is missing. Treats T-6h as a freshness hint only — runs the **T=0 finalize algorithm only** (Finding 1).

1. Validate date: must be `todayUTC` or `yesterdayUTC` (Finding 8); else 404.
2. GetItem `DAILY#{date}`. If present → proceed to normal handler logic.
3. If missing AND date == todayUTC → execute the T=0 finalize algorithm above. The TransactWriteItems' conditional `attribute_not_exists(PK)` guarantees that two concurrent GETs racing this fallback do not double-write — the loser sees `ConditionalCheckFailedException`, retries the GetItem, and reads the winner's row.
4. If missing AND date == yesterdayUTC → 404 (yesterday's row should always exist by the time today is requested; if it does not, system is in an unrecoverable state and the daily endpoint surfaces an error).

**Counter date semantics (Finding 7):** the `started`/`solved` counter increments on submission key off `assignedAt`'s date (the date the player was assigned), NOT `submittedAt`. A player assigned at 23:55:00 UTC and submitting at 00:00:02 UTC the next day still increments the prior date's counter. Integration test required.

## 5. API surface

Two endpoints, both under `/api/daily/`. Auth is Anonymous-or-User (Phase 6 middleware) — admins are not special-cased here.

| Method | Path | Auth | Body | Response |
|---|---|---|---|---|
| `GET` | `/api/daily/{date}` | Anonymous or User | none | `200 { puzzleId, grid, regions, assignedAt, outcome, serverElapsedMs?, submittedAt? }` ; `404` if date out of window |
| `POST` | `/api/daily/{date}/result` | Anonymous or User | `{ outcome: 'solved', playTimeMs: number, solution: number[][] }` | `200 { serverElapsedMs, leaderboardRank? }` ; `409` if outcome already `solved` ; `400` if solution invalid |

**`GET /api/daily/{date}` behavior:**
- Resolves `playerId` from auth context (`userId` if signed-in, else `deviceId` from request header).
- Validates date is `todayUTC` or `yesterdayUTC` (Finding 8); else 404.
- GetItem schedule row; sync-fallback if missing (section 4).
- GetItem PLAY row keyed by `playerId` + date.
  - **First GET (PLAY row missing):** PutItem PLAY with `outcome=started`, `assignedAt=now`, `ConditionExpression: attribute_not_exists(PK)` (Finding 3). On condition failure, GetItem and use that row's `assignedAt`.
  - **Subsequent GET (PLAY row present):** UpdateItem only `lastSeenAt` (advisory). Never touch `assignedAt`.
- Returns puzzle payload + the player's current state (`outcome`, and if solved, `serverElapsedMs` + `submittedAt`).

**`POST /api/daily/{date}/result` behavior:**
- Resolves `playerId` from auth.
- Validates the submitted `solution` against the puzzle's expected solution. If invalid → `400`.
- Reads the PLAY row to compute `serverElapsedMs = now - assignedAt`.
- TransactWriteItems (Finding 4):
  - **Leg 1 (always):** UpdateItem PLAY with `outcome=solved`, `serverElapsedMs`, `submittedAt=now`, `playTimeMs` (advisory); `ConditionExpression: outcome = :started` for idempotency (retry returns 409, no double-count).
  - **Leg 2 (always):** UpdateItem schedule row with `ADD solved :one`. Date is read from the PLAY row's `assignedAt`, not `now` (Finding 7).
  - **Leg 3 (signed-in only):** PutItem leaderboard row at `DAILY-LEADERBOARD#{playOriginDate}` with SK = `<paddedMs>#<userId>`.
- Response includes `serverElapsedMs` and (signed-in only) `leaderboardRank` computed by Querying the leaderboard partition with `SK <= :playerSK`.

**Anonymous identity contract (Finding 5).** `deviceId` from a header (frontend stores in localStorage). On localStorage clear, the player gets a new identity and their daily PLAY row from the prior identity is dangling. Acceptable since leaderboard is signed-in only.

## 6. Lesson 14 cross-doc sweep checklist

When R-8-01 / R-8-02 land, the following docs must be updated in the same branch (Lesson 14 — path/URL/env renames need a full-repo grep). Phase 8 introduces 5 new partition prefixes and 2 new endpoints — both are sweep-eligible.

- [ ] `ROADMAP.md` — flip R-8-01 and R-8-02 status rows; remove R-8-03 entry (dropped).
- [ ] `GLOSSARY.md` — add new terms surfaced by the design grill (Daily Puzzle, Schedule Row, Candidate Slot, Recycle, Sync Fallback, `assignedAt`, `serverElapsedMs`, `playerId`).
- [ ] `PROJECT_STRUCTURE.md` — add `/api/daily/{date}` and `/api/daily/{date}/result` to the API endpoints table; add new repository / handler files under their respective trees.
- [ ] `CLAUDE.md` — no port table change (Phase 8 adds no new service). Add a brief note under "Backend logging" if new structured-event names land.
- [ ] `openspec/changes/phase-8-daily-puzzle/specs/*.md` — generated in the next chunk.
- [ ] Grep for the 5 new partition prefixes (`DAILY#`, `DAILY-CANDIDATE`, `DAILY-LEADERBOARD#`, `PLAY#`, plus `lastDailyDate` field) across `*.md`, `*.go`, `*.ts`, `*.tsx`, `*.tf`, `*.yml`, and shell scripts before merge to catch stale references.

## 7. Risks + design-grill cross-reference

Full grill content lives in `design-grill-summary.md`. Resolutions are baked into sections 3-5. Quick index:

| # | Severity | Topic | Resolved by |
|---|---|---|---|
| 1 | MAJOR | Sync-fallback algorithm scope (T=0 only) | §4 sync-fallback contract |
| 2 | MAJOR | Candidate slot duplicate-cron atomicity | §4 T-6h step 4 conditional PutItem |
| 3 | MAJOR | First-GET PLAY row idempotency | §5 GET behavior conditional PutItem |
| 4 | MAJOR | Submission write atomicity (3 items) | §5 POST behavior TransactWriteItems with `outcome = :started` |
| 5 | MINOR | Anonymous deviceId churn | §5 Anonymous identity contract; accepted |
| 6 | MINOR | `lastDailyDate` race on recycle | §4 T=0 recycle path advances `lastDailyDate` |
| 7 | MINOR | Counter increments on cross-midnight submissions | §4 closing note + §5 leg 2 keys off `assignedAt` |
| 8 | MINOR | Date-string validation | §5 GET behavior date window check |
| 9 | MINOR | Approval pool empty at T-6h | §4 T=0 decision tree handles empty candidate |
| 10 | INFO | Dangling `started` PLAY rows | §3 row 4 known-limitation note; deferred |

**Documented residual risks (echoed from `proposal.md`):**
- DDB transaction limits — far under the 100-item / 4MB cap; documented for completeness.
- Recycle UX surprise — frontend post-completion screen copy must call out recycle days explicitly (R-8-02 task).
- Cron observability — structured events ship in R-8-01; CloudWatch alarms deferred to a follow-up infra slice.
