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
   Socratic questioning, 2–3 approaches with a recommendation, design presented in sections (its
   "one question at a time" default is adapted in step 2). Pull in `architecture` (design-time drift
   check) and `glossary` (vocab) as that skill directs. **HITL: resolve every design fork with the
   supervisor. Never assume** — an unresolved fork is the thing refinement exists to close.

2. **Redirect brainstorming's terminal steps** (the deliberate override; the skill is authored to
   defer to project preferences, so this works with the grain):
   - Its design-doc default is `docs/superpowers/specs/…` — **instead, write the design into the
     GitHub issue as the DoR comment.**
   - Its terminal step is `writing-plans` — **do NOT invoke it here.** `writing-plans` is the first
     *implementation* step, on the branch, later.
   - Its **"one question at a time"** default → ask the supervisor's design-fork questions through the
     **`AskUserQuestion` tool**. You MAY put multiple questions in one call **only if every question
     belongs to *this* issue.** **Never** include another issue's fork in the same call — a call
     spanning more than one issue is forbidden, however related or trivial. (This is the
     one-issue-at-a-time rule; the `preparing-ready-issues` driver keeps the issues themselves
     sequential — finish and confirm this issue before the next one's conversation starts.)
   - Do **not** create a branch.

3. **Post the Definition of Ready as an issue comment.** Fill every box of the checklist below. Any
   blank box means not Ready. Include the resolved design. Post **only this issue's** DoR — never bundle
   another issue's DoR into the same turn.

4. **Confirm with the supervisor, then move to "Up Next."** Post the DoR, then get the supervisor's
   confirmation of *this* issue's DoR — the per-issue focus checkpoint (see `preparing-ready-issues` →
   One issue at a time). Only then move the card to the project board's "Up Next" column via the
   authenticated `gh project` CLI (project 1, owner `lesteenman`), and only then start the next issue's
   conversation:

   ```
   PROJECT_ID=PVT_kwHOAA6y2s4BXy4s            # project 1, owner lesteenman
   STATUS_FIELD=PVTSSF_lAHOAA6y2s4BXy4szhS9T9Q
   UP_NEXT_OPTION=86c2b848                     # Status: Backlog bbf0514a, Up Next 86c2b848, In Progress e2c394a9, In Review 3f7ebe2b, Done 721d766f
   ITEM=$(gh project item-list 1 --owner lesteenman --limit 200 --format json --jq '.items[] | select(.content.number==<N>) | .id')
   gh project item-edit --project-id "$PROJECT_ID" --id "$ITEM" --field-id "$STATUS_FIELD" --single-select-option-id "$UP_NEXT_OPTION"
   ```

   This needs the `gh` auth (keyring) to carry the Projects scope. (Board moves are deliberately not a
   Taskfile target — the Taskfile is dev-flow only.)

## Definition of Ready (the gate)

An issue is Ready only once **all** of these are settled, together, in refinement. A blank box at
execution time is unclarity — the `executing-ready-issues` skill notifies and holds, never assumes.

- [ ] **Problem + PO rationale** — what, and why (the user value).
- [ ] **Acceptance criteria** — explicit, enumerated, testable. Not "make it work."
- [ ] **Grounding verified against the code** — any claim that data "already exists / is already
      persisted / is available from X" is grepped against the real struct, schema, or handler before
      Ready. If the field/attribute/path isn't actually there, scope **and** risk class must include
      adding it — a framed "read-only surface" can hide a write-path change. **Any specified
      persistence/key design must support the issue's required access patterns** (get / list / query /
      uniqueness) — e.g. a per-entity partition key (`PACK#{slug}`) cannot be listed without a Scan or a
      GSI, so "list all + unique slug + no new GSI" is internally contradictory. Check the key shape
      against every read the issue needs before Ready, or execution hits the contradiction (`#326`).
- [ ] **Data / environment availability checked** — does the acceptance depend on data or an
      environment that does not exist yet (real users, prod, an accrued corpus, real traffic)? If so,
      do **not** post a DoR whose acceptance is unachievable now: scope the issue to what is
      measurable/buildable today and split the data- or prod-gated remainder into a deferred follow-up
      (link it). Parking the whole issue (status `blocked` with the gate named) is also valid. Decide
      this **before** the design conversation, not mid-refinement.
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
- ❌ Asserting that state "already exists" / "is already persisted" without grepping the schema for it
  (e.g. claiming a struct field is stored when no write path sets it) — verify or the spec mis-scopes.
- ❌ A single `AskUserQuestion` call carrying forks for **more than one issue** (cross-issue batching —
  forbidden however related, independent, or trivial; see `preparing-ready-issues` → One issue at a
  time). Same-issue, multi-question calls are fine.
- ❌ Posting more than one issue's DoR in a turn, or posting issue B's DoR before issue A is confirmed.
- ❌ Overriding `superpowers:brainstorming` by name in `.claude/skills/` (forks a tuned skill, loses
  upstream updates). Compose it, don't shadow it.
