---
name: refining-an-issue
description: Use when taking a single tracked backlog GitHub issue to Definition of Ready before implementation — the user says "refine #N", or wants to settle one issue's acceptance criteria, scope, or open design questions, or the preparing-ready-issues skill is processing a batch. Not for improving or optimizing code, algorithms, or performance — this shapes an issue's spec, it does not change code. For a whole batch of issues, use preparing-ready-issues instead.
---

# Refining an Issue

Refinement turns one backlog issue into a **Definition of Ready** spec that the implementation phase
can run against autonomously. It is **always done with the supervisor** — this is where HITL lives.
For picking up and sequencing a batch, the `preparing-ready-issues` skill drives; it calls this skill per
issue. Execution against the Ready spec is the `executing-ready-issues` skill.

## The one rule that makes this different from plain brainstorming

**Refinement produces a Definition-of-Ready comment on the GitHub issue. It creates NO branch, NO
commits, and NO `docs/superpowers/specs/` file.** The issue is the canonical Ready record (GitHub =
source of truth). Branch, plan, and code are the *implementation* phase, which is separate.

## Procedure

1. **Run the design conversation via `superpowers:brainstorming`.** Use it for what it's good at:
   Socratic, one question at a time, 2–3 approaches with a recommendation, design presented in
   sections. Pull in `architecture` (design-time drift check) and `glossary` (vocab) as that skill
   directs. **HITL: resolve every design fork with the supervisor. Never assume** — an unresolved fork
   is the thing refinement exists to close.

2. **Redirect brainstorming's terminal steps** (the deliberate override; the skill is authored to
   defer to project preferences, so this works with the grain):
   - Its design-doc default is `docs/superpowers/specs/…` — **instead, write the design into the
     GitHub issue as the DoR comment.**
   - Its terminal step is `writing-plans` — **do NOT invoke it here.** `writing-plans` is the first
     *implementation* step, on the branch, later.
   - Do **not** create a branch.

3. **Post the Definition of Ready as an issue comment.** Fill every box of the checklist below. Any
   blank box means not Ready. Include the resolved design.

4. **Confirm Ready with the supervisor.** Refinement ends when the DoR comment is posted and the
   supervisor confirms. The issue can move to "Up Next."

## Definition of Ready (the gate)

An issue is Ready only once **all** of these are settled, together, in refinement. A blank box at
execution time is unclarity — the `executing-ready-issues` skill notifies and holds, never assumes.

- [ ] **Problem + PO rationale** — what, and why (the user value).
- [ ] **Acceptance criteria** — explicit, enumerated, testable. Not "make it work."
- [ ] **Design forks resolved** — every option-decision made and recorded. No open "should we X or Y."
- [ ] **Test-plan sketch** — what proves it: unit / integration / e2e. Cross-boundary contracts
      flagged for real-wire verification (Change Workflow integration step).
- [ ] **Edge cases enumerated.**
- [ ] **Out-of-scope** stated explicitly.
- [ ] **Dependencies + sequencing** — which issues it depends on; independent vs stacked.
- [ ] **Risk class tagged** — does it hit the hold-for-merge set? Determines merge vs hold-open.
- [ ] **Boundaries crossed** — which service boundaries it touches (drives integration verification).

## Then what

Implementation is a **separate phase** (the `executing-ready-issues` skill). At implementation start:
create the branch, run `superpowers:writing-plans` (plan doc on the branch), execute, open the PR.
Surface only via notify-and-hold on a genuine surprise, plus the end-of-session digest.

## Anti-patterns

- ❌ Creating a branch or committing a spec file during refinement (premature; clutters git).
- ❌ Invoking `writing-plans` as part of refinement (that's implementation).
- ❌ Answering your own design fork to "keep moving" (violates HITL — notify-and-hold instead).
- ❌ Overriding `superpowers:brainstorming` by name in `.claude/skills/` (forks a tuned skill, loses
  upstream updates). Compose it, don't shadow it.
