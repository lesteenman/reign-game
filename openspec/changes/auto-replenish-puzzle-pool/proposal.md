# Auto-replenish puzzle pool (proposal)

## What

Replenish the puzzle pool reactively from the read paths that drain it,
instead of requiring an admin to click "Replenish All" in `/admin`.

When a request reads from a `{size}#{mode}` partition, the handler
attempts to claim a per-combo debounce window via a conditional
DynamoDB update on the matching CONFIG record. If it wins the claim, a
fire-and-forget goroutine publishes a fixed batch of generation
requests to SQS. If another concurrent request already claimed the
window, this request skips silently.

Three read sites are wired:

1. `GET /api/puzzles/next` — practice serve, after `MarkServed`.
2. `daily.EnsureCandidate` (T-6h cron) — after the approved-pool read
   that materialises tomorrow's candidate.
3. The sync-fallback path in `GET /api/daily/{date}` — after the
   approved-pool read that bootstraps a missing schedule row.

The existing `POST /api/admin/replenish` endpoint stays as the
admin-driven full-sweep path; its loop logic is extracted into a
shared `internal/replenish` package that both the HTTP handler and
the reactive callers use.

## Why

The 2026-05-08 daily-puzzle sync-fallback fix (PR #103) self-bootstraps
from the approved verdict pool when `EnsureCandidate` and
`SyncFinalizeForToday` haven't run. That fallback drains 1 approved
9×9 Standard puzzle every cold-start day. Without an automatic
top-up trigger, the approved pool eventually drops below 1 and the
daily endpoint starts returning 500.

Today the only top-up path is an admin clicking a button. An
operations gap that requires a human-in-the-loop to keep production
healthy is not a launch-grade posture.

This slice closes the gap by tying replenishment to the act of
draining: every drain path becomes a self-healing path. No clock, no
new EventBridge rule, no new Lambda — just a hook on the operations
that already touch the partition.

## Scope

**In scope:**

- `backend/internal/replenish` package with `Sweep` (full-sweep used
  by HTTP handler) and `TryReactiveTopUp` (single-combo with debounce
  used by reactive callers). TDD: tests written first.
- `puzzle.Repository.TryClaimAutoReplenish(size, mode, now, window)`:
  conditional `UpdateItem` on `CONFIG#{size}#{mode}` that succeeds only
  when `last_auto_replenish_ts` is absent or older than `now - window`.
- Reactive trigger wired into the three drain sites listed above. All
  three call `TryReactiveTopUp` from a goroutine; the response/cron
  result is not blocked on the publish.
- `backend/internal/handler/replenish.go` rewritten to delegate to
  `replenish.Sweep`. Existing JSON response shape preserved; HTTP
  callers see no behaviour change.
- `cmd/api/main.go` repository wiring surfaces the new method (no new
  IAM — API Lambda already has `dynamodb:UpdateItem` and
  `sqs:SendMessage`).
- LocalStack seed updated with the new attribute path documented in
  the CONFIG schema (no migration — attribute is optional, absent
  treated as "never fired").

**Out of scope (explicit non-goals):**

- KI-012 (batch SendMessage). The reactive top-up batch is small
  enough — `threshold` messages, max single-digit per combo — that
  the existing serial loop is fine. Fix KI-012 in its own slice.
- KI-011 (sparse GSI for `CountReady`). The reactive path **avoids**
  `CountReady` entirely (skip-count fixed-batch design). The HTTP
  handler still calls it; that cost is unchanged.
- Per-combo opt-out. Every `enabled: true` combo participates. No new
  CONFIG flag. The `last_auto_replenish_ts` debounce is the only new
  attribute.
- EventBridge / cron-driven replenish. The roadmap entry's "extend
  daily-cron with replenish detail-type" path is dropped in favour of
  reactive. Rationale in `design.md`.
- Frontend changes. `/admin` button still works; no new UI.
- Production observability beyond structured log lines. CloudWatch
  alarms on the SQS queue depth land in a follow-up infra slice.

## Success criteria

- A `GET /api/puzzles/next` against a partition where
  `count < threshold` triggers exactly one SQS publish across N
  concurrent requests in the same debounce window. (Verified by unit
  test on `TryReactiveTopUp` with a mock that simulates concurrent
  conditional-update racers.)
- `EnsureCandidate` continues to drain the approved pool by 1; on
  partitions configured with auto-replenish enabled, the next
  generator-worker cycle restores the count without admin
  intervention.
- The HTTP `POST /api/admin/replenish` response shape is unchanged.
  Existing `AdminPage.tsx` calls work without modification.
- After the change, the only path that requires a human-in-the-loop
  for pool top-up is the admin button. The reactive path handles all
  steady-state traffic.
- `task build && task test && task lint:backend` green.
- `gitleaks detect --source .` clean.
- `review-local` CRITICAL/HIGH findings resolved before PR.

## Risks

- **Goroutine drop on Lambda freeze.** A reactive trigger fires a
  goroutine after the handler returns; AWS Lambda freezes the
  container after the response is written, which can pause or drop
  the goroutine before SQS publish completes. **Accepted** because
  the debounce window is a guard, not a guarantee — the next request
  after the window expires retries. Worst case: a single-digit-second
  delay before the next attempt. Mitigated by a short
  `context.WithTimeout(2s)` on the publish path so the goroutine
  doesn't sit idle when the runtime is about to freeze.
- **Debounce window too short or too long.** Hardcoded at 60s in this
  slice. Too short → SQS noise on hot partitions. Too long → pool
  drain visible to users. 60s is the same order-of-magnitude as the
  generator's per-puzzle latency for 9×9 Standard, which means a
  reactive trigger fires roughly once per generator cycle. Tunable
  via constant if production data argues otherwise; not a CONFIG
  knob this slice.
- **Race between reactive top-up and admin sweep.** Both publish to
  the same SQS queue; both are bounded by the per-combo threshold
  check on the next sweep. The generator workers' own per-combo
  threshold guard (read at consume time) prevents duplicate
  generations from being persisted past `threshold`. Acceptable
  steady-state.
- **First-fire after CONFIG migration.** Records without
  `last_auto_replenish_ts` must be treated as "never fired" by the
  conditional update. Verified by test for the
  `attribute_not_exists` path of the conditional expression.

## Slice ID

No phase commitment. Operational follow-up to PR #103. Change folder
name `auto-replenish-puzzle-pool` is the canonical reference; no
`R-<phase>-<slice>` ID issued.
