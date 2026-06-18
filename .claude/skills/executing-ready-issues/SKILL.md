---
name: executing-ready-issues
description: Use when picking up Ready (Definition-of-Ready) issues from the project board's "Up Next" column to implement them autonomously while the supervisor is away, or when the user says "work the ready issues", "run the backlog", or "pick up Up Next".
---

# Executing Ready Issues

The supervisor batches their involvement into refinement (see the `preparing-ready-issues` and
`refining-an-issue` skills). Once issues are Ready, execution runs against the written DoR specs with the
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
- **Out-of-repo deploy-role prerequisites get an `aws-personal` issue, not just a PR flag.** When a
  hold-open PR needs a deploy-role IAM grant (a new AWS resource type) or an OIDC trust subject (a new
  GitHub Environment), file the prerequisite as an issue on `lesteenman/aws-personal` (which owns the
  `github-actions-deploy` role) with the exact policy statement / OIDC `sub` + a link to the reign PR,
  and link it back from the PR. That's how it gets picked up — don't leave it only as a PR comment the
  supervisor has to transcribe. (See infra/CLAUDE.md lesson 8.)
- **Merge = deploy** (CD ships to acc on merge to main). Autonomous merge is safe in proportion to the
  post-deploy verification gate. **#241 (GitHub Environments + Deployments verification) is the stated
  prerequisite for widening autonomous merge** — until it lands, keep the hold set conservative.

## Merge mechanics (stacked PRs, deploy cadence)

When a batch contains stacked PRs (B based on A) or overlapping files, the order and method matter:

- **Never `--delete-branch` a stacked base until its children are retargeted.** GitHub *closes* (does
  not auto-retarget) a PR whose base branch is deleted, and a closed PR can't be reopened once its base
  is gone — you have to recreate it. Sequence: merge A **keeping its branch** → retarget B's base to
  `main` (`gh pr edit B --base main`) → **rebase B's branch onto `main` and force-push** (a base change
  alone does not fire CI; only a push does) → merge B → then delete the leftover branches.
- **Merge one PR per CD apply.** CD has no apply-serialization unless the `cd-acc` concurrency group is
  in place; rapid merges otherwise race the Terraform state lock and fail. Wait for each merge's CD run
  to drain (`gh run list --workflow CD` shows no in_progress/queued) before merging the next.
- **Overlapping files conflict after the first merge.** Two PRs editing the same file in the same region
  (e.g. #118 GSI + #164 SSE both in `modules/database/main.tf`) — the second goes CONFLICTING once the
  first lands. Rebase it on `main`, resolve to keep both blocks, force-push.

## Subagent dispatch mechanics

The lead orchestrates; subagents do the engineering. These rules keep dispatches from stalling,
colliding, or producing false findings:

- **Dispatch subagents in worktree isolation (`isolation: "worktree"`).** Subagents share the lead's
  checkout by default, so a subagent that runs `git checkout`/`switch` (even a read-only reviewer
  comparing branches) moves the shared working-tree HEAD out from under the lead — and two implementers
  can't run at once without clobbering each other. A per-subagent worktree makes that structurally
  impossible: each gets its own isolated checkout, commits land on the intended branch ref (visible to
  the lead for push), and the worktree auto-cleans if unchanged. This also unlocks **safe parallel
  implementers** for independent, disjoint-file issues — no need to serialize. Reviewers still read the
  diff three-dot and never need to switch branches. (A read-only reviewer on the shared tree silently
  reset HEAD during the packs batch — worktree isolation prevents it.)

- **Implementer prompts forbid long-running verification (> ~5 min); the lead runs the long gates.**
  An implementer that blocks on a soak / property-corpus / full-`-race` / e2e run hits the 600s Bash
  timeout and dies before committing (a finished tree left uncommitted — the lead then commits it as
  housekeeping, never re-engineers it). Scope the implementer to the **fast, CI-matching** checks
  (build, `-short` unit suites, `tsc`, lint) and explicitly hand the long gate (property corpus, soak,
  full e2e) to the orchestrator to run after the code lands. Match CI exactly — e.g. the generator
  suite is `-short` (full `go test ./internal/generator/...` exceeds 600s; see `backend/CLAUDE.md`).
- **Code-review subagents read the diff three-dot, never two-dot.** Give reviewers `gh pr diff <n>` or
  `git diff main...<branch>` (merge-base). A two-dot `git diff main <branch>` on a branch that is behind
  `main` (common for hold-open worktree branches reviewed before a rebase) renders every main-only
  change as a phantom *deletion* — reviewers raise false CRITICALs ("this reverts #X") that a real
  three-way merge would never produce.

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
