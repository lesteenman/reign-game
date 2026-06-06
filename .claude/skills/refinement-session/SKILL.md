---
name: refinement-session
description: Run a refinement session with the supervisor — pick up a small batch of backlog GitHub issues, sequence them by dependency, take each to Definition of Ready (via the refinement skill), and confirm the Ready set so it can move to "Up Next" for autonomous execution. Trigger when the user says "let's refine", "refinement session", "do a refinement batch", or wants to prep backlog issues for an autonomous run. For a single named issue, use the refinement skill directly.
---

# Refinement Session

A refinement session is where the supervisor (team lead + Product Owner) batches their involvement.
The session picks up several issues together, refines each to Definition of Ready, and hands a Ready
set to autonomous execution. This is the *design* half of the contract; execution is the
`autonomous-execution` skill.

The supervisor makes the calls. I prep — pre-analyze candidates, surface ambiguities, options, and
dependencies, draft the Definition-of-Ready fills — so the session is decision-making, not discovery.

## Procedure

1. **Pick the batch.** Start with 2–3 issues; expand only once calibration supports it (see below).
   **Sequence by dependency, independent issues first** so completed work banks before any surprise
   hang. Choose batches that **minimize PR stacking** — prefer independent breadth over stacked depth.

2. **Refine each issue.** Run the `refinement` skill per issue. That skill owns the per-issue work:
   the design conversation, the Definition-of-Ready checklist, and posting the DoR comment to GitHub.
   Refine the batch **together with the supervisor** — this is where HITL lives. Resolve every design
   fork now; an unresolved fork is the thing refinement exists to close.

3. **Confirm the Ready set.** A session ends when each issue has its DoR comment posted and the
   supervisor confirms. The issues move to "Up Next." Hand off to `autonomous-execution`.

GitHub is the source of truth for progress — a context reset is recoverable from issue + PR state.

## Batch sizing (calibration)

Treat the first 2–3 batches as **calibration, not production.** Start small. Measure rework rate (PRs
needing rework) and stall cost (how often and how long the autonomous run holds on a surprise). Expand
batch size only as the data supports it. The `retro` skill tunes the Definition of Ready from whatever
forks slipped through.

Revisit any "advance to a demonstrably-independent Ready issue while another is parked" exception only
if calibration shows sequential-hold stalls are expensive. Earn it with data.
