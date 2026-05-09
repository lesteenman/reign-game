# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Reign** (working title) is a puzzle game available as a Progressive Web App. Players place markers on a colored grid subject to row, column, region, and adjacency constraints. The default presentation is minimalist and abstract; alternative visual themes (including a classic "Queens" chess theme) are available. It offers curated puzzles across difficulty levels, daily challenges with speed-based leaderboards, and a freemium monetization model (no ads).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, AWS Lambda, API Gateway (REST) |
| Frontend | React 19, TypeScript, Vite, Tailwind CSS, Workbox (PWA) |
| Database | DynamoDB (on-demand pricing) |
| Testing | Go test (backend), Vitest (frontend unit), Playwright (e2e) |
| Build | Go build / Taskfile (backend), npm + Vite (frontend) |
| Infrastructure | Terraform, AWS (S3, CloudFront, Lambda, API Gateway, DynamoDB) |
| CI/CD | GitHub Actions — CI on PR, CD on merge to main |
| Dev Environment | LocalStack (local DynamoDB), Vite dev server (frontend) |

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

Every change — feature, fix, or refactor — follows this pipeline:

```
1. OpenSpec Explore    → parallel-plan + design-grill skills (understand the problem)
2. OpenSpec Propose    → spec artifacts (define the solution)
3. UI/UX Design        → wireframes + Nano Banana 2 prompts (if visual change)
4. Implementation      → red/green TDD, feature branch, commit per artifact
5. Security Scan       → gitleaks + dependency audit + review-local security agent
6. Self-Review         → reviewer + writer iterate until consensus (escalate to human if stuck)
7. OpenSpec Archive    → sync artifacts with final implementation
8. CD Watch            → after merge, poll the CD run for the merge SHA; surface failures immediately
9. Retro               → retrospective on the change
```

- **TDD is non-negotiable** for both backend and frontend. Red/green/refactor — write a failing test first.
- **Feature branches** for all work. Never commit directly to main.
- **Commits** happen after every artifact delivery (specs, wireframes, completed code), not just at the end.
- **Self-review** continues until reviewer agent and implementation agent agree. Escalate to human after two rounds with no consensus.
- **Each phase's `tasks.md` ends with a Verification Checklist** designed during design-flow. Walk it at phase close — every item gets a citation (file:line, test name, grep result, UI assertion).
- **PR description includes a "Key Decisions" section** listing intentional design choices. Phase-level PRs also include a "Workarounds shipped" section.

## Agent Teams

This project uses custom AI agents in `.claude/agents/`. The lead agent (Claude Code) orchestrates — it does NOT implement code itself.

| Agent | Role | When |
|-------|------|------|
| `product-owner` | Vision guardian, acceptance criteria, scope decisions | Before implementation |
| `design-flow` | Full design phase: explore, stress-test, glossary alignment, spec generation | Before implementation |
| `workflow-orchestrator` | Pipeline orchestration, team coordination, glossary enforcement | Full Pipeline Mode |
| `backend-dev` | Go implementation, API design, DynamoDB, Lambda + TDD | Any back-end work |
| `frontend-dev` | React/TS implementation, PWA, responsive UI + TDD | Any frontend work |
| `devops-engineer` | Terraform, GitHub Actions, AWS architecture, monitoring | Infrastructure and CI/CD work |
| `ui-ux-designer` | Wireframes, interaction design, brand guidelines | Visual design phases |
| `tester` | E2E test plans, edge cases, regression hunting, Playwright | After implementation |
| `code-review-final` | Code quality review of PRs | After all implementation |
| `security-review-final` | Security review (conditional — see Security section) | When diff touches security-sensitive files |

Sub-agents use skills by reading the skill's `SKILL.md` and following its instructions. They do NOT have access to a `Skill()` tool.

## Human-in-the-Loop + Notifications

This project is built by agents with a human supervisor who is NOT actively watching the screen. Pause-and-ask is always cheaper than guessing wrong.

**HITL Rule (CRITICAL — applies to design forks):**
- NEVER answer your own design questions or auto-approve decisions.
- NEVER assume you know what the human would choose.
- ALWAYS present options and wait for an explicit response.
- Confirm alignment before moving to the next phase.

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

After cloning the repo, configure git to run the project's hooks:

```bash
git config core.hooksPath .githooks
```

Without this, the pre-commit and pre-push gates silently don't run, and CI catches what your local shell should have.

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
- **ROADMAP.md** — Phased roadmap with explicit todos + known issues.
- **BRAND_GUIDELINES.md** — Design system. Required before any frontend visual work.

## Domain Conventions

Domain-specific conventions, logging rules, and per-domain lessons live in the agent files:

- **Backend (Go) + logging + DynamoDB access patterns** — see `.claude/agents/backend-dev.md`
- **Frontend (React + TypeScript) + brand integration** — see `.claude/agents/frontend-dev.md`
- **Infrastructure (Terraform, GitHub Actions, AWS)** — see `.claude/agents/devops-engineer.md`
- **Pipeline orchestration (parallel spawn rules, stall protocol)** — see `.claude/agents/workflow-orchestrator.md`

## Database (DynamoDB)

- On-demand (pay-per-request) billing — no provisioned capacity.
- Single-table design where practical, separate tables when access patterns diverge.
- All table definitions in Terraform (`infra/modules/database/`).
- No ORM — AWS SDK for Go v2 directly.
- Local development: LocalStack.

## Lessons (cross-cutting)

Slice ID scheme uses `R-<phase>-<slice>` where `<phase>` is the integer phase number and `<slice>` is `exploration` or a strictly increasing 2-digit number (`01`, `02`, …). Already-shipped slices keep historical IDs (e.g., `R-067a`, `R-08C`) — those references are preserved in archived OpenSpec artifacts.

1. **Run git from repo root.** Use absolute paths or `git -C <root>` to avoid CWD bugs after `cd` into subdirectories.
2. **Fetch before reporting git state.** Run `git fetch --prune` before reporting branch status, ahead/behind counts, or PR existence. Stale refs produce confidently wrong analysis.
3. **Run review-local before `gh pr create`, not after.** Every PR — including 1-commit changes — gets the 4-agent review loop first. "Too small to review" is never a valid reason to skip.
4. **Path/URL/env renames need a full-repo grep.** When renaming a route, endpoint, env var, port, or config key, grep the whole repo (Taskfile, workflows, docs, scripts) — not just obvious source files. This IS surgical (every site references the rename). Cleaning unrelated dead code is not.
5. **Trust the git hooks — don't re-run what they cover.** Pre-commit covers gofmt/golangci-lint on staged Go + tsc on staged TS. Pre-push covers full golangci-lint, go test, terraform fmt, frontend build+vitest+npm audit, gitleaks. After writing a change: `git add && git commit && git push`. Re-running them manually duplicates work.
6. **Slice completion includes flipping `tasks.md` rows to `[x]`.** OpenSpec's `tasks.md` is the single source of truth for slice state. Update the row in the slice's PR, not as post-hoc sweep. Parallel slices on the same `tasks.md` produce mechanical merge conflicts — keep both `[x]` flips when resolving.
7. **Grep ROADMAP for slice ID collisions before opening an OpenSpec change.** New IDs often collide with pre-declared future-phase IDs. `grep -n "R-<phase>" ROADMAP.md` before claiming a range.
8. **Verify dependency versions at the registry, not from memory.** When adding/bumping any dep (npm, Go module, Terraform provider, GitHub Action, Docker image, Homebrew, Clerk/AWS SDK), the version comes from the live registry or current docs page — never recollection. Re-verify when the slice that installs it starts.
9. **Non-slice perf fixes that block slice testability attach to the slice's PR with explicit commit-body justification.** When a perf fix isn't part of the slice's stated scope but is needed to playtest the slice end-to-end locally, ship it on the same branch with rationale stated explicitly. Do NOT split into a separate PR if it would block slice testing.
10. **Lockstep service config: capture EVERY consumer in the spec.** When two services share an identifier (queue URL, table name, env var), the spec's acceptance criteria must enumerate all sites. Define shared constants once in `Taskfile.yml::vars:` and reference from each env block — single source of truth.
11. **Watch CD after every merge to main.** After a PR merges, fetch the latest `cd.yml` run for that commit (`gh run list --workflow=cd.yml --branch=main --limit=1`), poll it to completion (`gh run watch <id>`), and surface failures immediately as in-flight blocking work — don't move on assuming green. CI green is not CD green: TF state can fail on pre-existing drift (PR #102 BucketNotEmpty), IAM policies can fail to attach, frontend sync can fail post-build. Three consecutive silent CD failures (PR #102/103/104, 2026-05-08) only surfaced when the user-facing acc surface degraded behind a CloudFront cache TTL — none of which would have happened if the first failure had been caught at merge time.

## Security: Baseline Gates (every cycle)

Run on every change:

1. **Secret scanning (pre-commit):** `gitleaks detect --source .` — blocks the commit if secrets found.
2. **Dependency audit (CI):** `govulncheck ./...` (backend) and `npm audit --audit-level=moderate` (frontend) — known vulnerabilities block merge.
3. **review-local security agent:** runs on every change. CRITICAL or HIGH findings block merge.

## Security: Deep Review Trigger (conditional)

Run the full `security-review-final` agent when the diff includes any of:
- `**/auth/**`, `**/middleware/**`
- `**/handler/*.go` (new or modified Lambda handlers)
- `go.mod`, `go.sum`, `package.json`, `package-lock.json` (dependency changes)
- `infra/**/*.tf` (IAM, networking, encryption)
- `docker-compose.yml`, `Dockerfile`
- `.github/workflows/**`
- Any file with `password`, `secret`, `token`, `credential`, `key` in path or content

Skip when the diff only touches: service logic, models, frontend components, tests, docs, OpenSpec artifacts.

## Available Skills

Skills in `.claude/skills/` are invoked by reading their `SKILL.md` and following the instructions:

- `design-grill` — Stress-test design decisions
- `parallel-plan` — Fan-out parallel approach comparison
- `glossary` — Ubiquitous language glossary management
- `review-local` — 4-agent parallel local code review
- `gitlab-code-review` — PR review via VCS CLI
- `playwright-cli` — Browser automation for e2e testing
- `write-simply` — Plain language writing
- `structure-clearly` — Pyramid principle document structure
- `retro` — Retrospective on the change

**Plugin-based skills** (require Claude Code plugin install — see CONTRIBUTING.md):
- `frontend-design` — Component-level design guidance
- `ui-ux-pro-max` — UX patterns, interaction design, design system

## How to Use the Agents

### Assisted Mode (small changes, bug fixes — under ~5 files)

1. Spawn the appropriate implementation agent (e.g., `backend-dev`).
2. Agent implements + tests + commits.
3. Run build verification.
4. Run `gitleaks detect --source .` — block if secrets found.
5. Read `skills/review-local/SKILL.md` and follow its instructions.
6. Fix CRITICAL/HIGH security findings before continuing.
7. Spawn `code-review-final` (+ `security-review-final` if deep review triggered).
8. Fix review comments, push, merge.

### Full Pipeline Mode (features, multi-module changes)

Spawn the `workflow-orchestrator` agent. It distributes tasks in parallel, runs build verification + pre-commit checks + local review, creates the PR, spawns reviewers, and Playwright e2e agents. For optional/ambiguous steps, it asks the human.

See `.claude/agents/workflow-orchestrator.md` for full pipeline details.

## Key Pipeline Rules

- **Sweep enforcement:** every review finding includes a grep command. Fix agents fix ALL matches, not just the reported file.
- **Cross-stack contract alignment:** when backend and frontend run in parallel, the frontend API service task MUST read the actual backend DTOs before writing interfaces.
- **PR Key Decisions section:** include intentional design choices in PR descriptions to prevent reviewers flagging them as bugs.
- **Two review passes max** — after pass 2, the PR is considered ready for merge.
