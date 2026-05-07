# Phase 8 Daily Puzzle — Design Grill Summary

Stress-tests of the locked decisions in `proposal.md`. Each finding has a severity, problem statement, and proposed resolution. Findings flagged "user input" need explicit confirmation before artifact lock-in.

### Finding 1 — Two-phase cron gap and sync-fallback completeness

**Severity:** MAJOR

**The grill:** Between T-6h (candidate top-up) and T=0 (swap/confirm), six hours pass during which the approval pool, yesterday's `solved` count, and even the candidate slot itself can change (a separate retry could overwrite it). If T-6h fires but T=0 does not, the first GET fallback at 00:01 UTC must replicate BOTH cron jobs, not just T=0 — otherwise the candidate could be stale relative to the latest approvals. The proposal says "first GET runs the same algorithm" but does not specify which algorithm (T-6h, T=0, or both).

**Resolution:** Mitigate. The sync fallback runs the **T=0 finalize algorithm only**, treating whatever is in the candidate slot as authoritative. T-6h is a freshness optimization, not a correctness requirement; if T-6h missed, the candidate is at worst a few hours older than ideal but still valid. Document this explicitly in `design.md` as the contract: "T-6h ensures freshness; T=0 (or its sync equivalent on first GET) is the only correctness-critical step."

### Finding 2 — Candidate slot atomicity under duplicate cron firings

**Severity:** MAJOR

**The grill:** EventBridge / Lambda Scheduler can fire a cron twice on rare occasions (at-least-once delivery). Two T-6h invocations could each pick a different approved puzzle and race on `PutItem` against `DAILY-CANDIDATE`. Without a condition expression, the loser overwrites the winner and the `lastDailyDate` field on one PuzzleRecord is set without that puzzle being scheduled.

**Resolution:** Accept with mitigation. Use `PutItem` with `ConditionExpression: attribute_not_exists(PK) OR puzzleId = :sameId` on the candidate slot, and **only stamp `lastDailyDate` on the puzzle record AFTER the candidate write succeeds**. The `lastDailyDate` write itself uses a conditional update that asserts no other date has claimed it within the last 14 days (recycle window).

### Finding 3 — PLAY row first-GET creation must be idempotent

**Severity:** MAJOR

**The grill:** The first GET stamps `assignedAt`. Refresh (browser reload), opening on a second device while signed in, or simply re-opening the page after a GamePage navigation must NOT reset `assignedAt` — that would let any player game the timer by refreshing right before submitting. Anonymous users on a single device with localStorage cleared mid-play likewise must not be able to "reroll" their own clock.

**Resolution:** Accept. PLAY row creation uses `PutItem` with `ConditionExpression: attribute_not_exists(PK)`. Subsequent GETs do an `UpdateItem` that touches only `lastSeenAt` (advisory) and never `assignedAt`. Anonymous deviceId rotation is treated as a NEW player — the old assignment is lost (consistent with anonymous identity contract; see Finding 5).

### Finding 4 — Submission write atomicity across 3 items

**Severity:** MAJOR

**The grill:** Submission must update PLAY (set `outcome=solved`, `serverElapsedMs`, `submittedAt`), increment the DAILY counter (`solved += 1`), and (signed-in only) write LEADERBOARD. If these are 3 separate writes, a crash mid-write leaves PLAY=solved but LEADERBOARD missing, or DAILY counter incremented twice on retry. Counter increment under retry is the worst — it directly biases tomorrow's recycle decision.

**Resolution:** Accept. Use `TransactWriteItems` (2 legs anonymous, 3 legs signed-in). The PLAY leg includes `ConditionExpression: outcome = :started` to guarantee idempotency on retry — a second submission attempt fails the condition and the whole transaction rolls back without double-counting. Documented in proposal Risks; design.md spells out the exact transaction shape.

### Finding 5 — Anonymous deviceId loss on localStorage clear

**Severity:** MINOR

**The grill:** Clearing localStorage (incognito mode, browser reset, "clear site data") regenerates deviceId. The player loses their daily history, can replay today's puzzle from scratch, and the `started` PLAY row from the prior identity is dangling. This is exploitable for time-grinding (start, refresh deviceId, retry until fast).

**Resolution:** Accept. Phase 8 explicitly does not enforce anti-cheat. The leaderboard is signed-in only, so deviceId churn cannot affect leaderboard placement. Anonymous players gaming their own local time is acceptable as a known limitation; signed-in players are bound to userId. Documented in proposal Risks under a new "Anonymous identity churn" bullet.

### Finding 6 — `lastDailyDate` race between cron and recycle

**Severity:** MINOR

**The grill:** On a recycle day (yesterday's solved=0), today's daily reuses yesterday's puzzle. The candidate slot stays untouched for tomorrow. But the same puzzle's `lastDailyDate` was already set to yesterday's date when it was first scheduled — recycling does not advance it. If a third operator (manual admin reseed, future feature) reads `lastDailyDate` to find "puzzles unused in last N days", the recycle-eligible puzzle wrongly appears stale.

**Resolution:** Mitigate. On recycle, advance `lastDailyDate` to today's date as part of the T=0 transaction. Cost: one extra UpdateItem leg in the recycle path. Trivial to add and prevents future-phase confusion. Captured as a tasks.md item.

### Finding 7 — Recycle invariant when sync fallback fires late

**Severity:** MINOR

**The grill:** If both crons fail and the first GET happens at 00:05 UTC, the sync fallback reads yesterday's `solved` count and either confirms the candidate or recycles. But what if a player solved yesterday's puzzle at 23:59:30 UTC and the submission's transaction committed at 00:00:02 UTC (after midnight)? The `solved` counter for that date is still incremented (the submission writes against the daily date the player was assigned to, not wall-clock submission time), so the count is correct. Verify this is explicitly stated.

**Resolution:** Accept. Counter increments key off the date in the PLAY row (the date the player was assigned), not `submittedAt`. Add an integration test for this exact scenario (submit at 00:00:02 UTC after assigning at 23:55:00 UTC the prior day) — the counter on the prior date must still be incremented.

### Finding 8 — Date-string validation against server UTC

**Severity:** MINOR

**The grill:** Client passes `/api/daily/2026-05-01`. Client wall clock could be skewed (timezone bug, broken NTP, deliberate manipulation). Server must validate: only "today UTC" or "yesterday UTC" (for late-night submissions of yesterday's daily) are accepted; future dates and dates older than yesterday return 404.

**Resolution:** Accept. Handler validates `date IN { todayUTC, yesterdayUTC }` else 404. The two-day window covers the legitimate case where a player started yesterday's daily at 23:55 UTC and submits at 00:02 today.

### Finding 9 — Approval pool empty during T-6h

**Severity:** MINOR

**The grill:** If the approved pool is empty at T-6h (generator regression, all candidates verdict-rejected), T-6h has nothing to write. Then at T=0 if `solved >= 1` ("confirm candidate"), there is no candidate to confirm. Spec must say: T=0 falls back to recycle when candidate slot is empty, regardless of solved count.

**Resolution:** Accept. T=0 algorithm: `if candidate empty → recycle`; `else if solved == 0 → recycle`; `else → confirm candidate`. This makes the empty-pool failure mode degrade gracefully (yesterday's puzzle re-runs) rather than 500-ing the daily endpoint. Generator pool depletion is a Phase 7 concern with its own alarms.

### Finding 10 — Dangling `started` PLAY rows

**Severity:** INFO

**The grill:** A player who taps the daily, GETs the puzzle, and never returns leaves a PLAY row with `outcome=started` forever. Across millions of players over years this is unbounded growth. Cleanup or accept?

**Resolution:** Defer. DDB on-demand pricing makes the cost negligible at projected scale (10^4 players/day × 365 days × maybe 20% drop-off = ~7 × 10^5 dangling rows/year, ~$0.02/year storage). A TTL attribute can be added in Phase 9+ if growth surprises us. Captured in design.md as a known and accepted limitation.

## Summary

10 findings: **4 MAJOR**, **5 MINOR**, **1 INFO**. All have concrete resolutions and none require user input before locking artifacts — every MAJOR is mitigated by a specific technical mechanism (conditional writes, TransactWriteItems, idempotency conditions) that fits cleanly in `design.md` and `tasks.md`. Findings 1, 6, and 7 add concrete invariants to the design doc; Findings 2, 3, 4 add explicit DDB condition expressions to repository tasks; Finding 10 is documented and deferred. Ready to proceed to design.md / tasks.md / specs in subsequent chunks.
