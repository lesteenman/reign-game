---
name: refinement
description: Take one or a small batch of backlog GitHub issues to Definition of Ready, together with the supervisor (HITL), BEFORE any implementation. Composes superpowers:brainstorming for the design conversation but redirects its output to the GitHub issue and creates no branch/commits. Trigger when the user says "let's refine", "refine #N", "refinement session", or picks backlog issues to prep for autonomous execution.
---

# Refinement

Refinement turns backlog issues into **Definition of Ready** specs the implementation phase can run against autonomously. It is the *design* half of the Refinement + Autonomous Execution Contract (`/CLAUDE.md`). It is **always done together with the supervisor** — this is where HITL lives.

## The one rule that makes this different from plain brainstorming

**Refinement produces a Definition-of-Ready comment on the GitHub issue. It creates NO branch, NO commits, and NO `docs/superpowers/specs/` file.** The issue is the canonical Ready record (GitHub = source of truth). Branch + plan + code are the *implementation* phase, which is separate.

## Procedure

1. **Pick the batch.** One issue, or a small batch (2–3; expand only after calibration). Sequence by dependency, independent issues first; choose batches that **minimize PR stacking**.

2. **Run the design conversation via `superpowers:brainstorming`.** Use it for what it's good at — Socratic, one question at a time, propose 2–3 approaches with a recommendation, present the design in sections. Pull in `architecture` (design-time drift check) and `glossary` (vocab) as that skill directs. **HITL: resolve every design fork with the supervisor. Never assume** — an unresolved fork is the thing refinement exists to close.

3. **Redirect brainstorming's terminal steps** (this is the deliberate override; the skill is *authored to defer to project preferences*, so this is working with the grain, not forking it):
   - Its design-doc default is `docs/superpowers/specs/…` — **instead, write the design into the GitHub issue as the DoR comment.**
   - Its terminal step is `writing-plans` — **do NOT invoke it here.** `writing-plans` is the first *implementation* step, on the branch, later.
   - Do **not** create a branch (Change Workflow step 2 is deferred to implementation).

4. **Post the Definition of Ready as an issue comment.** Fill every box of the DoR checklist in `/CLAUDE.md` §1 (problem + PO rationale, acceptance criteria, design forks *resolved*, test-plan sketch, edge cases, out-of-scope, dependencies + sequencing, risk class, boundaries crossed). Any blank box = not Ready. Include the resolved design.

5. **Confirm Ready with the supervisor.** Refinement ends when the DoR comment is posted and the supervisor confirms. The issue can now move to "Up Next."

## Then what

Implementation is a **separate phase** (see `/CLAUDE.md` → Refinement + Autonomous Execution Contract §3–§5). At implementation start: create the branch, run `superpowers:writing-plans` (plan doc on the branch, optionally a committed design doc too), execute, open the held PR. Surface only via notify-and-hold on a genuine surprise, plus the end-of-session digest.

## Anti-patterns

- ❌ Creating a branch or committing a spec file during refinement (premature; clutters git, especially for batches).
- ❌ Invoking `writing-plans` as part of refinement (that's implementation).
- ❌ Answering your own design fork to "keep moving" (violates HITL — notify-and-hold instead).
- ❌ Overriding `superpowers:brainstorming` by name in `.claude/skills/` (forks a tuned skill, loses upstream updates). Compose it, don't shadow it.
