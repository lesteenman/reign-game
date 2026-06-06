---
name: autonomous-execution
description: Trigger when picking up Ready issues from the project board's "Up Next" column for an autonomous implementation run, or when the user says "work the ready issues", "run the backlog", or "pick up Up Next". Runs Ready issues one PR at a time with the full Change Workflow gates, notify-and-hold on surprises, autonomous merge for low-risk work, and an end-of-session digest.
---

# Autonomous Execution

The supervisor batches their involvement into refinement (see the `refinement-session` and
`refinement` skills). Once issues are Ready, execution runs against the written DoR specs with the
supervisor away. This skill is the execution half of the contract. It does **not** relax the HITL
rule — refinement front-loads the design forks; this defines what to do with the few that still
surface.

Shape: **work Ready issues sequentially → autonomous merge for low-risk, hold-open for the risk set →
notify-and-hold on residual unclarity → end-of-session digest.**

## Execution contract

Work the Ready issues one after another, a PR per issue. Each PR runs the full Change Workflow gates
(integration verification, `requesting-code-review`, the `security-review` skill when the deep-review
trigger fires). Autonomy changes *who decides design*, not *whether the gates run*.

- **Notify-and-hold on any residual unclarity.** If execution uncovers something refinement didn't
  settle — an ambiguous criterion, a contract that doesn't behave as specced, a missed design
  implication — `PushNotification` immediately and **hold until answered. Do not assume, do not skip
  ahead.** Strict sequential hold: one surprise stalls the run until the supervisor answers. The only
  lever that buys duration back is Definition-of-Ready quality.
- **Park-and-continue applies to exactly one case:** a *completed* PR that is non-blocking for
  downstream work but needs supervisor approval to merge (the risk set below). Leave it open and move
  to the next independent issue. This is the only time work moves on with something outstanding.
- **Issue is wrong, not just unclear** — mis-scoped, duplicate, should-be-split, obsoleted: route to
  the supervisor as Product Owner (notify + hold). Never silently redefine or tidy the backlog.

## Merge authority

- **Merge autonomously** a PR that is fully green — CI + `requesting-code-review` + security gate —
  **and outside the hold set.**
- **Hold-for-supervisor-merge set = the Security Deep-Review Trigger list** (auth, middleware, Lambda
  handlers, dependency manifests, `infra/**/*.tf`, `.github/workflows/**`, anything with
  `secret`/`token`/`key`/`credential`) **∪ any change that could materially affect AWS cost.** These
  stay open for explicit approval.
- Also held: any PR where my own code-review or security pass surfaced a finding I could **not** fully
  resolve.
- **Merge = deploy** (CD ships to acc on merge to main). Autonomous merge is safe in proportion to the
  post-deploy verification gate. **#241 (GitHub Environments + Deployments verification) is the stated
  prerequisite for widening autonomous merge** — until it lands, keep the hold set conservative.

## Re-entry digest

When the supervisor returns, deliver a **single digest** (not "go read N PRs cold"), via
`PushNotification` + session summary:

- **Merged** — what shipped, and therefore deployed to acc.
- **Open PRs awaiting you** — each with its hold reason (risk class / unresolved finding) and a
  one-line review pointer.
- **Parked / held questions** — any notify-and-hold still outstanding, with the question, the options,
  and my recommendation, so it's answerable in one line.
- **Retro flags** — forks I hit that Definition of Ready should have caught (feeds back into the DoR
  checklist), stalls, anything that smelled like drift.

## HITL reconciliation

The HITL Rule is **unchanged**: the supervisor clarifies, I don't assume. This contract changes only
*when* the clarifying happens — design forks resolve up front in batched refinement, so fewer surface
mid-execution. The autonomy granted is over **execution decisions inside a Ready issue**, never over
design forks or scope. When genuinely unsure whether something is a small clarification or a real
fork: **park and ask.** Holding has been made cheap on purpose.
