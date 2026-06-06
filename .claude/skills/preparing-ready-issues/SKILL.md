---
name: preparing-ready-issues
description: Use when prepping a batch of backlog GitHub issues for autonomous execution — the user says "let's refine", "refinement session", "do a refinement batch", or points at several issues to take to Definition of Ready as a batch. For refining a single named issue, use the refining-an-issue skill directly.
---

# Preparing Ready Issues

A refinement session is where the supervisor (team lead + Product Owner) batches their involvement.
The session picks up several issues together, refines each to Definition of Ready, and hands a Ready
set to autonomous execution. This is the *design* half of the contract; execution is the
`executing-ready-issues` skill.

The supervisor makes the calls. I prep — pre-analyze candidates, surface ambiguities, options, and
dependencies, draft the Definition-of-Ready fills — so the session is decision-making, not discovery.

## Procedure

1. **Pick the batch.** **Sequence by dependency, independent issues first** so completed work banks
   before any surprise hang. Choose batches that **minimize PR stacking** — prefer independent breadth
   over stacked depth.

2. **Refine each issue — strictly one at a time, sequentially.** Run the `refining-an-issue` skill per
   issue. Fully complete one issue's design conversation **and get the supervisor's confirmation of its
   DoR before starting the next issue's conversation.** Never run several issues' design discussions
   together. Prep (pre-analysis, reading code) MAY span the whole batch — that's invisible homework;
   the **supervisor interaction is what is sequential.** That skill owns the per-issue work: the design
   conversation, the Definition-of-Ready checklist, and posting the DoR comment to GitHub. Refine
   **together with the supervisor** — this is where HITL lives. Resolve every design fork now; an
   unresolved fork is the thing refinement exists to close. See **One issue at a time** below — it is
   non-negotiable.

3. **Confirm the Ready set.** A session ends when each issue has its DoR comment posted and the
   supervisor confirms. (Each issue was already confirmed per step 2 — this is the closing handoff, not
   a second gate.) The issues move to "Up Next." Hand off to `executing-ready-issues`.

GitHub is the source of truth for progress — a context reset is recoverable from issue + PR state.

## One issue at a time — non-negotiable

Refining a batch does **not** mean refining issues in parallel. Each issue gets its own focused design
session the supervisor can verify in detail, start to finish, **before the next begins.**

**Questions go through the `AskUserQuestion` tool.** A single call MAY carry multiple questions **only
if every question belongs to the same issue.** An `AskUserQuestion` call that references **more than one
issue is the violation** — it is the exact shape of cross-issue batching, forbidden no matter how
related, independent, or trivial the issues are. Likewise: post **one** issue's DoR and get its
confirmation before the next issue's conversation — **never post multiple DoRs in one turn.**

**Violating the letter of this rule is violating the spirit of it.** "Fewer round-trips for a busy
supervisor" is not the goal; per-issue focus the supervisor can verify is.

### Rationalizations — STOP if you think any of these

| Excuse | Reality |
|--------|---------|
| "Minimal round-trips for a busy supervisor → put all forks in one call" | The metric is per-issue focus, not call count. Fewer calls at the cost of focus IS the failure this rule prevents. |
| "These issues are independent — nothing gained by serializing them" | Independence governs *execution* sequencing, not *refinement* focus. Each issue gets its own call(s) and its own confirmation. |
| "I'll post all the DoRs together once the decisions land" | One DoR, one confirmation, then the next issue. Never multiple DoRs in a turn. |
| "Batching is more efficient" | Efficiency is not the metric; a thorough, verifiable per-issue design is. |
| "I'll just group the trivial / related ones into one call" | Same-issue grouping only. Two issues never share an `AskUserQuestion` call, however related or trivial. |

### Red flags — you are about to violate the rule

- An `AskUserQuestion` call listing more than one issue number.
- A DoR posted for issue B while issue A is unconfirmed.
- More than one DoR comment posted in a single turn.
- The thought "to save the supervisor time, I'll ask everything at once."

## Batch sizing

Pick a batch you can refine well — typically a handful of issues. **Sequence by dependency, independent
issues first**, and prefer independent breadth over stacked depth to minimize PR stacking. The `retro`
skill tunes the Definition of Ready from whatever forks slipped through; the workflow keeps improving
from there. Size up or down as the work warrants — but always refine the batch one issue at a time
(above).
