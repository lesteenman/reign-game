# Daily Recycle Counter — Design

**Issue:** [#184 — Daily puzzle recycled when started but not solved](https://github.com/lesteenman/reign-game/issues/184)
**Date:** 2026-05-18
**Status:** Approved (awaiting writing-plans)
**Bundled (one-time exception):** `formatTime` MM:SS overflow fix surfaced from the closed #185 investigation.

## Context

A daily puzzle assigned on 2026-05-17 was re-assigned on 2026-05-18 despite a player having opened it. DDB confirms in production:

| Date | puzzleId | counters.solved | counters.started | schedule.assignedAt |
|---|---|---|---|---|
| 2026-05-17 | `16f0e877-1ee6-4648-8ba8-8d0ecda8a98d` | 0 | 0 | `2026-05-17T00:00:07Z` |
| 2026-05-18 | `16f0e877-1ee6-4648-8ba8-8d0ecda8a98d` (same) | 0 | 0 | `2026-05-18T00:00:07Z` |

Plus an anonymous PLAY row keyed `DAILY#2026-05-18` (`assignedAt: 2026-05-18T12:36:17Z, outcome: started`) — confirms a player started, yet `counters.started` is `0`.

### Two-layer root cause

1. **Wrong condition in the recycle decision tree** — `service/daily/sync.go:143-146` only checks `Solved == 0`. "Started but not solved" recycles. The doc comment at `sync.go:43-48` documents this as the intended algorithm, so the spec is also wrong.
2. **`counters.started` is a write-only-zero counter** — initialised to `0` in `FinalizeDaily` (`daily.go:159`), no writers anywhere in the codebase. `ScheduleCounterStarted` exists as a constant in `repository/daily.go:73` but is unused.

### Bundled formatting fix

`GameBoard.tsx:16-20` `formatTime` always renders MM:SS. A 5h50m elapsed displays as `350:05`, which reads as a four-digit number rather than "hours and minutes". Surfaced when investigating #185 (which closed as works-as-designed — the wall-clock-anchored display is already wired and the value was correct, just badly formatted).

## Decisions

### "Started" semantic — GET-creates-PLAY

`counters.started` is incremented on first GET that materialises a PLAY row. Alternative semantics considered:

- **First-move signal** — would require a new endpoint or POST piggyback. More work; "started" would reflect real engagement; recycle path stays meaningful.
- **Drop "started" entirely** — close #184 wontfix and address as UX.

GET-creates-PLAY is the simplest match to the user's framing. Acknowledged limitation: at scale, any bot minting deviceIds disqualifies the puzzle from recycle, effectively killing the recycle path. Theoretical at current scale (today's only PLAY row was the reporter's own). Revisit if the recycle path is observably dead and the candidate pool starts draining faster than it refills.

### Atomicity — single TransactWriteItems

PLAY-row creation and `counters.started` bump go in **one** `TransactWriteItems` call. Invariant: PLAY row exists ⇒ schedule counter ≥ 1.

Alternative considered: best-effort separate `UpdateItem` after the Put. Simpler diff, but allows drift; per CLAUDE.md lesson 6 ("two layers defending the same condition must pick one policy"), atomicity is the cleaner choice. Atomic also gives free idempotency under repeated GETs: leg-0 conditional failure aborts the whole TX, counter does not double-increment.

Composition lives in `service/` per the architecture rule "multi-leg DDB transactions live in service/, not repository/" — same pattern `FinalizeDaily` already uses.

### Decision-tree gate

`chooseFinalizeTarget` (`sync.go:143`) becomes:

```go
if yesterday.Counters.Solved == 0 && yesterday.Counters.Started == 0 {
    return yesterday.PuzzleID, yesterday.SourcePartition, repository.FinalizeModeRecycle, nil
}
return candidate.PuzzleID, candidate.SourcePartition, repository.FinalizeModeConfirm, nil
```

Doc comment at `sync.go:43-48` (step-4 decision tree) updated to mention the started counter.

### Timer formatting

`GameBoard.tsx:16-20` `formatTime` switches on `seconds >= 3600`:

```ts
function formatTime(seconds: number): string {
  const s = seconds % 60;
  const m = Math.floor(seconds / 60) % 60;
  const h = Math.floor(seconds / 3600);
  const ss = String(s).padStart(2, '0');
  const mm = String(m).padStart(2, '0');
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}
```

Hour digit un-padded — slimmer rendering, presence of `H:` itself signals hour-scale.

### No retro-fix for today's already-recycled schedule

The deployed fix doesn't change `DAILY#2026-05-18`'s row (already recycled under the old logic). Tomorrow's behaviour is correct. Flagged in PR body; no migration code.

## Implementation outline

### Backend (`backend/internal/service/daily/`)

- `helpers.go` — `materializePlayRow` create branch: assemble `[]types.TransactWriteItem{...}` (PLAY put with `attribute_not_exists(PK)` condition + schedule `UpdateItem` with `ADD #counters.#started :one`), call `store.WriteTransaction(items)`. On condition failure on leg 0 → re-read PLAY (race-loser branch, current behaviour preserved). On other TX failures → propagate.
- `sync.go:143` — add `&& yesterday.Counters.Started == 0` to the recycle branch. Update doc comment at `sync.go:43-48`.
- `daily.go::Service.Store` interface — `PutPlayStartedIfAbsent` removed (no remaining callers — verify by grep before delete).
- `repository/daily.go` — `PutPlayStartedIfAbsent` removed if no other callers in `backend/`.

### Frontend (`frontend/src/shared/game/components/`)

- `GameBoard.tsx:16-20` — replace `formatTime` with the H-aware variant.

## Tests (TDD — write first)

### Backend

1. `chooseFinalizeTarget` table-driven:
   - `started=0, solved=0`, candidate present → `FinalizeModeRecycle`.
   - `started=1, solved=0`, candidate present → `FinalizeModeConfirm`.
   - `started=5, solved=2`, candidate present → `FinalizeModeConfirm`.
   - `started=0, solved=0`, no candidate, no yesterday → `ErrPoolExhausted`.
2. `materializePlayRow`:
   - First call: TX succeeds, PLAY row returned, `WriteTransaction` invoked with both legs (assert via fake store).
   - Second call (existingPlay non-nil): short-circuits, no TX.
   - Race-loser: TX condition-fails on leg 0, re-reads existing PLAY, returns winner's row; counter does NOT double-increment.
3. `GetDaily` (integration, existing test file): assert `schedule.counters.started` goes 0 → 1 after a fresh GET on a freshly-finalized schedule.

### Frontend

`formatTime` unit test (export the function or test via rendered DOM):

| seconds | expected |
|---|---|
| 0 | `00:00` |
| 59 | `00:59` |
| 60 | `01:00` |
| 303 | `05:03` |
| 3599 | `59:59` |
| 3600 | `1:00:00` |
| 3723 | `1:02:03` |
| 21005 | `5:50:05` |
| 36000 | `10:00:00` |

## Out of scope

- **Server-side pause/resume on idle** — the wall-clock-anchored display is correct as designed; making it pause on idle is a product decision needing design-grill, not a bug fix.
- **First-move "started" semantic** — listed as an alternative above; defer unless the recycle path is observably dead in production.
- **Migration / retro-fix for today's recycled schedule** — accepted as known, single-day cost.

## PR scope

Single branch `feat/184-daily-recycle-counter`, single PR. Key Decisions section reproduces the four bulleted decisions above. Cross-boundary check: response shape unchanged, no playwright-cli verification needed. Security trigger: `service/daily/*.go` + `repository/daily.go` modifications; not on the deep-review list, but let `code-review-final` decide whether to escalate.
