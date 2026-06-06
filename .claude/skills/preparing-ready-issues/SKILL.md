---
name: preparing-ready-issues
description: Use when prepping a batch of backlog GitHub issues for autonomous execution — the user says "let's refine", "refinement session", "do a refinement batch", or points at several issues to take to Definition of Ready together. For refining a single named issue, use the refining-an-issue skill directly.
---

# Preparing Ready Issues

A refinement session is where the supervisor (team lead + Product Owner) batches their involvement.
The session picks up several issues together, refines each to Definition of Ready, and hands a Ready
set to autonomous execution. This is the *design* half of the contract; execution is the
`executing-ready-issues` skill.

The supervisor makes the calls. I prep — pre-analyze candidates, surface ambiguities, options, and
dependencies, draft the Definition-of-Ready fills — so the session is decision-making, not discovery.

## Procedure

1. **Pick the batch.** Start with 2–3 issues; expand only once calibration supports it (see below).
   **Sequence by dependency, independent issues first** so completed work banks before any surprise
   hang. Choose batches that **minimize PR stacking** — prefer independent breadth over stacked depth.

2. **Refine each issue.** Run the `refining-an-issue` skill per issue. That skill owns the per-issue work:
   the design conversation, the Definition-of-Ready checklist, and posting the DoR comment to GitHub.
   Refine the batch **together with the supervisor** — this is where HITL lives. Resolve every design
   fork now; an unresolved fork is the thing refinement exists to close.

3. **Confirm the Ready set.** A session ends when each issue has its DoR comment posted and the
   supervisor confirms. The issues move to "Up Next." Hand off to `executing-ready-issues`.

GitHub is the source of truth for progress — a context reset is recoverable from issue + PR state.

## Batch sizing (calibration)

Treat the first 2–3 batches as **calibration, not production.** Start small. Measure rework rate (PRs
needing rework) and stall cost (how often and how long the autonomous run holds on a surprise). Expand
batch size only as the data supports it. The `retro` skill tunes the Definition of Ready from whatever
forks slipped through.

Revisit any "advance to a demonstrably-independent Ready issue while another is parked" exception only
if calibration shows sequential-hold stalls are expensive. Earn it with data.
