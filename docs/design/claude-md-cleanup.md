# CLAUDE.md Cleanup — Design Summary

## Final Design

CLAUDE.md becomes a 325-line operating manual whose first 110 lines cover principles, workflow, agent teams, and the human-in-the-loop protocol. Four new core rules — Think Before Coding, Simplicity First, Surgical Changes, Goal-Driven Execution — sit at the top. Domain conventions and lessons that belong to one agent move to that agent's file. Cross-cutting lessons stay, compressed to one line each. Manual lead-agent takeover is housekeeping only; agents notify the human via `PushNotification` when blocked.

## Decisions

### Coding Principles (new top section)

- **Add the 4 rules verbatim.** Insert as a new section after Tech Stack. They become the single source of truth for project-wide engineering principles. Duplicate clean-code prose elsewhere (system-prompt-style guidance, repeated SOLID/clean-code blocks in agent files) gets deleted.
- **Keep "bias toward caution over speed" as written.** Caution = surface confusion to the human, not agent-self-preservation. The escape hatch ("for trivial tasks, use judgment") stays.
- **No softening for "speed" arguments.** Past justifications of "manual takeover ships faster" are wrong and get removed wherever they appear.

### Manual Takeover Policy

- **Lead agent commits work that an agent finished but timed out before committing.** That is the only takeover mode. No manual implementation, no writing tests on the agent's behalf, no engineering shortcuts.
- **Stall protocol is mechanical.** Split the dispatch into smaller batches. If a chunk stalls twice, split smaller again or notify the human. Never re-dispatch the same chunk a third time.

### Notifications

- **Agents call `PushNotification` when blocked or uncertain.** The human supervisor is not actively watching the screen. Silent stalling is worse than asking.
- **HITL applies to design forks.** Strong form. The agent presents options and waits.
- **Think Before Coding applies to implementation ambiguity.** Everyday form. The agent states assumptions explicitly and asks if material.

### Conflict Resolutions

- **Surgical Changes vs. "remove unused imports."** Tighten the Pre-Commit Quality Checklist in backend-dev.md and frontend-dev.md to: "Remove imports your changes orphaned. Don't remove pre-existing dead code."
- **Surgical Changes vs. full-repo rename grep.** Not a conflict — rename creates orphans everywhere, all downstream of your change. Add one line: "Touching renamed identifiers everywhere is surgical. Cleaning up unrelated dead code is not."
- **Simplicity First vs. defense-in-depth.** Default is no defensive code. When defensive code exists, layers must agree on the policy (existing lesson #33).

### Lessons Cleanup (36 → ~10 in CLAUDE.md)

- **Drop 5:** #6, #7, #21, #26 are obsolete or duplicated; #27 (verification checklist) survives as one line inside Change Workflow.
- **Move 21 to agent files:**
  - backend-dev: #8, #10, #16, #23, #24, #32, #33, #35
  - frontend-dev: #3, #5, #9, #16, #28, #30
  - devops-engineer: #22, #36
  - workflow-orchestrator: #1, #4, #11 (rewritten), #20 (rewritten), #31 (elevated as section header)
- **Compress 10 in CLAUDE.md to one line each:** #2, #12, #13, #14 (+ surgical note), #15, #17, #18, #19, #25, #29.
- **Move #34 (`core.hooksPath`)** to CLAUDE.md Setup section AND to README.md for human contributors.
- **Lesson #16 is split.** Defining persisted shapes once is a backend-and-frontend rule; duplicate in both agent files.

### Domain Pointer Block

- **CLAUDE.md keeps a 5-line "Domain Conventions" pointer block.** One line per agent file. Costs nothing; helps when scanning CLAUDE.md to verify a convention exists.

### New Ordering (20 sections, ~325 lines)

1. Project Overview
2. Tech Stack
3. **Coding Principles** (new)
4. **Change Workflow** (elevated)
5. **Agent Teams + Available Agents** (elevated)
6. **Human-in-the-Loop + Notifications** (elevated)
7. Build Commands
8. Running the Dev Stack
9. Testing
10. Git Hooks
11. Dev Server Ports
12. Setup (incl. `core.hooksPath`)
13. Project Structure pointer
14. Roles
15. Key References
16. Domain Conventions (pointer block)
17. Database (compressed)
18. Lessons (compressed)
19. Security: Baseline Gates + Deep Review Trigger
20. How to Use the Agents (Assisted / Full Pipeline)

### Memory Updates

- **Update `feedback_small_batch_agents.md`:** drop the "ships faster" implication. Add: manual takeover is housekeeping only, never engineering.
- **Add a new feedback memory** on `PushNotification` use when uncertain — the human is not watching.

## Deferred Items

- **`task setup-hooks`** — automating `git config core.hooksPath .githooks` plus pinned-version installs of `golangci-lint` and `gitleaks`. Mentioned during the grill on lesson #34 but not part of this cleanup.
- **Tester agent file lessons.** #36 (e2e seed every operational state) is moved to devops-engineer; whether to also duplicate in tester.md is left to the implementing agent.

## Constraints & Assumptions

- **CLAUDE.md is the source of truth for cross-cutting guidance.** Agent files own domain-specific conventions and lessons.
- **Sub-agents read their own agent file plus CLAUDE.md.** They do not read other agents' files. Content placement must respect this — anything a sub-agent needs must be in its agent file or in CLAUDE.md.
- **The 4 new rules are the only project-wide principles.** Anything that contradicts or duplicates them gets deleted, not preserved alongside.
- **`PushNotification` is available to agents.** Project relies on it as the channel when human input is needed and not present in-conversation.
