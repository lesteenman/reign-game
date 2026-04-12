---
name: gitlab-code-review
description: >
  Code review for GitLab merge requests using glab CLI. Reviews MRs with parallel specialized agents,
  confidence-based filtering, and posts findings as MR comments. Use this skill when the user wants to
  review a GitLab merge request, mentions "review MR", "code review" in a GitLab context, provides a
  GitLab MR URL or ID, says "review this merge request", or asks to check an MR for bugs. Also use when
  the user is working in a repo hosted on GitLab and asks for a code review, or when another agent needs
  to request or receive a code review in a GitLab workflow.
---

# GitLab Code Review

Review GitLab merge requests with rigor. Find real bugs, skip the noise, post findings directly on the MR.

This skill has two modes:
1. **Command mode** — `/gitlab-code-review <MR-ID>` reviews an MR and posts findings as a comment
2. **Workflow mode** — for agents requesting or receiving reviews during development

For the automated workflow guide, read `references/automation-workflow.md`.

---

## Command Mode: Reviewing an MR

When invoked with an MR ID, branch name, or URL, follow these steps precisely.

### 1. Eligibility Check

Use a Haiku agent to check if the MR:
- Is closed or merged
- Is a draft
- Is trivial/automated (e.g., dependency bumps, generated files)
- Already has a code review comment from you

If any of these are true, stop and explain why.

### 2. Gather Context

Use a Haiku agent to find relevant CLAUDE.md files: the root CLAUDE.md (if it exists) and any CLAUDE.md
files in directories whose files the MR modifies. Return only the file paths.

### 3. Summarize Changes

Use a Haiku agent to view the MR and return a summary of the change:
```bash
glab mr view <MR-ID> --output json
glab mr diff <MR-ID> --color=never
```

### 4. Parallel Review (5 Sonnet Agents)

Launch 5 agents in parallel. Each returns a list of issues with the reason each was flagged:

1. **CLAUDE.md compliance** — Audit changes against CLAUDE.md guidance. Not all CLAUDE.md instructions
   apply during review (some are writing-time guidance). Focus on rules that are verifiable in review.

2. **Bug scan** — Read the MR diff and do a shallow scan for obvious bugs. Focus on the changes
   themselves, not surrounding context. Look for large bugs, skip nitpicks. Ignore likely false positives.

3. **History context** — Read `git blame` and `git log` of modified files to find bugs that only become
   apparent with historical context (e.g., reverting a fix, breaking an invariant established earlier).

4. **Previous MR comments** — Find previous MRs that touched these files, check for comments that may
   also apply here:
   ```bash
   glab api "projects/:id/merge_requests?state=merged&per_page=10" | # filter by relevant files
   ```

5. **Code comment compliance** — Read code comments (TODOs, invariants, warnings) in modified files and
   verify the MR changes respect them.

### 5. Confidence Scoring

For each issue found in step 4, launch a parallel Haiku agent that scores confidence 0-100. Give the
agent the issue, the MR context, and the CLAUDE.md file list. Scoring rubric (pass verbatim to agent):

- **0**: False positive that doesn't hold up to light scrutiny, or a pre-existing issue.
- **25**: Might be real, but could be false positive. Agent couldn't verify. Stylistic issues not
  explicitly called out in CLAUDE.md.
- **50**: Verified real issue, but a nitpick or unlikely in practice. Not very important relative to
  the rest of the MR.
- **75**: Double-checked and very likely real. Will be hit in practice. Existing approach is
  insufficient. Directly impacts functionality or is explicitly mentioned in CLAUDE.md.
- **100**: Confirmed real issue. Will happen frequently. Evidence directly confirms this.

For CLAUDE.md-flagged issues, the agent must verify the CLAUDE.md actually calls it out specifically.

### 6. Filter

Remove issues scoring below 80. If nothing remains, post the "no issues" comment and stop.

### 7. Re-check Eligibility

Use a Haiku agent to confirm the MR is still open and eligible (it may have been closed/merged while
you were reviewing).

### 8. Post Comment

Post the review as an MR note using `glab mr note`. Keep it brief, link to code, avoid emojis.

**To get the project URL and full SHA for links:**
```bash
# Get remote URL for link construction
REMOTE_URL=$(glab repo view --output json | jq -r '.web_url')
# Get the latest commit SHA on the MR source branch
HEAD_SHA=$(glab mr view <MR-ID> --output json | jq -r '.sha')
```

**Comment format (if issues found):**

```
### Code review

Found N issues:

1. <brief description> (CLAUDE.md says "<...>")

<REMOTE_URL>/-/blob/<HEAD_SHA>/path/to/file#L<start>-<end>

2. <brief description> (bug due to <reason>)

<REMOTE_URL>/-/blob/<HEAD_SHA>/path/to/file#L<start>-<end>

Generated with Claude Code
```

**Comment format (no issues):**

```
### Code review

No issues found. Checked for bugs and CLAUDE.md compliance.

Generated with Claude Code
```

**Link format** — GitLab uses `/-/blob/` and `#L<start>-<end>` (no `L` prefix on end line):
- `https://gitlab.com/group/project/-/blob/abc123def/src/main/App.java#L10-15`
- Always use the full SHA, never a variable or command substitution in the comment text
- Include 1 line of context before and after the flagged lines

### False Positives to Filter (Steps 4 and 5)

- Pre-existing issues (not introduced by this MR)
- Something that looks like a bug but isn't
- Pedantic nitpicks a senior engineer wouldn't flag
- Issues a linter, typechecker, or CI pipeline would catch (imports, formatting, type errors)
- General quality issues (test coverage, docs) unless explicitly required in CLAUDE.md
- Issues silenced by lint-ignore comments
- Intentional functionality changes directly related to the MR's purpose
- Real issues on lines the author did not modify

---

## Workflow Mode: Requesting a Code Review

When an agent has completed work and needs a review before proceeding:

### When to Request

- After completing a task or feature (mandatory)
- Before merge to main (mandatory)
- When stuck, before refactoring, after fixing complex bugs (optional)

### How to Request

1. Get the commit range:
   ```bash
   BASE_SHA=$(git rev-parse HEAD~N)  # where N = number of commits in this change
   HEAD_SHA=$(git rev-parse HEAD)
   ```

2. Dispatch a code-reviewer subagent with:
   - What was implemented
   - What the plan/requirements were
   - BASE_SHA and HEAD_SHA
   - Brief description

3. The reviewer evaluates against:

   **Code Quality** — Clean separation of concerns? Proper error handling? Type safety? DRY? Edge cases?

   **Architecture** — Sound design? Scalable? Performance? Security?

   **Testing** — Tests actually test logic (not just mocks)? Edge cases covered? Integration tests?

   **Requirements** — All requirements met? Implementation matches spec? No scope creep?

   **Production Readiness** — Migration strategy? Backward compatibility? Documentation?

4. Issues are categorized by severity:
   - **Critical**: Bugs, security issues, data loss risks, broken functionality
   - **Important**: Architecture problems, missing features, poor error handling, test gaps
   - **Minor**: Code style, optimizations, documentation improvements

5. Verdict: clear yes/no/with-fixes and 1-2 sentence technical reasoning.

---

## Workflow Mode: Receiving a Code Review

When you receive review feedback (from a human or another agent):

### Process

1. **Read** — Complete feedback without reacting
2. **Understand** — Restate the requirement in your own words, or ask for clarification
3. **Verify** — Check against the actual codebase. Don't trust the review blindly.
4. **Evaluate** — Is this technically sound for THIS codebase?
5. **Respond** — Technical acknowledgment or reasoned pushback
6. **Implement** — One item at a time, test each

### Rules

- Never use performative phrases ("You're absolutely right!", "Great catch!", "Let me implement that!")
- Never implement blindly — verify against the codebase first
- Push back with technical reasoning if the reviewer is wrong
- For multi-item feedback: fix blocking issues first, then simple, then complex

### Responding to Correct Feedback

- "Fixed. [Brief description]"
- "Good catch — [specific issue]. Fixed in [location]."
- Or just fix it silently and show in the diff.

### External Reviewer Checklist

Before implementing feedback from an external reviewer, verify:
- Technically correct for THIS codebase?
- Will it break existing functionality?
- Is there a reason for the current implementation?
- Does the reviewer understand the full context?

---

## Notes

- Use `glab` for all GitLab interactions, not web fetch or curl
- Do not run builds or type-checks — CI handles that separately
- For the automation workflow (CI/CD integration, webhook-driven reviews), see `references/automation-workflow.md`
