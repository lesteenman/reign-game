# CLAUDE.md Cleanup — Design

**Date:** 2026-06-06 · **Branch:** `docs/claude-md-cleanup`

## Goal

CLAUDE.md has grown to ~492 lines and 30+ sections. Much of it is refinement/execution
protocol that applies only during specific phases, plus an agent roster we no longer use.
Strip CLAUDE.md down to what almost always applies. Move phase-specific detail into skills.
Move first-checkout setup into a doc. Delete the agents.

## Decisions (resolved with supervisor)

1. **Refinement + Autonomous Execution Contract → skills.** CLAUDE.md keeps only a pointer.
   - `refinement-session` skill (new) — pick up a batch, sequence, loop refinement, confirm Ready.
   - `refinement` skill (update) — per-issue DoR work; absorbs the full DoR checklist so it stands alone.
   - `autonomous-execution` skill (new) — execution contract, merge authority + hold-set, re-entry digest, HITL reconciliation.
2. **Setup → `SETUP.md`.** A doc fits human first-checkout better than a skill. CLAUDE.md keeps one line.
3. **Lessons → distributed.** Domain/workflow lessons fold into their logical section; a short
   "Cross-cutting lessons" remainder holds the few with no home. Historical lessons (#6, #7) dropped.
4. **No agents.** Delete all of `.claude/agents/`. Reviews run as skills
   (`superpowers:requesting-code-review` + `architecture`; the `security-review` skill gated by the
   Deep Review Trigger list). The three personas (product-owner, ui-ux-designer, tester) fold into
   existing skills + HITL: PO → supervisor via refinement; design → `frontend-design` / `ui-ux-pro-max`;
   testing → `test-driven-development` + `playwright-cli`.
5. **Available Skills section deleted.** Skill summaries already drive discovery.
6. **Full rewrite, not surgical.** Apply `write-simply` to the whole doc. Run a critical content
   review after.

## Lesson distribution

| Lesson | Destination |
|--------|-------------|
| #1 git-from-root, #3 review-before-PR, #4 rename-grep, #5 trust-hooks, #9 perf-fix-attach, #14 verify-branch | Change Workflow |
| #12 unit-tests-don't-prove-contract | Testing (already referenced there) |
| #2 fetch-first, #8 verify-dep-versions, #10 lockstep-config, #11 CD-monitor, #13 trace-from-callsite | Cross-cutting lessons (remainder) |
| #6, #7 | dropped (historical / frozen) |

## Artifacts

- New: `SETUP.md`, `.claude/skills/refinement-session/SKILL.md`, `.claude/skills/autonomous-execution/SKILL.md`
- Updated: `CLAUDE.md` (full rewrite), `.claude/skills/refinement/SKILL.md`
- Deleted: `.claude/agents/*` (6 files)

## Verification

- Re-read the rewritten CLAUDE.md for accuracy.
- Grep live (non-frozen) files for dangling references to deleted agents and removed sections.
- Critical content review by a subagent: does every retained line still apply, is anything lost,
  are the skill pointers correct.
