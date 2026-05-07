# Daily Puzzle — Backend Spec (R-8-01)

Acceptance criteria for the backend slice. Cross-references design §3-5 and the design-grill findings. Numbering is contiguous; backend owns DP-01..DP-22 and OUT-01..OUT-05.

## Schedule row + candidate

**DP-01.** Schedule rows live in `puzzle-pool` with `PK = DAILY#YYYY-MM-DD`, `SK = <single>`. Item shape: `{ puzzleId, assignedAt, sourcePartition, started, solved }`. `assignedAt` is the cron's UTC stamp (or sync-fallback's stamp) and is never overwritten after creation. Counters default to `0`.

**DP-02.** Candidate slot is a singleton row at `PK = DAILY-CANDIDATE`, `SK = <single>`. Item shape: `{ puzzleId, queuedAt, sourcePartition }`. The row persists across days when a recycle leaves it unconsumed (D7).

**DP-03.** T-6h cron (cron expression `0 18 * * ? *`) is a freshness optimization, not a correctness gate. Steps: GetItem `DAILY-CANDIDATE`; if present and `queuedAt < 24h` → exit (idempotent). Else Query `PK = 9#standard` filtered by `verdictSummary.up >= 1 AND verdictSummary.down == 0 AND (lastDailyDate IS MISSING OR lastDailyDate < :fourteenDaysAgo)`, deterministic pick (lowest `puzzleId`), PutItem `DAILY-CANDIDATE` with `ConditionExpression: attribute_not_exists(PK) OR puzzleId = :pickedId`. On condition failure, exit cleanly — another invocation won. Empty pool → log + clean exit (Finding 9).

**DP-04.** T=0 cron (cron expression `0 0 * * ? *`) is correctness-critical. Steps: GetItem `DAILY#today` (if present → exit; another invocation won), GetItem `DAILY#yesterday` (missing → treat `solved == 0`), GetItem `DAILY-CANDIDATE`. Decision: candidate empty OR `yesterday.solved == 0` → recycle; else confirm. Recycle leaves `DAILY-CANDIDATE` untouched; confirm deletes it. All `PutItem` on `DAILY#today` use `ConditionExpression: attribute_not_exists(PK)` — at-least-once safe.

**DP-05.** Sync fallback inside `GET /api/daily/{date}` engages only when the schedule row is missing AND `date == todayUTC` (Finding 1). Runs the T=0 finalize algorithm only — never the T-6h ensure path. Two concurrent GETs racing this fallback resolve via the schedule row's conditional `attribute_not_exists(PK)`: the loser sees `ConditionalCheckFailedException`, retries the GetItem, and reads the winner's row.

**DP-06.** Counter increments use a DDB `UpdateItem` with `UpdateExpression: ADD <field> :one` against the schedule row. Atomic by construction — no read-modify-write race. `:one` is the integer 1; field is `started` or `solved`.

## API surface

**DP-07.** `GET /api/daily/{date}` validates `{date}` as a UTC `YYYY-MM-DD` string. Future dates → 404. Dates older than `yesterdayUTC` → 404 (Finding 8). `todayUTC` and `yesterdayUTC` → allowed for both Anonymous and User. Malformed strings → 400.

**DP-08.** GET resolves `playerId` (`userId` if signed-in, else `deviceId` from request — see DP-10). On PLAY-row miss, `PutItem` with `outcome=started`, `assignedAt=now`, `ConditionExpression: attribute_not_exists(PK)`. On condition failure, GetItem and use the winner's `assignedAt`. Subsequent GETs (PLAY row present) update only an advisory `lastSeenAt`; `assignedAt` is NEVER reset (Finding 3, DP-19).

**DP-09.** GET 200 response shape: `{ puzzleId, grid, regions, assignedAt, outcome, serverElapsedMs?, submittedAt? }`. `serverElapsedMs` and `submittedAt` are present only when `outcome == 'solved'`.

**DP-10.** Anonymous identity: `deviceId` arrives in the `X-Device-Id` request header (frontend stores in localStorage). PLAY row's `playerId` field stores `deviceId` for anonymous, `userId` for signed-in. No leaderboard row is written for anonymous players (D13). GET with no auth and no `X-Device-Id` → 401.

**DP-11.** `POST /api/daily/{date}/result` body shape: `{ outcome: 'solved', playTimeMs: number, solution: number[][] }`. Outcome must be the literal string `'solved'`. `playTimeMs` is a non-negative integer. `solution` is the player-claimed solved grid; backend re-validates against the puzzle's expected solution. Invalid solution → 400. Malformed body → 400.

**DP-12.** Submission is `TransactWriteItems` with up to three legs (D14, Finding 4):
- Leg 1 (always): UpdateItem PLAY with `outcome=solved`, `serverElapsedMs`, `submittedAt=now`, `playTimeMs`; `ConditionExpression: outcome = :started` for idempotency. Retry returns 409, no double-count.
- Leg 2 (always): UpdateItem schedule row with `ADD solved :one`. Date keys off the PLAY row's `assignedAt` date, NOT `now` (Finding 7, DP-13's twin in the cross-midnight contract).
- Leg 3 (signed-in only): PutItem `DAILY-LEADERBOARD#{playOriginDate}` with `SK = <paddedMs>#<userId>`. `paddedMs` is zero-padded `serverElapsedMs` (8 digits → max ~27.7 hours, ample headroom for any legitimate solve time). Anonymous players skip this leg.
All three commit or none do.

**DP-13.** Server stamps `serverElapsedMs = submittedAt - assignedAt` from the PLAY row's `assignedAt`. The client's `playTimeMs` is recorded for telemetry but is not authoritative. Cross-midnight submission: a player assigned at `23:55:00 UTC` and submitting at `00:00:02 UTC` next day still increments the prior date's `solved` counter (Finding 7).

**DP-14.** Auth matrix: GET allows Anonymous (with `X-Device-Id`) and User; missing both → 401. POST allows the same matrix; anonymous POST commits Legs 1+2 only (no leaderboard row).

## Approval

**DP-15.** Pool selection filter is the single approval gate: `verdictSummary.up >= 1 AND verdictSummary.down == 0` against `PK = 9#standard` (D3). No new admin UI — approval is derived from existing Phase 7 verdicts. The combined eligibility filter is `up >= 1 AND down == 0 AND (lastDailyDate IS MISSING OR lastDailyDate < :fourteenDaysAgo)`.

**DP-16.** Empty approved pool at T-6h → log + clean exit; T=0 will recycle yesterday. Empty pool at T=0 with no candidate → recycle yesterday's puzzle (D6 path). If yesterday's row is also missing AND no candidate AND pool empty → fail loudly to ops (structured error log + non-2xx return); do NOT loop, do NOT silently no-op. Finding 9 covers this — recycling forever is forbidden.

## PuzzleRecord backref

**DP-17.** `PuzzleRecord` (existing row at `PK = 9#standard`, `SK = <puzzleId>`) gains a single `lastDailyDate` field. Single date string `YYYY-MM-DD` — NOT an array (D11). Existing reads ignore the field; existing writes preserve it.

**DP-18.** `lastDailyDate` is set ONLY at T=0 finalization — atomically inside the schedule TransactWriteItems on confirm, and inside the recycle path on T=0 (recycle advances `lastDailyDate` to today, Finding 6). T-6h does NOT touch `lastDailyDate`: the candidate slot's conditional PutItem is the sole guard against duplicate-cron races, and reserving `lastDailyDate` before the puzzle is actually used would be semantically wrong. On a recycle day, the T-6h candidate stays untagged and is naturally reused tomorrow.

## Anti-cheat scaffolding

**DP-19.** `assignedAt` is server-stamped at the first GET that materializes the PLAY row, never overwritten. Refresh, second device, and second GET all return the original `assignedAt`. Frontend cannot influence its value.

**DP-20.** Client-claimed `playTimeMs` is captured into the PLAY row but `serverElapsedMs` is the recorded source of truth. Phase 9 leaderboards rank on `serverElapsedMs` only; `playTimeMs` is informational telemetry useful for spotting clock-skew or offline-play patterns.

## Error contracts

**DP-21.** Status codes:
- 200 — success
- 400 — malformed date, malformed body, invalid solution
- 401 — missing auth AND missing `X-Device-Id` on either GET or POST
- 404 — date out of `[yesterdayUTC, todayUTC]` window, OR sync fallback exhausted (yesterday's row missing AND not creatable)
- 409 — submission idempotency: PLAY row already has `outcome = 'solved'`
- 500 — TransactWriteItems failure (see DP-22)

**DP-22.** TransactWriteItems failure on submission returns 500 with a clear structured log line (`daily submit: total_ms=N step1_ms=N step2_ms=N error=...`). No partial write commits — DDB transaction guarantees this. The handler logs `subsystem`, the failing leg's index, and the underlying DDB error code so ops has enough context.

## Out of scope (R-8-01)

**OUT-01.** Streak counter row — Phase 9 will derive streaks from PLAY history at read time. No new row shape introduced here.

**OUT-02.** Leaderboard reader endpoint (`GET /api/daily/{date}/leaderboard`) — Phase 9. R-8-01 writes the leaderboard partition; R-8-01 does not read it back to the client beyond `leaderboardRank` returned in the POST response.

**OUT-03.** ELO / rating cron — Phase 9.

**OUT-04.** Submission attestation tokens (e.g. signed grid-state proofs) — Phase 9 if anti-cheat surfaces real abuse.

**OUT-05.** Per-player private leaderboard / friends-only views — out of v1 scope entirely.
