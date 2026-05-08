# Auto-replenish puzzle pool — tasks

## Status

| ID | Slice | Status |
|---|---|---|
| — | Auto-replenish (reactive trigger + shared package) | [ ] |

Flip to `[x]` in the same branch as the implementation per CLAUDE.md
lesson 6.

## Implementation agent

**`backend-dev`** owns this slice (Go-only; no infra delta, no
frontend changes). The agent must read its own definition and the
lessons in CLAUDE.md before starting.

## Per-file work items

### `backend/internal/repository/puzzle.go`

1. Add `TryClaimAutoReplenish(ctx, size int, mode string, now time.Time, window time.Duration) (bool, error)`:
   - Builds key `{PK: "CONFIG", SK: fmt.Sprintf("%d#%s", size, mode)}`.
   - `UpdateExpression: SET last_auto_replenish_ts = :now`
   - `ConditionExpression: attribute_not_exists(last_auto_replenish_ts) OR last_auto_replenish_ts < :cutoff`
   - `:now`   = `now.UTC().Format(time.RFC3339Nano)`
   - `:cutoff` = `now.Add(-window).UTC().Format(time.RFC3339Nano)`
   - On `ConditionalCheckFailedException` → return `(false, nil)`.
   - On any other DDB error → return `(false, err)`.
   - On success → return `(true, nil)`.
2. Tests (`puzzle_test.go` or new file `puzzle_auto_replenish_test.go`):
   - First-fire (no attribute) returns `(true, nil)` and persists `:now`.
   - Fresh attribute (`now - 30s`) with `window=60s` returns `(false, nil)`.
   - Stale attribute (`now - 90s`) with `window=60s` returns `(true, nil)`.
   - Non-existent CONFIG record: condition fails → `(false, nil)` (we
     intentionally do not create CONFIG records reactively).
   - DDB transient error → propagated.

### `backend/internal/replenish/replenish.go` (new)

1. Create the package. Two exported functions:

   ```go
   type SweepDeps struct {
       Configs   ConfigReader
       Counter   PoolCounter
       Publisher MessagePublisher
   }

   type ReactiveDeps struct {
       Configs   ConfigReader            // narrow interface — by size+mode lookup
       Claimer   AutoReplenishClaimer    // TryClaimAutoReplenish
       Publisher MessagePublisher
       Window    time.Duration           // default 60s
       Clock     func() time.Time        // injectable for tests
   }

   type Filter struct { Size int; Mode string }
   type Result struct { Triggered []TriggeredEntry; Skipped []SkippedEntry }

   func Sweep(ctx, deps SweepDeps, filter Filter) (Result, error)
   func TryReactiveTopUp(ctx, deps ReactiveDeps, size int, mode string) error
   ```

   Sentinel: `var ErrSkippedDebounced = errors.New("replenish: debounced")`.

2. `Sweep` is the existing handler loop, lifted unchanged:
   - Read all configs.
   - Apply `filter` (zero values = no filter, matching current handler).
   - Skip disabled combos.
   - Per combo: `CountReady`. If `count >= threshold`, append `Skipped`.
     Else publish `(threshold - count)` messages, append `Triggered`.
   - First publisher error short-circuits the sweep with `(partial Result, err)`.

3. `TryReactiveTopUp`:
   - Lookup the single config for `{size, mode}`. If absent or
     `enabled: false`, return `nil` (no error — nothing to do).
   - `claimed, err := deps.Claimer.TryClaimAutoReplenish(...)`.
     - err → return err.
     - !claimed → return `ErrSkippedDebounced`.
   - Publish `config.Threshold` generation requests, one
     `PublishGenerationRequest` per message (KI-012 deferred).
     - First publisher error → return err. (Caller is the goroutine;
       it logs and moves on.)
   - Return `nil`.

4. Tests (`replenish_test.go`):
   - Sweep: covers empty, filter-by-size, filter-by-mode, mixed
     below/above, publisher error mid-sweep.
   - TryReactiveTopUp: claim wins → publishes N messages; claim loses
     → returns `ErrSkippedDebounced`, no publishes; missing config →
     returns nil, no publishes; disabled config → returns nil, no
     publishes; publisher error after claim → returns err.

### `backend/internal/repository/config.go` (or wherever single-config lookup lives)

1. If `GetConfig(size, mode)` doesn't already exist, add it.
   `GetAllConfigs` exists; the reactive path needs a single lookup
   that's cheaper than reading every config every drain.
2. Test: present, absent (returns sentinel `ErrConfigNotFound` or
   nil — match the existing pattern of the repo).

### `backend/internal/handler/replenish.go`

1. Replace the inlined loop with a thin shim:

   ```go
   func ReplenishHandler(deps replenish.SweepDeps) http.HandlerFunc {
       return func(w http.ResponseWriter, r *http.Request) {
           filter, err := parseFilter(r.URL.Query())
           if err != nil { httperr.WriteError(...); return }
           result, err := replenish.Sweep(r.Context(), deps, filter)
           if err != nil { ... }
           writeJSON(w, result)
       }
   }
   ```

2. Existing handler tests must still pass without modification —
   response shape is byte-identical. If the existing tests imported
   the old internal types, port them to the package-level types
   (`replenish.TriggeredEntry`, `replenish.SkippedEntry`).

3. Wire-up in `backend/cmd/api/main.go`: build `SweepDeps` once,
   pass to `ReplenishHandler`. The reactive deps bundle is built
   too and passed to whichever handler factories need it (see
   below).

### `backend/internal/handler/puzzles_next.go`

1. After successful `MarkServed`, dispatch:

   ```go
   go func() {
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
       defer cancel()
       err := replenish.TryReactiveTopUp(ctx, h.reactiveDeps, size, mode)
       if err != nil && !errors.Is(err, replenish.ErrSkippedDebounced) {
           log.Printf("reactive replenish: %dx%d %s: %v", size, size, mode, err)
       }
   }()
   ```

2. Inject `reactiveDeps replenish.ReactiveDeps` into the handler
   constructor. Update `cmd/api/main.go` accordingly.

3. Test: assert that after a successful serve, a fake reactive func
   was invoked with `(size, mode)`. Use a function-typed field on
   the handler struct so tests can replace `replenish.TryReactiveTopUp`
   with a counting fake. Same pattern as Phase-8 daily handler tests.

### `backend/internal/daily/ensure_candidate.go`

1. After the approved-pool read inside `EnsureCandidate`, dispatch
   the same goroutine. The function's signature gains a
   `replenishHook func(size int, mode string)` parameter — keep it
   injectable rather than reaching into a global, matching the
   daily package's existing test conventions.

2. The cron Lambda's `realService.EnsureCandidate` wires the hook
   via a closure over `replenish.TryReactiveTopUp` + the daily-cron
   Lambda's reactive deps bundle.

3. **IAM:** the daily-cron Lambda needs `sqs:SendMessage` added to
   its IAM policy. Update `infra/modules/daily-cron/main.tf`:
   - Add a new `aws_iam_role_policy.daily_cron_sqs` resource
     granting `sqs:SendMessage` on the generation queue ARN passed
     in via a new `var.generation_queue_arn` variable.
   - Plumb `var.generation_queue_arn` through `infra/main.tf` from
     `module.generation.queue_arn`.
   - Add `SQS_QUEUE_URL` env var to the daily-cron Lambda
     (sourced from `var.generation_queue_url` similarly plumbed).
   - `cmd/daily-cron/main.go` reads `SQS_QUEUE_URL`, builds a
     `queue.Publisher`, builds the reactive deps, passes to
     `daily.EnsureCandidate`.

4. Test: existing `daily/ensure_candidate_test.go` gets a new case
   asserting the hook is called once after a successful approved-pool
   read; not called when the pool read returns
   `ErrCandidatePoolEmpty`.

### `backend/internal/handler/daily.go` (sync-fallback path)

1. The sync-fallback shipped in PR #103. Find the function that
   reads from the approved pool when no `DAILY#date` row exists
   (likely `daily.SyncFinalizeForToday` or a helper) and dispatch
   the same goroutine after a successful pool read.

2. Inject the hook the same way as `EnsureCandidate` — function
   parameter, not global.

3. Test: success path triggers the hook; pool-empty path does not.

### `backend/cmd/api/main.go`

1. Build the `replenish.ReactiveDeps` bundle once, alongside the
   existing handler-deps wiring.
2. Pass it to: `puzzles_next` handler, daily handler, replenish
   handler.

### `backend/cmd/daily-cron/main.go`

1. Build a `queue.Publisher` from the new `SQS_QUEUE_URL`.
2. Build the reactive deps bundle.
3. Pass the closure to `realService.EnsureCandidate` and
   `realService.SyncFinalizeForToday`.

### `infra/modules/daily-cron/main.tf` + `infra/modules/daily-cron/variables.tf` + `infra/main.tf`

1. New variables: `generation_queue_arn`, `generation_queue_url`.
2. New IAM policy for `sqs:SendMessage` on the queue ARN.
3. New env var `SQS_QUEUE_URL` on the Lambda.
4. Plumb both vars from `module.generation` outputs in `infra/main.tf`.

### `.localstack/init-aws.sh` (or wherever CONFIG seeding lives)

1. No change required — `last_auto_replenish_ts` is added on first
   reactive fire and absent records are treated as "never fired."
2. Document the attribute in whatever schema doc covers the
   `puzzle-pool` table.

### `ROADMAP.md`

1. Mark the operational backlog "Auto-replenish the puzzle pool when
   it runs low" entry as done with a 2026-05-08+ date and a pointer
   to this PR.
2. No new KI added — KI-011 and KI-012 still tracked as before.

## Verification (run at slice close)

- [ ] `task build && task test && task lint:backend`
- [ ] `go test ./backend/internal/replenish/...` — new package green.
- [ ] `go test ./backend/internal/repository/... -run TryClaimAutoReplenish`
      — debounce-claim tests green.
- [ ] `gitleaks detect --source .` — clean.
- [ ] `review-local` — CRITICAL/HIGH resolved.
- [ ] `task e2e:up && task e2e:status` — fixture flow still serves
      (sanity that the goroutine doesn't break the path).
- [ ] PR description carries a Key Decisions section enumerating
      D1-D6 from `design.md`.
