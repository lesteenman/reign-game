# Phase 8 — Daily Puzzle

## Why

Reign needs a recurring engagement loop. Phase 1-7 built the puzzle generator, the verdict-based curation pipeline, and the auth+landing surface. Daily Puzzle is the first **player-facing scheduled flow**: one canonical puzzle per UTC day, drawn from the approved pool, with leaderboard timing for signed-in players.

This is also the precedent-setting phase for Phase 9+ (Packs, Leaderboards proper, Stats). Cleanliness of the schema, schedule logic, and play-history model here propagates forward.

## What

A daily puzzle resolved in this sequence:

1. A pre-warm cron at **T-6h UTC** ensures a fresh approved puzzle is queued in the candidate slot.
2. A finalize cron at **T=0 UTC** reads yesterday's `solved` count.
   - `solved == 0` → recycle: today's daily = yesterday's puzzle. Candidate slot stays for tomorrow.
   - `solved >= 1` → confirm: today's daily = candidate. Slot empties.
3. If either cron misses, the **first GET request runs the same algorithm synchronously** — no spec-level dependency on cron uptime.
4. `GET /api/daily/{date}` returns the puzzle and stamps `assignedAt` for the player on first call (idempotent on retries).
5. `POST /api/daily/{date}/result` records the result. Server computes `serverElapsedMs = submittedAt - assignedAt`. Anonymous players record only the play row; signed-in players additionally land on the daily leaderboard.

A single canonical puzzle (9×9 Standard) for everyone. No combo rotation in Phase 8 (`sourcePartition` field present for Phase 9+).

## Scope

**In scope:**

- Backend handler, repository, schema, two cron jobs, sync fallback (R-8-01).
- Frontend: landing-tile state machine, GamePage variant for Daily, post-completion screen with elapsed time and (signed-in) leaderboard placement (R-8-02).
- Anti-cheat scaffolding: server-stamped `assignedAt`, server-computed `serverElapsedMs`, client-claimed `playTimeMs` recorded but advisory.
- Anonymous play recorded against `deviceId`. Signed-in play recorded against `userId` + leaderboard.

**Out of scope (explicit non-goals):**

- Admin approval UI. Approval is **derived** from existing Phase 7 verdict corpus (`up >= 1 && down == 0`). No new admin surface.
- Combo rotation. Daily resolves only against `9#standard`; `sourcePartition` field reserved for future.
- Anti-cheat enforcement (signed payloads, replay detection, suspicious-time flagging). Scaffolding only — Phase 9 reads it.
- Rate limiting on submission. Dependency mentioned in ROADMAP; lands in a future slice.
- Custom domain / production Clerk tenant for daily-specific concerns. Phase 6 leftover.
- Percentile display. Leaderboard row is written; percentile calculation deferred to the dedicated Leaderboards phase.
- Pack flow (`PLAY#{playerId}` SK schema is forward-compatible but no PACK code lands here).

## Slices

| Slice | Scope | Deliverable |
|-------|-------|-------------|
| **R-8-01** | Backend: schema + repository + handler + 2 crons + sync fallback | API endpoints live, both crons scheduled, schema migrated, lint+test+build green |
| **R-8-02** | Frontend: landing tile + GamePage variant + post-completion screen | Daily flow playable end-to-end, anonymous + signed-in branches working |

R-8-03 (admin curation UI) was previously reserved and is now **dropped** — derived approval makes it unnecessary.

## Success criteria

- A signed-in player can play the daily, submit, see their time, and find their entry on the daily leaderboard.
- An anonymous player can play the daily, submit, see their time, and is told they would need to sign in for the leaderboard.
- After a UTC midnight rollover, today's daily resolves to the candidate puzzle (when yesterday had ≥1 solve) or yesterday's puzzle (when yesterday had zero solves).
- If both crons fail, the first GET after midnight still resolves correctly via the sync fallback — verified by integration test that disables the schedulers.
- `serverElapsedMs` is invariant to client refresh; `assignedAt` recorded on first GET is preserved on subsequent GETs.

## Risks

- **DDB transaction limits**. Submission uses `TransactWriteItems` (2-3 legs). DDB caps per-transaction at 100 items and 4MB; we're far under, but the failure mode (one leg's condition fails → whole transaction rolls back) is documented in design.md.
- **Recycle UX surprise**. Players who solved yesterday and return today on a recycle day see the same puzzle. Document copy makes this explicit (post-completion screen on recycle: "Recycled from yesterday — try a Practice puzzle for a fresh challenge").
- **Cron observability**. Two cron jobs need monitoring. Phase 8 logs structured events; CloudWatch alarms land in a follow-up infra slice.
