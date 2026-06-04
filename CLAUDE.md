# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Reign** (working title) is a puzzle game available as a Progressive Web App. Players place markers on a colored grid subject to row, column, region, and adjacency constraints. The default presentation is minimalist and abstract; alternative visual themes (including a classic "Queens" chess theme) are available. It offers curated puzzles across difficulty levels, daily challenges with speed-based leaderboards, and a freemium monetization model (no ads).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, AWS Lambda, API Gateway (REST) |
| Frontend | React 19, TypeScript, Vite, Tamagui (UI primitives, cross-platform), TanStack Query (server state), Workbox (PWA) |
| Database | DynamoDB (on-demand pricing) |
| Testing | Go test (backend), Vitest (frontend unit), Playwright (e2e) |
| Build | Go build / Taskfile (backend), npm + Vite (frontend) |
| Infrastructure | Terraform, AWS (S3, CloudFront, Lambda, API Gateway, DynamoDB) |
| CI/CD | GitHub Actions — CI on PR, CD on merge to main |
| Dev Environment | LocalStack (local DynamoDB), Vite dev server (frontend) |

**Frontend transition state.** Tamagui + TanStack Query installed in Track 2; #176 migrates existing code into the Bulletproof React feature-folder layout + the `LoadState` → TanStack hooks transition. Tailwind retired in #176 (removed from package.json + index.css). New code uses Tamagui from the start; chrome styling is inline `style={}` + theme tokens until the Tamagui kickoff slice lands. See `frontend/CLAUDE.md` and `.claude/skills/architecture/SKILL.md`.

## Coding Principles

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: every changed line should trace directly to the user's request. A rename or path migration that ripples across the repo IS surgical — every site references the renamed identifier. Cleaning up unrelated dead code you happen to see is not.

Important exception: no "test failed on pre-existing issue, ignoring". If we encounter broken code, always assume it's part of the
required changes. If it feels needed, involve the user again to check before implementing or researching.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Change Workflow (MANDATORY)

Every change — feature, fix, or refactor — follows this pipeline. The full pipeline is driven by Superpowers skills (installed via `/plugin install superpowers@claude-plugins-official`).

```
1. Issue triage           → pick or open a GitHub issue; capture acceptance criteria in issue comments
2. Refine to Ready        → `refinement` skill (composes Superpowers `brainstorming` + `architecture` design-time check + `glossary`). Output is a **Definition-of-Ready comment on the GitHub issue** — **no branch, no commits, no spec file** at this stage. See the Refinement + Autonomous Execution Contract below.
3. Branch                 → **AT implementation start (not before refinement)**: Superpowers `using-git-worktrees` (preferred) or feature branch. The Ready issue is the handoff unit; the branch carries the plan + code.
4. Plan                   → Superpowers `writing-plans` skill (decompose into 2–5 min tasks) — the **first implementation step on the branch**, optionally alongside a committed design doc. For multi-approach exploration: `parallel-plan` skill first to compare 5 approaches, then `writing-plans` to decompose the chosen one.
5. TDD execution          → Superpowers `subagent-driven-development` or `executing-plans`, gated by `test-driven-development`. Subagents auto-load the relevant subdirectory CLAUDE.md (`backend/CLAUDE.md`, `frontend/CLAUDE.md`, `infra/CLAUDE.md`) based on the file paths they touch.
6. Integration verification → **For any change that crosses a service boundary (frontend↔backend, backend↔DB, backend↔SQS, frontend↔SW, frontend↔CloudFront edge), exercise the real wire before opening a PR.** Choose the lightest form that fits: (a) `playwright-cli` against `task dev:up` for one-off verifications where adding a permanent test would be ceremony — e.g. "does this single new endpoint accept the method I expect" or "does my CSS change render right"; (b) a durable Playwright e2e spec under `frontend/playwright/e2e/` (run via `task e2e:up && task test:e2e`) when the contract is worth catching regressions on long-term — e.g. the connectivity probe in `offline-banner.spec.ts`, where a frontend↔backend method mismatch shipped to merge in #179 because unit tests on both sides agreed on a contract that never existed. Default to (a); promote to (b) when the same boundary will keep getting touched. Unit tests on both sides do NOT prove the contract — see lesson 12. Skip only when the diff is purely within one layer (a pure-engine refactor, a frontend visual tweak with no new API calls, etc.).
7. Inter-task review      → Superpowers `requesting-code-review` + the `architecture` skill's review-time drift greps. Findings get a SWEEP grep command — fix agents fix ALL matches, not just the reported file.
8. Security gate          → `security-review-final` agent (always; deep-review trigger list in the Security section below)
9. Finish branch          → Superpowers `finishing-a-development-branch`. **PRECONDITION (HARD GATE): steps 6 + 7 + 8 MUST have run on the current branch's diff before this step is invoked.** No PR opens without integration verification on cross-boundary changes + `requesting-code-review` on the diff + `security-review-final` (when triggers met). Finding any of these steps skipped post-hoc — as with #179's HEAD/405 bug — is itself a workflow bug to flag in the next retro. PR description includes a "Key Decisions" section listing intentional design choices.
10. CD/Dependabot         → monitored by the `Reign CD + Dependabot monitor` Claude routine (twice daily, 09:00 + 21:00 Europe/Amsterdam = `0 7,19 * * *` UTC). Failures auto-open a `priority:p0`+`area:devops`+`type:bug`+`status:blocks-prod` GitHub issue. No inline post-merge watch.
11. Retro                 → `retro` skill on shipped features
```

- **TDD is non-negotiable.** Superpowers `test-driven-development` enforces RED-GREEN-REFACTOR.
- **Feature branches or worktrees** for all work. Never commit directly to main.
- **Commits** happen after every artifact delivery (plan, individual tasks, completed feature), not just at the end.
- **Verification before completion.** Superpowers `verification-before-completion` requires running build + tests + verifying output before claiming done. Evidence before assertions.
- **PR description shape — keep it lean.** The body exists to help a reviewer (and future-you grepping git history) decide what's intentional. Default structure:
  - **Summary** — 2-4 bullets on what changed and net LOC.
  - **Key Decisions** — **max 3-5 items.** Only include a decision if (a) a reviewer would actively question its absence, (b) it documents a non-obvious tradeoff future-you would want to grep for, or (c) it captures a HITL fork the user explicitly chose between. **Skip:** workarounds already in CLAUDE.md (reference the doc instead), patterns established in prior PRs ("same as #N's X"), implementation details the diff makes obvious.
  - **Test plan** — one line. "415/415 vitest, 17/17 Playwright, smoke verified X+Y+Z." Outcomes, not steps.
  - **Drop:** "After this slice" / "Remaining work" sections. File follow-up issues instead.
  Target: ~40-60 line PR bodies, not 100+. Key Decisions exists to prevent reviewer churn, not to comprehensively narrate every micro-choice.

## Agent Teams

Custom agents live in `.claude/agents/`. After Track 2, the implementation-agent slots (backend-dev/frontend-dev/devops-engineer) are gone — Superpowers' `subagent-driven-development` dispatches subagents directly per task, and the subdirectory CLAUDE.md files (`backend/CLAUDE.md`, `frontend/CLAUDE.md`, `infra/CLAUDE.md`) provide the area-specific context that auto-loads when subagents touch files in those directories. The remaining agents handle non-implementation roles where having a named, invokable persona is the right shape.

| Agent | Role | When |
|-------|------|------|
| `product-owner` | Vision guardian, acceptance criteria, scope decisions. Does NOT write code. | Before implementation |
| `ui-ux-designer` | Wireframes, interaction design, brand guidelines, Nano Banana 2 prompts | Visual design phases |
| `tester` | Test plans, edge case discovery, coverage audits, Playwright execution, bug-found protocol (unit test first) | After implementation (focused or broad audit) |
| `code-review-final` | Code quality review of PRs via `gh pr` + Superpowers `requesting-code-review` + `architecture` skill | After all implementation, before security |
| `security-review-final` | Security review (conditional — see Security section) | When diff touches security-sensitive files |

Implementation work happens via Superpowers subagent dispatch (no named implementation agent). Each subagent reads the relevant subdirectory `CLAUDE.md` for conventions.

## Human-in-the-Loop + Notifications

This project is built by agents with a human supervisor who is NOT actively watching the screen. Pause-and-ask is always cheaper than guessing wrong.

**HITL Rule (CRITICAL — applies to design forks):**
- NEVER answer your own design questions or auto-approve decisions.
- NEVER assume you know what the human would choose.
- ALWAYS present options and wait for an explicit response.
- Confirm alignment before moving to the next phase.
- **Under the Refinement + Autonomous Execution Contract (below), this rule is UNCHANGED.** That contract front-loads design forks into batched refinement so fewer surface mid-flight; residual unclarity during execution is **notify-and-hold**, never an assumption.

**Think Before Coding (everyday form — applies to implementation ambiguity):**
- State assumptions explicitly. If material, ask.
- If multiple interpretations exist, present them — don't pick silently.

**Notification channel:** Use `PushNotification` when blocked or uncertain. The supervisor isn't watching, so silent stalling is worse than asking.

**Manual lead-agent takeover policy:**
- **Allowed:** committing work that an agent finished but timed out before committing. Housekeeping only.
- **Forbidden:** writing application code, writing tests, fixing bugs, completing partial implementations. Never as engineering shortcut.

**Agent stall protocol:**
- First stall on a chunk → re-dispatch the same chunk once.
- Second stall on the same chunk → split into smaller batches (~3-5 tool uses each).
- Recurring stall after splitting → escalate via `PushNotification`. Never re-dispatch a third time.
- If a stalled agent had completed its work in the working tree, the lead agent commits it. That is the only takeover mode.

## Refinement + Autonomous Execution Contract

Adopted 2026-06-03. Governs how multi-issue work runs when the supervisor (team lead + Product Owner) is not actively checking in. **Goal:** the supervisor batches their involvement into **refinement**; execution then runs autonomously against written specs. This does **not** relax the HITL rule — it front-loads design forks so fewer arise mid-flight, and defines exactly what to do with the ones that still do.

Shape: **batched refinement (always together) → Definition of Ready gate → autonomous execution with notify-and-hold on residual unclarity → autonomous merge for low-risk / hold-open for the risk-class set → end-of-session digest.**

### 1. Definition of Ready (the gate)

An issue enters "Up Next" / autonomous execution only once **all** of these are settled, together, in refinement. If any is blank at execution time, that is unclarity → notify + hold (§3), never an assumption.

- [ ] **Problem + PO rationale** — what, and why (the user value).
- [ ] **Acceptance criteria** — explicit, enumerated, testable. Not "make it work."
- [ ] **Design forks resolved** — every option-decision made and recorded. No open "should we X or Y" remains.
- [ ] **Test-plan sketch** — what proves it: unit / integration / e2e. Cross-boundary contracts flagged for real-wire verification (Change Workflow step 6 / lesson 12).
- [ ] **Edge cases enumerated.**
- [ ] **Out-of-scope** stated explicitly.
- [ ] **Dependencies + sequencing** — which issues it depends on; independent vs stacked (stacking is the exception, §2).
- [ ] **Risk class tagged** — does it hit the merge-hold set (§4)? Determines merge vs hold-open.
- [ ] **Boundaries crossed** — which service boundaries it touches (drives integration verification).

Refinement output is persisted as a **Definition-of-Ready comment on the GitHub issue** — the canonical Ready record. **Refinement creates no branch and no commits** (see the `refinement` skill). The `writing-plans` plan doc and any committed design doc come later, on the implementation branch. **GitHub is the source of truth for progress** — a context reset is recoverable from issue + PR state.

### 2. Refinement protocol

- Driven by the **`refinement` skill** (`.claude/skills/refinement/`), which composes `superpowers:brainstorming` for the design conversation but **redirects its output to the issue DoR comment and defers branch + `writing-plans` to implementation**. The brainstorming skill is authored to defer to project preferences, so this is the supported override — do **not** shadow it by name in `.claude/skills/`.
- Always **together** — this is where HITL lives. The supervisor is team lead + Product Owner and makes the calls; I prep (pre-analyze candidates, surface ambiguities + options + dependencies, draft the Definition-of-Ready fill) so the session is decision-making, not discovery.
- Refine a **small batch** (start 2–3 issues; expand only after calibration — see Rollout). Small batches bound both review blast-radius and spec rot.
- **Sequence by dependency; put independent issues first** so completed work banks before any surprise hang. Select batches to **minimize stacking** — prefer independent breadth over stacked depth.

### 3. Execution contract

I work the Ready issues **one after another**, a PR per issue.

- **Notify-and-hold on any residual unclarity.** If execution uncovers something not settled in refinement — an ambiguous criterion, a contract that doesn't behave as specced, a missed design implication — `PushNotification` immediately and **hold until answered. Do not assume, do not skip ahead.** Strict sequential hold for v1: a single surprise stalls the run until the supervisor answers; the only lever that buys duration back is Definition-of-Ready quality.
- **Park-and-continue applies to exactly one case:** a *completed* PR that is non-blocking for downstream work but requires supervisor approval to merge (the risk-class set, §4). Leave it open and proceed to the next independent issue. This is the only time work moves on with something outstanding.
- **Issue is wrong, not just unclear** — mis-scope, duplicate, should-be-split, obsoleted: route to the supervisor as Product Owner (notify + hold). Never silently redefine or "tidy" the backlog.
- Each PR still runs the full Change Workflow gates (integration verification, `requesting-code-review`, `security-review-final` when triggered). Autonomy changes *who decides design*, not *whether the gates run*.

### 4. Merge authority

- **Merge autonomously** a PR that is fully green — CI + `requesting-code-review` + security gate — **and outside the hold set.**
- **Hold-for-supervisor-merge set = the Security Deep-Review Trigger list** (auth, middleware, Lambda handlers, dependency manifests, `infra/**/*.tf`, `.github/workflows/**`, anything with `secret`/`token`/`key`/`credential`) **∪ any change that could materially impact AWS cost.** These stay open for explicit approval.
- Also held: any PR where my own code-review/security pass surfaced a finding I could **not** fully resolve.
- **Merge = deploy** (CD ships to acc on merge to main). Autonomous merge is safe in proportion to the post-deploy verification gate: **#241 (GitHub Environments + Deployments verification) is a stated prerequisite for widening autonomous merge** — until it lands, keep the hold set conservative.

### 5. Re-entry digest

When the supervisor returns, the deliverable is a **single digest** (not "go read N PRs cold"), via `PushNotification` + session summary:

- **Merged** — what shipped (and therefore deployed to acc).
- **Open PRs awaiting you** — each with its hold reason (risk class / unresolved finding) + a one-line review pointer.
- **Parked / held questions** — any notify-and-hold still outstanding, with the question + options + my recommendation so it's answerable in one line.
- **Retro flags** — possibly-critical points: forks I hit that Definition of Ready should have caught (→ feeds back into §1), stalls, anything that smelled like drift.

### 6. HITL reconciliation

The HITL Rule is **unchanged**: the supervisor clarifies, I do not assume. This contract changes only *when* the clarifying happens — design forks are resolved up front in batched refinement, so fewer surface mid-execution. The autonomy granted is over **execution decisions inside a Ready issue**, never over design forks or scope. When genuinely unsure whether something is a small clarification or a real fork: **park and ask** — holding has been made cheap on purpose.

### Rollout & calibration

Treat the first 2–3 batches as **calibration, not production**: start small, measure rework rate (PRs needing rework) and stall cost (how often/long the run holds on a surprise), and expand batch size only as the data supports it. The `retro` skill tunes the Definition of Ready from whatever forks slipped through. Revisit an "advance to a demonstrably-independent Ready issue while one is parked" exception (§3) **only** if calibration shows sequential-hold stalls are expensive — earn it with data.

## Build Commands

```bash
# Build / test
task build              # Build backend + frontend
task test               # Run all tests
task build:backend      # Build Go backend
task test:backend       # Run backend tests
task lint:backend       # Run golangci-lint
task build:frontend     # Build frontend
task test:frontend      # Run frontend unit tests
task deploy             # Build + terraform apply

# E2E lifecycle
task e2e:up              # Bring up the full e2e stack (LocalStack + e2e backend + e2e frontend + seeded fixtures)
task e2e:down            # Tear down the e2e stack (LocalStack stays up — shared with dev)
task e2e:seed            # Re-seed committed fixture puzzles into puzzle-pool-e2e (idempotent)
task e2e:status          # Show e2e backend status + fixture count
task e2e:genfixtures     # Regenerate committed fixture puzzles (deterministic from fixed seeds)
task e2e:up:generator    # Start the e2e generator worker (reads puzzle-generation-e2e queue, writes to puzzle-pool-e2e)
task e2e:down:generator  # Stop the e2e generator worker
```

## Running the Dev Stack (STANDARD — always use these)

The dev stack (LocalStack + backend + generator + frontend) runs as background processes with logs streamed to `./logs/*.log`. **Always use these tasks** — do not launch `go run ./cmd/api` or `npm run dev` directly. Doing so breaks the shared logging/lifecycle contract and leaves orphan processes.

```bash
task dev:up             # Start LocalStack + backend + generator + frontend (waits for readiness)
task dev:down           # Stop everything
task dev:restart        # Stop + start
task dev:status         # Show what's running (PIDs + ports)

task dev:logs            # Stream backend + generator + frontend logs to stdout (Ctrl+C to exit)
task dev:logs:backend    # Stream only backend logs
task dev:logs:generator  # Stream only generator logs
task dev:logs:frontend   # Stream only frontend logs

# Individual services
task dev:up:backend         # Start backend only (also brings up LocalStack via deps)
task dev:up:generator       # Start SQS generator worker only
task dev:up:frontend        # Start frontend only
task dev:up:localstack      # Start LocalStack only
task dev:down:backend       # Stop backend
task dev:down:generator     # Stop generator
task dev:down:frontend      # Stop frontend
task dev:down:localstack    # Stop LocalStack
task dev:restart:backend    # Restart backend (picks up Go code changes)
task dev:restart:generator  # Restart generator (picks up Go code changes)
task dev:restart:frontend   # Restart frontend
```

**How it works:**
- Services run detached via `nohup ... &`; stdout+stderr redirect to `./logs/{backend,generator,frontend}.log`.
- `dev:up` polls each service until healthy before returning. Backend check hits `/api/health`; frontend check waits for port `:5180` to listen; generator waits for the "starting local SQS poller" log line.
- Backend/frontend identity tracked by port (`lsof -ti:PORT`). Generator has no port, so its PID is persisted to `./logs/generator.pid` — robust against orphaned files because a stale PID is detected and cleaned up.
- LocalStack readiness check waits for BOTH the `/_localstack/health` endpoint AND the init-aws.sh script to finish (puzzle-generation queue exists + puzzle-pool table is ACTIVE).
- `./logs/` is gitignored.
- LocalStack runs in Docker via `docker-compose.yml`; init script `.localstack/init-aws.sh` creates the DynamoDB table, SQS queues, and seeds initial CONFIG items.

**After changing Go source:** `task dev:restart:backend` and/or `task dev:restart:generator` (neither is hot-reloaded).
**After changing frontend source:** Vite HMR handles most updates; if you edit `vite.config.ts` or similar, `task dev:restart:frontend`.

**Taskfile shell pitfalls (read before editing Taskfile.yml):**
Task runs `cmds:` blocks in its built-in interpreter (`mvdan.cc/sh`), not system sh or bash. It mostly matches POSIX but diverges in a few places that bite process-lifecycle code:

- `$!` after a background job returns a goroutine handle (e.g. `g1`), not an OS PID. Capturing `echo $! > file.pid` stores garbage.
- `kill -0 "$PID"` against external OS PIDs is unreliable.
- `disown` is not implemented.
- `set -e` behavior around command substitution differs from bash.

When a task needs to track a backgrounded process by PID (anything without a port), wrap the block in a bash heredoc:

```yaml
cmds:
  - |
    bash <<'BASH'
    set -e
    nohup long-running-cmd > log 2>&1 </dev/null &
    echo $! > pid
    BASH
```

Port-based lifecycle (`lsof -ti:PORT`) works fine in Task's shell and is preferred whenever a port exists.

## Testing

- Always run the full test suite after making changes.
- After fixing one bug, verify no regressions before moving on.
- All unit tests use **Arrange-Act-Assert** structure with explicit `// Arrange`, `// Act`, `// Assert` comments.
- **Cross-boundary work requires integration testing — pick the lightest form that fits.** When a change crosses frontend↔backend, backend↔DB, backend↔SQS, frontend↔SW, or frontend↔CloudFront edge: default to a quick `playwright-cli` verification against `task dev:up` (interactive, no ceremony). Promote to a durable Playwright e2e spec under `frontend/playwright/e2e/` when the contract is worth catching regressions on — same-boundary code will keep being touched, the failure mode is subtle, or unit tests on both sides could keep passing while the wire diverges (the #179 HEAD/405 pattern). Unit tests on both sides are NOT a contract: mocked `fetch` and direct-handler `httptest` calls both pass while the wire format diverges. See lesson 12.

## Git Hooks

Pre-push hook (`.githooks/pre-push`) runs before every push: backend lint + tests, frontend build + tests + npm audit, and `gitleaks` secret scan. Pre-commit (`.githooks/pre-commit`) runs gofmt/golangci-lint on staged Go and tsc on staged TS.

## Dev Server Ports

| Service      | Port | Started by                 |
|--------------|------|----------------------------|
| Frontend     | 5180 | `task dev:up:frontend`     |
| Backend      | 5181 | `task dev:up:backend`      |
| E2E Backend  | 5182 | `task e2e:up:backend`      |
| E2E Frontend | 5183 | `task e2e:up:frontend`     |
| Generator    | —    | `task dev:up:generator` (SQS consumer, no HTTP port; PID in `logs/generator.pid`) |
| E2E Generator | —   | `task e2e:up:generator` (no port; PID at `logs/e2e-generator.pid`) |
| LocalStack   | 4566 | `task dev:up:localstack`   |

Frontend already binds `--host 0.0.0.0` (for mobile testing over LAN); the Vite proxy forwards `/api/*` to `localhost:5181`. All backend routes live under `/api/`.

## Setup

After cloning the repo:

```bash
# 1. Git hooks (pre-commit + pre-push + post-checkout). Installs delegate shims in
#    .git/hooks/ that forward to .githooks/. .git/hooks/ is per-clone and
#    shared across worktrees, so this needs to run ONCE per fresh clone —
#    every worktree picks up the hooks automatically.
scripts/install-hooks.sh

# 2. Frontend dependencies (React, Vite, Tamagui, TanStack, Clerk, etc.)
#    Re-run inside any new `git worktree add`/`EnterWorktree` worktree
#    too — node_modules is gitignored and per-worktree. The pre-push
#    hook's `tsc` step fails with "command not found" otherwise.
cd frontend && npm ci && cd ..

# 3. Playwright CLI for agent-driven browser testing
npm install -g @playwright/cli@latest
playwright-cli install --skills   # writes/updates .claude/skills/playwright-cli/

# 4. Superpowers plugin (per-machine; the repo only commits the
#    `enabledPlugins` flag in .claude/settings.json — the actual
#    skill files live in ~/.claude/plugins/, per-machine cache)
#    Run from inside a Claude Code session:
#        /plugin install superpowers@claude-plugins-official
```

Without step 1, the pre-commit and pre-push gates silently don't run, and CI catches what your local shell should have. The previous setup (`git config core.hooksPath .githooks`) is replaced by the install script because `core.hooksPath` didn't reliably propagate across machines and worktrees, while delegate shims in `.git/hooks/` do (see `https://www.gitworktree.org/guides/hooks`).

Without step 4, the Superpowers skills (`brainstorming`, `writing-plans`, `subagent-driven-development`, etc.) are referenced by `.claude/settings.json` but the skill files won't be available locally — the workflow falls back to ad-hoc behavior.

## Project Structure

See **[PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)** for the full project tree and API endpoints.

## Roles

Role names are Title-Case in prose (`Anonymous` / `User` / `Admin`); the Clerk metadata claim values are lowercase (`'user'` / `'admin'`).

| Role | Identity | Access |
|------|----------|--------|
| Anonymous | No account; device-linked local identity | Practice puzzles, daily challenge, local stats, see own percentile |
| User | Signed-in via Clerk (Google OAuth); default role | Same as Anonymous for now; reserved for later phases |
| Admin | Signed-in via Clerk with `publicMetadata.role === 'admin'` | All User access + `/admin` UI and `/api/admin/*` routes |

## Key References

- **GLOSSARY.md** — Ubiquitous Language glossary. Consult before using domain terms.
- **PROJECT_STRUCTURE.md** — Full project tree + all API endpoints.
- **GAME_DESIGN.md** — Living game design vision document.
- **BRAND_GUIDELINES.md** — Design system. Required before any frontend visual work.
- **[GitHub Issues](https://github.com/lesteenman/reign-game/issues)** — current todos, backlog, known issues (replaces ROADMAP.md as of Track 1, 2026-05-15)
- **[`Reign` project board](https://github.com/users/lesteenman/projects/1)** — status overview by Kanban column, with `Priority` / `Area` / `Estimate (days)` custom fields
- **[Wiki](https://github.com/lesteenman/reign-game/wiki)** — Phases 0–8 roadmap history, design decisions log, workflow narrative

## Domain Conventions

Domain-specific conventions, logging rules, and per-domain lessons live in the subdirectory `CLAUDE.md` files. Claude Code auto-loads these when working on files within those directories.

- **Backend (Go) + logging + DynamoDB access patterns** — see `backend/CLAUDE.md`
- **Frontend (React + TypeScript + Tamagui + TanStack)** — see `frontend/CLAUDE.md`
- **Infrastructure (Terraform, GitHub Actions, AWS)** — see `infra/CLAUDE.md`
- **Architecture rules (per-area, design-time + review-time)** — see `.claude/skills/architecture/SKILL.md`

## Database (DynamoDB)

- On-demand (pay-per-request) billing — no provisioned capacity.
- Single-table design where practical, separate tables when access patterns diverge.
- All table definitions in Terraform (`infra/modules/database/`).
- No ORM — AWS SDK for Go v2 directly.
- Local development: LocalStack.

## Lessons (cross-cutting)

Slice IDs (`R-<phase>-<slice>`) are historical — new work uses GitHub issue numbers. Existing references in archived OpenSpec artifacts and Wiki pages are preserved as a frozen record. Lessons below that mention slice IDs (#6, #7) are historical-only as of Track 1 (2026-05-15).

1. **Run git from repo root.** Use absolute paths or `git -C <root>` to avoid CWD bugs after `cd` into subdirectories.
2. **Fetch before reporting git state.** Run `git fetch --prune` before reporting branch status, ahead/behind counts, or PR existence. Stale refs produce confidently wrong analysis.
3. **Run Superpowers `requesting-code-review` before `gh pr create`, not after.** Every PR — including 1-commit changes — gets a review pass first. "Too small to review" is never a valid reason to skip.
4. **Path/URL/env renames need a full-repo grep.** When renaming a route, endpoint, env var, port, or config key, grep the whole repo (Taskfile, workflows, docs, scripts) — not just obvious source files. This IS surgical (every site references the rename). Cleaning unrelated dead code is not.
5. **Trust the git hooks — don't re-run what they cover.** Pre-commit covers gofmt/golangci-lint on staged Go + tsc on staged TS. Pre-push covers full golangci-lint, go test, terraform fmt, frontend build+vitest+npm audit, gitleaks. After writing a change: `git add && git commit && git push`. Re-running them manually duplicates work.
6. **(Historical)** ~~Slice completion includes flipping `tasks.md` rows to `[x]`.~~ OpenSpec is frozen as of Track 1 (2026-05-15). New work closes via the linked GitHub issue, not via `tasks.md` flips. Kept here so older PR descriptions referencing this lesson are still resolvable.
7. **(Historical)** ~~Grep ROADMAP for slice ID collisions before opening an OpenSpec change.~~ Slice IDs are no longer used (Track 1, 2026-05-15). New work uses GitHub issue numbers, which are unique by construction.
8. **Verify dependency versions at the registry, not from memory.** When adding/bumping any dep (npm, Go module, Terraform provider, GitHub Action, Docker image, Homebrew, Clerk/AWS SDK), the version comes from the live registry or current docs page — never recollection. Re-verify when the slice that installs it starts.
9. **Non-slice perf fixes that block slice testability attach to the slice's PR with explicit commit-body justification.** When a perf fix isn't part of the slice's stated scope but is needed to playtest the slice end-to-end locally, ship it on the same branch with rationale stated explicitly. Do NOT split into a separate PR if it would block slice testing.
10. **Lockstep service config: capture EVERY consumer in the spec.** When two services share an identifier (queue URL, table name, env var), the spec's acceptance criteria must enumerate all sites. Define shared constants once in `Taskfile.yml::vars:` and reference from each env block — single source of truth.
11. **CD + Dependabot monitored by a scheduled Claude routine — don't inline-watch.** The `Reign CD + Dependabot monitor` routine fires twice daily (09:00 + 21:00 Europe/Amsterdam = `0 7,19 * * *` UTC) and opens a `priority:p0`+`area:devops`+`type:bug`+`status:blocks-prod` GitHub issue on any CD failure or critical/high Dependabot alert. **Don't run `gh run watch` after a merge** — let the routine surface failures. The inline-watch practice (motivated by the silent PR #102/103/104 failures on 2026-05-08) is replaced by this routine as of Track 1. The underlying lesson still applies as design context: _CI green is not CD green — TF state can fail on pre-existing drift, IAM policies can fail to attach, frontend sync can fail post-build._
12. **Unit tests on both sides of a boundary don't prove the contract.** Frontend mocks `fetch` to return `{ ok: true }` and backend tests call handlers directly with `httptest` — both pass while the wire format diverges. #179's HEAD/405 bug shipped to a merged-but-not-yet-deployed branch precisely this way: the frontend probe sent HEAD, chi's route was registered GET-only, the user saw "405 Method Not Allowed" in their browser the first time they ran the dev stack. The fix is procedural, not technical: cross-boundary changes get a Playwright e2e spec under `frontend/playwright/e2e/` that exercises the real wire (Vite → backend through the actual proxy, or `playwright-cli` against `task dev:up` for interactive checks). This is now step 6 of the Change Workflow.
13. **Trace from the call site before claiming an implementation is buggy.** When investigating a user-reported "X is broken", reading the suspected file in isolation can mislead — find where it's actually called from and follow the path to the rendered behaviour first. The #185 false alarm (timer "uses client wall-clock") came from reading `useTimer.ts` and not checking whether `DailyGameBoard.tsx` actually uses it (it doesn't — daily anchors on the server's `assignedAt` via a separate path in `GameBoard.tsx`). The user's "I thought we already do this" was the right read, dismissed too quickly. Cost: ~30 min + one filed-and-immediately-closed GitHub issue. The discipline: when a user says "I thought X already works", treat it as a hint to grep callers before re-reading internals. Same lesson applies to **"pre-existing failure" claims**: when a test fails during local smoke and the agent attributes it to a known historical issue, the lead must check CI on the same SHA before propagating that claim. The #191 retro almost filed a follow-up issue for a "consistently failing" e2e test that CI proved was passing 16/16 — the local failure was transient and got mislabelled as the older #179 HEAD/405 case.
14. **Subagents commit to whichever branch they're checked out on — verify before reporting DONE.** During #191 a docs subagent's commit (`74a3a4d`) landed on local `main` instead of the feature branch. Cause unknown — possibly an implicit `cd` during the agent's bash session or shell-state drift in the cheaper model. Recovery cost: cherry-pick to the feature branch + `git reset --hard origin/main` on local main. The defence is procedural, not technical: every implementer prompt should require the subagent to run `git branch --show-current` (or `git log -1 --format='%H on %s'`) as a self-review step BEFORE reporting DONE, and assert it matches the branch named in the prompt. Lead also spot-checks `git log main..HEAD` between tasks to catch drift early.

## Security: Baseline Gates (every cycle)

Run on every change:

1. **Secret scanning (pre-commit):** `gitleaks detect --source .` — blocks the commit if secrets found.
2. **Dependency audit (CI):** `govulncheck ./...` (backend) and `npm audit --audit-level=moderate` (frontend) — known vulnerabilities block merge.
3. **Superpowers `requesting-code-review` security pass:** runs on every change as part of the workflow; CRITICAL or HIGH findings block merge. The full `security-review-final` agent runs additionally when the deep-review trigger (next section) applies.

## Security: Deep Review Trigger (conditional)

Run the full `security-review-final` agent when the diff includes any of:
- `**/auth/**`, `**/middleware/**`
- `**/handler/*.go` (new or modified Lambda handlers)
- `go.mod`, `go.sum`, `package.json`, `package-lock.json` (dependency changes)
- `infra/**/*.tf` (IAM, networking, encryption)
- `docker-compose.yml`, `Dockerfile`
- `.github/workflows/**`
- Any file with `password`, `secret`, `token`, `credential`, `key` in path or content

Skip when the diff only touches: service logic, models, frontend components, tests, docs, archived OpenSpec artifacts.

## Security: Dependabot PRs (discipline)

GitHub Actions are pinned to commit SHAs (#113), not tags. The pin only buys safety if a human glances at each Dependabot bump before merging — confirm the new SHA matches the new `# vX.Y.Z` comment by spot-checking the action's release notes / commit log linked in the Dependabot PR body. **Do not auto-merge** `area:devops`+`type:infra` Dependabot PRs that touch `.github/workflows/**`; that collapses the trust model back to "whatever the bot says is `v6.0.3`" and undoes the SHA-pin discipline.

## Available Skills

Project-local skills in `.claude/skills/` (invoked by reading their `SKILL.md` and following the instructions):

- `architecture` — Per-area architecture rules (backend layered, frontend feature-folders, infra modules-vs-envs) with design-time and review-time drift greps
- `refinement` — Take backlog issues to Definition of Ready (composes `superpowers:brainstorming`, output to the issue DoR comment, no branch until implementation). See the Refinement + Autonomous Execution Contract.
- `parallel-plan` — Fan-out parallel approach comparison (5 approaches in parallel, then synthesize)
- `glossary` — Ubiquitous language glossary management
- `playwright-cli` — Browser automation for e2e testing (Microsoft's `@playwright/cli`)
- `write-simply` — Plain language writing
- `structure-clearly` — Pyramid principle document structure
- `retro` — Retrospective on the change

**Plugin-based skills** (require Claude Code plugin install):
- `superpowers:*` — Full workflow chain: `brainstorming`, `using-git-worktrees`, `writing-plans`, `subagent-driven-development`, `executing-plans`, `test-driven-development`, `requesting-code-review`, `receiving-code-review`, `finishing-a-development-branch`, `systematic-debugging`, `verification-before-completion`, `dispatching-parallel-agents`, `using-superpowers`, `writing-skills`. Install: `/plugin install superpowers@claude-plugins-official`.
- `frontend-design`, `ui-ux-pro-max` — Component-level design guidance, UX patterns, design system

## How to Use the Agents

### Assisted Mode (small changes, bug fixes — under ~5 files)

For one-off small work, skip the full Superpowers chain:

1. Dispatch a fresh subagent (via Task tool) to do the work. The subagent auto-loads `/CLAUDE.md` plus the relevant subdirectory CLAUDE.md based on the files it touches.
2. Subagent implements + tests + commits.
3. Run build verification (Superpowers `verification-before-completion`).
4. Pre-commit hook covers `gitleaks` + `golangci-lint`/`gofmt` (Go) + `tsc` (TS).
5. Run Superpowers `requesting-code-review` over the diff. Fix CRITICAL/HIGH findings.
6. Spawn `code-review-final` (+ `security-review-final` if the deep-review trigger applies).
7. Fix review comments, push, merge.

### Full Pipeline Mode (features, multi-area changes)

Follow the full Change Workflow above. The lead agent (Claude Code) reads each Superpowers skill in turn and dispatches subagents per `subagent-driven-development`. For optional/ambiguous design forks, the lead agent stops and asks the human (HITL rule).

The `architecture` skill is consulted at two points: during `brainstorming` (design-time check) and during `requesting-code-review` + `code-review-final` (review-time drift greps).

## Key Pipeline Rules

- **Sweep enforcement:** every review finding includes a grep command. Fix agents fix ALL matches, not just the reported file.
- **Cross-stack contract alignment:** when backend and frontend run in parallel, the frontend API service task MUST read the actual backend DTOs before writing interfaces.
- **PR Key Decisions section:** include intentional design choices in PR descriptions to prevent reviewers flagging them as bugs. **Cap at 3-5 items and ~40-60 line bodies overall** — see the "PR description shape" rule under Change Workflow above for the full convention.
- **Two review passes max** — after pass 2, the PR is considered ready for merge.
- **Architecture drift = blocking finding.** A `architecture: drift in <file>` finding from the architecture skill blocks merge unless the PR is explicitly marked as introducing a documented exception (in the Key Decisions section).
