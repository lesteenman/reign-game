---
description: "Local code review and cleanup of uncommitted or branch changes. Use when the user says '/review', 'review my changes', 'review this branch', '/simplify', or wants a code review without a GitLab MR. Analyzes the current diff for security, efficiency, code quality, and reuse -- then automatically fixes actionable issues."
user_invocable: true
---

# Local Code Review & Fix

Review the current working changes (or branch diff from main) for issues across four categories, then fix what's actionable.

## Phase 1: Identify Changes

Run `git diff` (or `git diff HEAD` if there are staged changes) to see what changed. If the working tree is clean, use `git diff main...HEAD`. If both are empty, tell the user there's nothing to review.

## Phase 2: Launch Four Review Agents in Parallel

Use the Agent tool to launch all four agents concurrently in a single message. Pass each agent the full diff so it has the complete context.

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

### Agent 3: Code Quality Review

**Spec-awareness (if project uses OpenSpec):** Before reviewing, check if an OpenSpec change exists for this work (`openspec/changes/*/`). If so, read the spec files to understand the intended behavior. When a local review "fix" would contradict the spec, flag it as a finding instead of auto-fixing.

Review the same changes for hacky patterns:

1. **Redundant state**: state that duplicates existing state, cached values that could be derived
2. **Parameter sprawl**: adding new parameters instead of generalizing or restructuring
3. **Copy-paste with slight variation**: near-duplicate code blocks that should be unified
4. **Leaky abstractions**: exposing internal details that should be encapsulated
5. **Stringly-typed code**: using raw strings where constants, enums, or branded types already exist
6. **Unnecessary comments**: comments explaining WHAT the code does (well-named identifiers already do that)

### Agent 4: Efficiency Review

Review the same changes for efficiency:

1. **Unnecessary work**: redundant computations, repeated file reads, duplicate network/API calls, N+1 patterns
2. **Missed concurrency**: independent operations run sequentially when they could run in parallel
3. **Hot-path bloat**: new blocking work added to startup or per-request hot paths
4. **Memory**: unbounded data structures, missing cleanup, event listener leaks
5. **Overly broad operations**: reading entire files when only a portion is needed

## Phase 3: Fix Issues

Wait for all four agents to complete. Aggregate their findings into a structured report:

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
```

Then **fix each actionable issue directly**. If a finding is a false positive or not worth addressing, note it and move on.

After fixing, run the project's test suite to verify no regressions. Commit the fixes if tests pass.

When done, briefly summarize what was fixed (or confirm the code was already clean).

## Severity Levels
- **CRITICAL** -- security vulnerability or data loss risk, must fix
- **MAJOR** -- bug, significant performance issue, or spec violation
- **MINOR** -- code quality improvement, nice-to-have
- **INFO** -- observation, no action needed
