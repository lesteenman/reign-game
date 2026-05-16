# internal/daily/

Multi-step orchestration for the Daily Puzzle feature. Single-DDB-call primitives live in `repository/daily.go`; algorithms that compose several primitives — and that the handler and the cron need to share verbatim — land here.

## Data flow

- **In** — Called by `internal/handler/daily.go::DailyGetHandler` (synchronous fallback when today's schedule is missing) and by `cmd/daily-cron/main.go` (EventBridge-driven Lambda).
- **Calls** — A `Repo` interface that's structurally satisfied by `*repository.PuzzleRepository`. Plus the `replenishHook func(size int, mode string)` injected from the caller.
- **Out** — On success, a populated `*repository.ScheduleRecord` (for `SyncFinalizeForToday`) or `nil` error (for `EnsureCandidate`). Sentinel errors `ErrCandidatePoolEmpty` / `ErrPoolExhausted` indicate "approved pool is empty"; the caller decides whether to recycle, 404, or 500.

## Files

| File | Responsibility |
|---|---|
| `cron.go` | T-6h `EnsureCandidate` algorithm — look up the candidate, refresh if stale (>24h), pick a deterministic puzzle from the approved pool, conditionally put. |
| `sync.go` | T=0 `SyncFinalizeForToday` algorithm — the four-branch decision tree (candidate present?, yesterday present?, yesterday solved?) plus the cold-start bootstrap that calls `EnsureCandidate` synchronously when both signals are absent. |

## Key types and exported symbols

- `Repo` — narrow interface (`GetSchedule`, `GetCandidate`, `FinalizeDailyTransaction`, `ListApprovedPool`, `PutCandidateIfAbsent`).
- `EnsureCandidate(ctx, repo, now, replenishHook) error` — T-6h cron entry point.
- `SyncFinalizeForToday(ctx, repo, today, yesterday, now, replenishHook) (*ScheduleRecord, error)` — T=0 / sync-fallback entry point.
- `ErrCandidatePoolEmpty`, `ErrPoolExhausted` — sentinels.
- Constants: `CandidateFreshnessWindow = 24h`, `CandidatePoolSize = 9`, `CandidatePoolMode = "standard"`.

## Algorithmic notes

- **Deterministic candidate selection.** `lowestPuzzleID` sorts the pool and picks the lexicographically smallest ID so duplicate cron firings collapse cleanly on `PutCandidateIfAbsent`.
- **Replenish hook timing.** Hook fires after a non-empty `ListApprovedPool` result, **before** the candidate Put — the Put outcome (winner / race-loser / error) doesn't affect whether replenish should run.
- **Cold-start bootstrap.** `SyncFinalizeForToday` detects the "no candidate AND no yesterday schedule" path (was the original `ErrPoolExhausted` branch) and synchronously calls `EnsureCandidate`. Latency cost on the unlucky first request: one extra `Query` + one conditional `Put` + one `GetItem`.

## Rules specific to this directory

- **Time zone agnostic.** `today` and `yesterday` are caller-supplied YYYY-MM-DD strings (UTC). Callers compute them; this package never sees `time.Now()` except as the `now` parameter, which is itself caller-supplied.
- **Idempotency on duplicate firings.** Both `EnsureCandidate` and `SyncFinalizeForToday` treat `ErrCandidateAlreadyExists` / `ErrScheduleAlreadyFinalized` as race-loser successes — return the canonical row, not an error.
