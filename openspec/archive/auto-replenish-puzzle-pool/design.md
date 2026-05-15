# Auto-replenish puzzle pool — design

## Decisions

### D1. Reactive over cron

The roadmap entry suggested extending the `daily-cron` Lambda with a
`replenish` detail-type fired by EventBridge. Rejected in favour of a
reactive trigger on the read paths that drain the pool.

**Why reactive wins for this codebase:**

- The drain rate is request-driven, not time-driven. A cron firing
  every N hours is calibrated against an assumption about traffic;
  reactive is calibrated against the actual traffic.
- Quiet pools don't need refilling. Cron wakes up regardless;
  reactive only fires when something was actually consumed.
- No new IAM scaffolding on the daily-cron Lambda. The reactive
  trigger lives in handlers and code paths that already have the
  required IAM (the API Lambda has `sqs:SendMessage`; the daily-cron
  Lambda would have needed it added).
- No new EventBridge rule, no new Terraform, no new Lambda permission
  block. The infrastructure delta for this slice is zero.

**What reactive loses:**

- Cron triggers regardless of whether anyone has read from the
  partition. A pool that drains via a path we haven't wired (e.g.
  future admin tooling) would not auto-refill until the missing site
  is wired. Mitigated by treating drain-site enumeration as part of
  this slice's spec contract: every read-from-pool callsite gets
  audited.

### D2. Per-combo conditional-update debounce

Concurrent requests against a partition below threshold all see the
same low count and would all publish a deficit's worth of generation
messages. We coordinate them via a per-combo conditional `UpdateItem`
on the matching `CONFIG#{size}#{mode}` record:

```
UpdateExpression:        SET last_auto_replenish_ts = :now
ConditionExpression:     attribute_not_exists(last_auto_replenish_ts)
                         OR last_auto_replenish_ts < :cutoff
ExpressionAttributeValues:
  :now    = <ISO-8601 string of current UTC>
  :cutoff = <ISO-8601 string of (now - window)>
```

The first concurrent request whose conditional update succeeds wins
the claim. Losers receive `ConditionalCheckFailedException`, which
the repository translates to `(claimed=false, err=nil)` so the caller
treats it as a quiet skip.

**Window:** 60 seconds, hardcoded in this slice. Rationale:

- Same order-of-magnitude as the generator worker cycle for a 9×9
  Standard puzzle at the current implementation. A reactive trigger
  fires roughly once per generator cycle on a steady-stream-of-reads
  partition.
- Short enough that a hot partition refills inside one user's
  practice session.
- Long enough that 50 concurrent requests collapse to one publish.

**Storage:** ISO-8601 string in DDB. We already store timestamps that
way elsewhere in the schema; lexicographic comparison on ISO-8601 is
correct for the "<" predicate. Avoids epoch-vs-iso confusion.

### D3. Skip-count, fixed-batch publish

When the debounce claim succeeds, the reactive caller publishes a
**fixed batch of `threshold` generation requests** without first
counting the partition.

**Why skip the count:**

- `CountReady` is the function flagged by KI-011 — full-partition
  scan with `FilterExpression`. It is the most expensive read in the
  pool.
- The reactive path is on the user-facing hot path. Adding a
  partition scan per top-up event is exactly what KI-011 warns
  against.
- The generator worker's own threshold check at consume time bounds
  over-generation: workers reading from SQS verify the partition is
  still below threshold before persisting a generated puzzle. So
  publishing `threshold` messages can produce at most `threshold` net
  new puzzles regardless of the actual deficit at publish time.

**Why `threshold` and not `threshold/2`:**

- `threshold` is the simplest invariant — "publish exactly enough to
  refill from empty." The generator workers' consume-time check
  truncates the actual generation count to whatever the partition
  needs, so over-publishing is bounded server-side, not by us.
- A smaller batch (e.g. half) would be a tunable that's never the
  right answer in either direction.

The HTTP `POST /api/admin/replenish` path keeps its existing
count-then-publish-deficit logic — admin sweeps are explicitly
opt-in, called rarely, and the count cost is acceptable at that
cadence.

### D4. Async goroutine, best-effort

The reactive trigger fires from a goroutine after the response/cron
result is committed. The handler does **not** block on the SQS
publish.

**Risk:** AWS Lambda freezes the container after the handler returns.
Goroutines started during the handler may be paused mid-execution
and only resumed on the next invocation, or dropped entirely if the
container is recycled. The publish may never complete.

**Why we accept it:**

- The debounce window provides natural retry. If the goroutine
  drops, the next request after `window` expires gets a fresh claim
  and retries. Worst-case staleness: `window` seconds before
  recovery — same as the steady-state debounce slot.
- A 2-second `context.WithTimeout` on the publish call bounds
  resource usage if the runtime is about to freeze. Without this,
  the goroutine could sit idle for the lifetime of the next
  invocation.
- The alternative — sync publish — adds 10-30ms per response on the
  hot path and shifts the failure mode from "drop and retry" to
  "user-visible 500."

The `/admin/replenish` HTTP path keeps sync publish — admin callers
expect synchronous behaviour and the JSON response includes per-combo
results, which can't be returned async.

### D5. Shared `internal/replenish` package surface

Two exported functions, two distinct call shapes:

```go
package replenish

// Sweep iterates every enabled combo (or filtered subset), counts each,
// publishes deficit-many messages where below threshold. Used by the
// HTTP admin endpoint. Synchronous; returns a per-combo result list.
func Sweep(ctx context.Context, deps SweepDeps, filter Filter) (Result, error)

// TryReactiveTopUp performs the reactive single-combo path:
//   1. Try to claim the per-combo debounce window.
//   2. If claimed, publish `threshold` generation requests.
//   3. If not claimed, return ErrSkippedDebounced (sentinel).
// Used by drain-site goroutines. Synchronous on the goroutine; the
// caller is async.
func TryReactiveTopUp(ctx context.Context, deps ReactiveDeps, size int, mode string) error
```

`SweepDeps` and `ReactiveDeps` are explicit interface bundles —
distinct from each other because the reactive path needs the new
`TryClaimAutoReplenish` repository method but does not need
`CountReady`, while the HTTP path needs `CountReady` but does not
need `TryClaimAutoReplenish`. Keeping the dependency surfaces
separate makes the test doubles smaller and the call-site purpose
explicit.

The HTTP handler in `backend/internal/handler/replenish.go`
becomes a 30-line shim that builds `SweepDeps`, calls
`replenish.Sweep`, and serialises the result to JSON.

### D6. Drain-site enumeration

Three sites trigger reactively. The repository's `NextReady`,
`MarkServed`, and the daily flow's approved-pool reads are the
known drain points; admin reads (e.g. `GET /api/admin/pool`) are
not drains and do not trigger.

**Site 1 — `GET /api/puzzles/next` after `MarkServed`:**

```go
// existing handler logic (read + serve + mark)
if err := repo.MarkServed(ctx, partition, puzzle.ID); err != nil { ... }
go reactiveTopUp(size, mode)            // NEW
return puzzleJSON
```

**Site 2 — `daily.EnsureCandidate` after the approved-pool read:**

The approved-pool read drains 1 from the approved partition
(`9#standard`). Same goroutine pattern, same log line.

**Site 3 — Daily sync-fallback in `GET /api/daily/{date}`:**

The cold-start path (PR #103) drains 1 from the approved partition
when no `DAILY#date` row exists. Same trigger.

`reactiveTopUp` in each site is a thin wrapper that closes over the
package-level dependency bundle and calls
`replenish.TryReactiveTopUp` with a 2-second context.

## What we considered and rejected

### Sync publish on every drain

Rejected. Adds 10-30ms per practice serve on the user-facing path;
shifts SQS failure into user-visible 500 territory. Async with
debounce-as-retry is strictly better for our latency budget.

### In-memory Lambda-local debounce

Rejected. Doesn't survive cold starts; doesn't coordinate across
concurrent containers when traffic spikes. We've already had
container-level coordination bugs (KI-021, the multiple-generator
incident) and want to keep coordination on a shared substrate (DDB).

### Counter attribute on CONFIG

Rejected. Maintaining a denormalised `ready_count` on CONFIG is
KI-011's actual fix, and it's a real schema change with backfill
considerations. Out of scope for an operational slice that just
needs to stop the bleed.

### Configurable per-combo `auto_replenish: bool`

Rejected. Premature. Every `enabled: true` combo benefits from
auto-refill; if a combo doesn't, the right answer is to set
`enabled: false`, not to add a second flag.

### Cron + EventBridge as originally roadmapped

Rejected for this slice. The cron approach has a place — for
combos that drain through paths we haven't wired (or that drain
silently, like a future cleanup job). Today there's no such combo.
If a future drain path can't be reactively wired, we revisit the
cron path then.

## Verification checklist

Walked at slice close. Every line gets a citation.

- [ ] `internal/replenish.Sweep` test covers: empty config list,
      filtered-by-size, filtered-by-mode, mixed below/above
      threshold, publisher error mid-sweep.
- [ ] `internal/replenish.TryReactiveTopUp` test covers: first-fire
      success, debounce-blocked, missing CONFIG record (no panic),
      publisher error after claim (claim leaks; documented as
      acceptable since next window expiry retries).
- [ ] `repository.TryClaimAutoReplenish` test covers: absent
      attribute, expired attribute, fresh attribute, missing CONFIG.
- [ ] HTTP `POST /api/admin/replenish` regression: existing handler
      tests pass with the new shim wiring `Sweep`. Response shape
      byte-identical.
- [ ] All three drain sites have a unit test asserting `go` was
      dispatched after the drain (use a fake `reactiveTopUp` func
      injected for tests; assert called with correct size+mode).
- [ ] `task build && task test && task lint:backend` green on
      branch.
- [ ] `gitleaks detect --source .` clean.
- [ ] `review-local` CRITICAL/HIGH findings resolved.
- [ ] Existing e2e suite passes — no behaviour change for the
      `/api/puzzles/next` or daily flows from the player's
      perspective.
