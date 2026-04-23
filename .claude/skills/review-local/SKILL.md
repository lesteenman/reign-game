---
description: "Local code review and cleanup of uncommitted or branch changes. Use when the user says '/review', 'review my changes', 'review this branch', '/simplify', or wants a code review without a GitLab MR. Analyzes the current diff for security, efficiency, code quality, and reuse -- then automatically fixes actionable issues."
user_invocable: true
---

# Local Code Review & Fix

Review the current working changes (or branch diff from main) for issues across five categories, then fix what's actionable.

## Phase 1: Identify Changes

**Always fetch first** — stale refs will make you miss or misattribute changes:

```bash
git fetch --prune origin main
```

Then collect the full set of changes on the branch, not just the current session's diff:

1. `git diff main...HEAD` — the branch-vs-base diff. This is what the PR will contain and what human reviewers see. **Use this as the primary review scope.**
2. `git diff` / `git diff --cached` — uncommitted + staged changes. Review these too; they're about to join the branch diff.
3. `git log --oneline main..HEAD` — the commit list. Scan for prior-session commits you didn't personally author this session. **Do not narrow the review to only the current session's commits** — prior commits that never got reviewed (fast-merged WIP, commits made before /review existed on this branch, commits from another agent's session) can ship unchecked unless this skill catches them. Pre-push hooks have caught lint issues in prior-session commits more than once; this skill should surface higher-value findings in those same commits.

If `git diff main...HEAD` is empty AND the working tree is clean, tell the user there's nothing to review.

## Phase 2: Launch Five Review Agents in Parallel

Use the Agent tool to launch all five agents concurrently in a single message. Pass each agent the full diff so it has the complete context.

### Agent 1: Security Review

For each change:

1. **OWASP Top 10** -- injection, auth bypass, data exposure, XSS, insecure deserialization
2. **Access control** -- missing authorization checks, IDOR risks, frontend-only enforcement without backend gate
3. **Data exposure** -- sensitive fields in API responses, secrets in committed files, verbose error messages
4. **Input validation** -- missing validation annotations, unbounded inputs, path traversal

### Agent 2: Code Reuse Review

For each change:

1. **Search for existing utilities and helpers** that could replace newly written code. Look for similar patterns elsewhere in the codebase.
2. **Flag any new function that duplicates existing functionality.** Suggest the existing function to use instead.
3. **Flag any inline logic that could use an existing utility** -- hand-rolled string manipulation, manual path handling, custom environment checks.
4. **Intra-function structural duplication**: near-identical blocks within a single function (e.g., the same logic repeated for rows, columns, and regions). Suggest extracting a shared helper even if it's only called from one place.

### Agent 3: Code Quality Review

**Spec-awareness (if project uses OpenSpec):** Before reviewing, check if an OpenSpec change exists for this work (`openspec/changes/*/`). If so, read the spec files to understand the intended behavior. When a local review "fix" would contradict the spec, flag it as a finding instead of auto-fixing.

Review the same changes for hacky patterns and clean code violations:

1. **Redundant state**: state that duplicates existing state, cached values that could be derived
2. **Parameter sprawl**: adding new parameters instead of generalizing or restructuring
3. **Copy-paste with slight variation**: near-duplicate code blocks that should be unified
4. **Leaky abstractions**: exposing internal details that should be encapsulated. Watch for Law of Demeter violations (`a.getB().getC().doSomething()`) -- callers reaching through object chains to access distant internals
5. **Stringly-typed code**: using raw strings where constants, enums, or branded types already exist
6. **Poor naming**: names that don't reveal intent (`d` instead of `elapsedTimeInDays`), misleading names (`accountList` for a Map), vague distinctions (`ProductData` vs `ProductInfo`), unpronounce­able or unsearchable names. Function names should be verbs, class/type names should be nouns
7. **Comments that compensate for bad code**: comments explaining WHAT the code does (the code itself should be clear enough). Redundant comments restating the code. Commented-out code. Exception: legal headers, intent clarification for external libraries, and TODOs are acceptable
8. **Oversized functions**: functions exceeding ~80 lines, doing more than one thing, or mixing abstraction levels (e.g., high-level orchestration interleaved with low-level bit manipulation). Flag with a suggested decomposition. Also flag functions with 3+ parameters -- consider an options struct or builder
9. **Poor formatting**: related code separated by unrelated code (vertical density violation), variables declared far from their usage, file not structured top-down (high-level public API should come before private helpers -- the newspaper metaphor)

### Agent 4: Efficiency Review

Review the same changes for efficiency:

1. **Unnecessary work**: redundant computations, repeated file reads, duplicate network/API calls, N+1 patterns
2. **Missed concurrency**: independent operations run sequentially when they could run in parallel
3. **Hot-path bloat**: new blocking work added to startup or per-request hot paths
4. **Memory**: unbounded data structures, missing cleanup, event listener leaks
5. **Overly broad operations**: reading entire files when only a portion is needed

### Agent 5: Doc Drift Review

The top-level docs (`ROADMAP.md`, `PROJECT_STRUCTURE.md`, `CLAUDE.md`, `GLOSSARY.md`, runbooks under `docs/runbooks/`, and the active OpenSpec change's `tasks.md` + `proposal.md` + `specs/`) need to move with the code. Drift is invisible per-slice but crippling at epic close.

Read the diff plus the top-level docs, then flag:

1. **Renamed / moved paths referenced by the old path.** If the diff renames, moves, or deletes a file or directory, grep every `.md` in the repo root + `docs/` + `openspec/` for the old path. Example from Phase 5: `backend/internal/generator/v2/` was merged up to `backend/internal/generator/`, but the spec, proposal, and a test comment still cited the `v2/` subdir weeks later.
2. **Pre-declared IDs that collide with shipped IDs.** If the diff ships a task ID, slice ID, or KI number, grep `ROADMAP.md` for the same ID in a *later* phase block — pre-declared future IDs are often forgotten and create silent collisions. Phase 5 shipped R-062..R-06D; ROADMAP Phase 6/7/8 still had R-063..R-068 pre-declared until epic-close.
3. **tasks.md status column lies.** For any slice whose artifacts exist on disk, the `tasks.md` Status column should be `[x]`. Walk each row and compare against the filesystem. Stale `[ ]` rows are a common finding.
4. **PROJECT_STRUCTURE.md omits new files/directories.** Every `cmd/*` directory, every top-level new file, every test-infrastructure layout change should appear in the tree listing. If the diff adds `backend/cmd/reproduce/` but `PROJECT_STRUCTURE.md` doesn't list it, flag.
5. **KI "Related" column or runbook references broken IDs.** Cross-refs to slice IDs that no longer exist (typos or renumberings) mislead future readers.
6. **Future-looking phrasing after the thing happened.** Text like "R-06B will introduce" or "the new generator will replace the old" that survived into a post-cutover world. Flag and rewrite past-tense.

Flag each as MAJOR (doc states something demonstrably false) or MINOR (stale but not misleading). Doc drift rarely rises to CRITICAL.

## Phase 3: Triage

Before presenting findings to the user, **verify each CRITICAL and MAJOR by opening the cited file at the cited line.** Agents occasionally hallucinate findings — a claim that a test "asserts nothing" or a handler "leaks raw errors" must be checked against the actual code. A false positive wastes a triage turn with the user and erodes trust in the whole review.

For each CRITICAL / MAJOR:

1. Read the cited file + line range.
2. Confirm the claim holds. If not, mark as a false positive in the report with one sentence of evidence ("M1: all 500 paths already call `writeError(..., sanitized-message)`; raw error is only logged, not returned.") and drop it from the consolidated list.
3. If the finding holds but the severity is wrong, re-grade it (e.g. a CRITICAL that's actually dependent on an unshipped code path may be MAJOR or MINOR).

Do not triage MINORs — the volume is too high and the cost of a false-positive MINOR is near zero.

## Phase 4: Fix Issues

Wait for all five agents to complete. Aggregate their verified findings into a structured report:

```
## Review Summary

### Security (X findings)
- [SEVERITY] file:line -- description

### Efficiency (X findings)
- [SEVERITY] file:line -- description

### Code Quality (X findings)
- [SEVERITY] file:line -- description

### Reuse Opportunities (X findings)
- [SEVERITY] file:line -- description

### Doc Drift (X findings)
- [SEVERITY] file:line -- description
```

Then **fix each actionable issue directly**. If a finding is a false positive or not worth addressing, note it and move on.

After fixing, run the project's test suite to verify no regressions. Commit the fixes if tests pass.

When done, briefly summarize what was fixed (or confirm the code was already clean).

## Severity Levels
- **CRITICAL** -- security vulnerability or data loss risk, must fix
- **MAJOR** -- bug, significant performance issue, or spec violation
- **MINOR** -- code quality improvement, nice-to-have
- **INFO** -- observation, no action needed

## Security Escalation (MANDATORY)

CRITICAL or HIGH security findings from Agent 1 **block merge**. They must be fixed before proceeding. After fixing, re-run the security review agent to confirm resolution. If a fix cannot be determined, escalate to the human.

## Pre-Commit Secret Scan (MANDATORY)

Before committing any fixes from this review, run:

```bash
gitleaks detect --source .
```

If secrets are found, resolve them before committing. Never commit API keys, tokens, passwords, or high-entropy strings.
