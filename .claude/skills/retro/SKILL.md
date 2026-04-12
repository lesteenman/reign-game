---
description: "Retrospective for the agent team pipeline. Use when the user says '/retro', 'run the retrospective', or after completing a feature. Reviews what went well, what didn't, and proposes improvements to agents and workflow. Works with any project — no metrics infrastructure required. User decides what to implement."
user_invocable: true
---

# Retrospective

Run a retrospective for the current or most recent completed feature.

## Steps

### 1. Gather Data

Collect from all available sources. Not every source will exist — use what's available and note gaps.

**Git history (always available):**
- `git log --oneline main..HEAD` (or feature branch) for commit list
- Count: total commits, fix/rework commits (messages containing "fix", "rework", "address"), test commits
- `git diff --stat main..HEAD` for files changed and lines added/removed
- Derived: rework ratio = fix commits / total commits

**MR/PR comments (if MR exists):**
- Read review comments via `glab mr view` / `gh pr view`
- Categorize findings by type: validation, authorization, contract-mismatch, error-handling, performance, test-gap, other
- Count: total findings, findings by severity (critical/high/medium/low)

**Agent memory files (if they exist):**
- Read `.claude/agent-memory/*/MEMORY.md` for accumulated patterns and past findings
- Look for recurring issues that lessons should have prevented

**Past retrospectives:**
- Check git log for commits with "retro" or "retrospective" in the message
- Read memory files for feedback entries about workflow improvements

**User input:**
- Ask the user: "What went well? What was painful? Anything surprising?"
- This fills gaps that automated sources miss (e.g., "the frontend agent kept misunderstanding the layout")

### 2. Produce the Report

```
## Retrospective: <feature-name>

### What Happened
- Commits: N total, M rework (fix) commits
- Rework ratio: M/N (X%)
- Files changed: N, Lines: +X / -Y
- Review findings: X total (Y critical/high, Z medium/low)
- Tests added: N (estimated from commit messages and file names)

### Finding Patterns
Which types of bugs keep appearing? Compare against previous features.
If the same bug type recurs despite agent lessons targeting it, the lesson isn't working.

### Improvement Tracking
For each improvement applied in past retros:
- What was changed
- Did it help? (yes / no / too early to tell)
- Evidence from this feature's data

### Proposed Improvements
For each proposal:
| Field | Content |
|-------|---------|
| **What** | The specific change (file, section, wording) |
| **Why** | What problem it targets |
| **Expected gain** | What improvement we'd expect |
| **Evidence** | What data from this retro supports this |
| **Confidence** | High / Medium / Low |

Number each proposal. Do NOT auto-apply any of them.
```

### 3. User Decides

Present the proposals and ask: "Which improvements do you want to apply? (e.g., 1, 3, 5 or 'all' or 'none')"

### 4. Apply Selected Improvements

For each selected proposal:
1. Make the change (update CLAUDE.md, agent files, skills, or memory as needed)
2. If adding a new "Lesson from Past Reviews" to an agent, append it with the next number in sequence

### 5. Commit

If improvements were applied, commit with message:
```
chore: apply retro improvements from <feature-name>
```

## Guardrails

- Never apply improvements without user approval
- Each proposal must have concrete evidence — no speculative "this might help"
- If a past improvement shows no impact after 3+ features, flag it for removal
- Proposals should target the highest-impact problems first
- If data sources are sparse (first feature, no MR yet), lean on user input and keep the retro lightweight
