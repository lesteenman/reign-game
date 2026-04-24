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
```

## Running the Dev Stack (STANDARD — always use these)

The dev stack (LocalStack + backend + generator + frontend) runs as background
processes with logs streamed to `./logs/*.log`. **Always use these tasks** — do
not launch `go run ./cmd/api` or `npm run dev` directly. Doing so breaks the
shared logging/lifecycle contract and leaves orphan processes.

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
- LocalStack readiness check waits for BOTH the `/_localstack/health` endpoint AND the init-aws.sh script to finish (puzzle-generation queue exists + puzzle-pool table is ACTIVE). Without this, services race with init and log spurious `NonExistentQueue` errors.
- Task's built-in shell (mvdan/sh) runs commands in-process, so `$!` and `kill -0` against external PIDs are unreliable. Generator lifecycle blocks wrap their logic in `bash <<'BASH' ... BASH` heredocs to get real POSIX semantics.
- `./logs/` is gitignored.
- LocalStack runs in Docker via the existing `docker-compose.yml`; init script `.localstack/init-aws.sh` creates the `puzzle-pool` DynamoDB table, the SQS queues, and seeds initial CONFIG items.

**After changing Go source:** `task dev:restart:backend` and/or `task dev:restart:generator` (neither is hot-reloaded).
**After changing frontend source:** Vite HMR handles most updates; if you edit `vite.config.ts` or similar, `task dev:restart:frontend`.

**Taskfile shell pitfalls (read before editing Taskfile.yml):**
Task runs `cmds:` blocks in its built-in interpreter (`mvdan.cc/sh`), not system sh or bash. It mostly matches POSIX but diverges in a few places that bite process-lifecycle code:

- `$!` after a background job returns a **goroutine handle** (e.g. `g1`), not an OS PID. Capturing `echo $! > file.pid` stores garbage.
- `kill -0 "$PID"` against **external** OS PIDs is unreliable — it can report "not alive" for a process that is demonstrably running.
- `disown` is not implemented.
- `set -e` behavior around command substitution differs from bash in edge cases.

When a task needs to track a backgrounded process by PID (anything without a port to probe via `lsof -ti:PORT`), wrap the whole block in a bash heredoc:

```yaml
cmds:
  - |
    bash <<'BASH'
    set -e
    nohup long-running-cmd > log 2>&1 </dev/null &
    echo $! > pid
    # ...check readiness, verify alive, etc.
    BASH
```

Port-based lifecycle (`lsof -ti:PORT` for status, `kill $(lsof -ti:PORT)` for down) works fine in Task's shell and is preferred whenever a port exists.

## Testing

- Always run the full test suite after making changes
- After fixing one bug, verify no regressions were introduced before moving on
- When writing controller tests, check if the test security config has specific auth behavior
- All unit tests must use **Arrange-Act-Assert** structure with explicit `// Arrange`, `// Act`, `// Assert` comments separating the sections. This applies to both frontend (Vitest) and backend (Go) tests.

## Git Hooks

Pre-push hook (`.githooks/pre-push`) runs before every push:
- Backend: `golangci-lint run` + `go test ./...`
- Frontend: `npm run build` (includes tsc) + `npx vitest run` + `npm audit`
- Secret scan: `gitleaks detect`

Configure with: `git config core.hooksPath .githooks`

## Dev Server Ports

| Service    | Port | Started by                 |
|------------|------|----------------------------|
| Frontend   | 5180 | `task dev:up:frontend`     |
| Backend    | 5181 | `task dev:up:backend`      |
| Generator  | —    | `task dev:up:generator` (SQS consumer, no HTTP port; PID in `logs/generator.pid`) |
| LocalStack | 4566 | `task dev:up:localstack`   |

Frontend already binds `--host 0.0.0.0` (for mobile testing over LAN) and the Vite proxy forwards `/api/*` to `localhost:5181`. All backend routes live under `/api/` — SPA routes (e.g., `/admin` page) stay on the frontend. Do not start services with raw `go run`/`npm run dev` — always go through `task dev:up` (see "Running the Dev Stack" above).

## Project Structure

See **[PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)** for the full project tree and API endpoints. Search by domain keyword to locate any file.

## Project-Specific Conventions

### General
- Monorepo: frontend/, backend/, infra/, design/ at the root
- Feature branches for all changes, merged via PR to main
- Commit after every artifact delivery (specs, wireframes, code)
- Every change follows: OpenSpec explore -> propose -> UI/UX design (if visual) -> implementation -> archive/sync -> retro
- TDD (red/green) for all implementation — backend AND frontend
- Self-review session after implementation until reviewer and writer agree

### Backend (Go)
- Standard Go project layout: cmd/ for entry points, internal/ for private packages
- Table-driven tests preferred
- Exported functions must have doc comments
- Error handling: wrap errors with context (`fmt.Errorf("doing X: %w", err)`)
- No global mutable state — pass dependencies via struct fields
- DynamoDB single-table design where practical

### Backend logging

Stdlib `log` only — no `slog`, no third-party loggers. Small project, small surface.

- **Format.** Every log line starts with `<subsystem>: <what>`. Subsystem is the handler name, package role, or service (`admin pool`, `config modes`, `serve handler`, `generator`). Keeps grep-by-feature trivial.
- **Levels are implicit.** `log.Printf` for warn/error. `log.Fatal*` is reserved for "can't continue at all" — startup failures, missing required config. Never for request-path errors.
- **Warnings get an explicit `WARN:` prefix** so grep can find them. Example: `"WARN: generator: safety-net fired 2 times on puzzle X (seed=Y)"`.
- **Pure packages stay silent.** `backend/internal/generator/` has zero `log.` calls. If the pure layer needs to surface a signal, it goes through return values or struct fields (e.g. `Metrics.SafetyNetTrips`) and a caller (worker, handler) logs. Keeps the generator testable and side-effect-free.
- **Per-message lines** (e.g. `"generator: produced puzzle X …"`) use key=value pairs separated by commas so they parse cleanly: `key1=val1, key2=val2`.

### Frontend (React + TypeScript)
- Functional components only, no class components
- Custom hooks for reusable logic (useGame, useTimer, usePuzzle)
- Strict TypeScript — no `any`, no type assertions without justification
- Component files: PascalCase (Grid.tsx), hooks: camelCase (useGame.ts)
- TDD with Vitest — write failing test first, then implementation
- All components must be responsive (mobile-first)
- Accessibility: WCAG 2.1 AA minimum

### Infrastructure
- All infrastructure in Terraform — no manual AWS console changes
- Terraform modules for reusable components (frontend, api, database)
- Single environment initially, parameterized for dev/prod split later

### Frontend Design Rules

**MANDATORY**: When implementing or modifying ANY frontend visual code (components, pages, layouts, styles), you MUST:

1. Read `skills/frontend-design/SKILL.md` and follow its instructions BEFORE writing any UI code
2. Read `skills/ui-ux-pro-max/SKILL.md` and follow its instructions with `--design-system --persist` to generate brand guidelines
3. Persist the output as `BRAND_GUIDELINES.md` in the project root
4. Reference `BRAND_GUIDELINES.md` for all color palettes, font pairings, spacing, and UX patterns
5. Never output plain/generic styling — every component must reflect the brand guidelines

`BRAND_GUIDELINES.md` is the single source of truth for visual design decisions. All frontend agents and reviewers reference it. If it doesn't exist when frontend visual code is being written, that is a CRITICAL review finding.

## Roles

| Role | Identity | Access |
|------|----------|--------|
| FREE | Fully anonymous, no account | Practice puzzles, daily challenge, local stats, see own percentile |
| PREMIUM | Authenticated (OAuth, one-time purchase) | All free features + full archive, leaderboard identity, detailed stats, cross-device sync, premium themes |
| ADMIN | Authenticated | Curation UI, puzzle management, generation tooling |

## Key References

- **GLOSSARY.md** -- Ubiquitous Language glossary. Consult before using domain terms in specs, designs, and code.
- **PROJECT_STRUCTURE.md** -- Full project tree + all API endpoints. Search by domain keyword to locate any file.
- **GAME_DESIGN.md** -- Living game design vision document. The north star for what we're building.
- **ROADMAP.md** -- Phased roadmap with explicit todos + known issues. The Jira-lite task tracker.
- **BRAND_GUIDELINES.md** -- Design system (colors, fonts, spacing, component patterns). Generated by following the ui-ux-pro-max skill with `--design-system --persist`. Required before any frontend visual work.

---

## Agent Teams

This project uses custom AI agents that work together as a team. The lead agent (Claude Code) orchestrates the pipeline -- it does NOT implement code itself but distributes tasks to sub-agents.

### Agent Architecture

Agents are markdown files in `.claude/agents/` that define specialized roles. They are spawned as sub-agents via the `Agent` tool. Sub-agents use skills by reading the skill's SKILL.md file and following its instructions -- they do NOT have access to a `Skill()` tool.

### How Agents Use Skills

Skills are `.md` files in `.claude/skills/`. Agents use them by:
1. Reading the skill file (e.g., `skills/design-grill/SKILL.md`)
2. Following its instructions completely — executing the full process described in that file

Agents must NOT just summarize or paraphrase a skill. They must read and execute.

### Lessons from Past Reviews

Pipeline, CI, infra, and git-hook lessons live on the **devops-engineer** agent (`.claude/agents/devops-engineer.md`), not here — they only apply when that agent is running. Always spawn `devops-engineer` for any change under `.github/workflows/`, `infra/`, `.githooks/`, `docker-compose.yml`, `Dockerfile`, or workflow-shaped changes to `Taskfile.yml`. The agent owns its own checklists.

1. **Parallel agent spawning:** When spawning parallel agents (e.g., backend-dev + frontend-dev), always use a single message with multiple Agent tool calls. Never spawn one agent, wait for it, then spawn another — this wastes time and breaks the parallelism the task plan designed for.
2. **Git commands from repo root:** Always run git commands from the repo root. Use absolute paths or `git -C <repo-root>` to avoid working-directory issues after `cd` into subdirectories.
3. **Touch/pointer e2e tests first:** For any touch/pointer interaction code, write Playwright e2e tests before unit tests. jsdom does not simulate synthesized mouse events after touch events, so unit tests pass while the actual mobile experience is broken. The touch double-fire bug (Phase 1) was only caught by user playtesting.
4. **Sub-agents must use Write/Edit, not Bash for files:** When spawning implementation agents, explicitly instruct them to use the Write and Edit tools for file creation — not Bash with cat/heredoc. Bash file writes may still prompt for user approval even in bypassPermissions mode.
5. **First-paint correctness for visual components:** Never render a component at a default/placeholder size then resize after measuring. Use CSS-based sizing or defer rendering until the container is measured. Layout flicker is a user-visible bug.
6. **Even pixel values for SVG strokes:** Use 2px, 4px — never 2.5px or other subpixel values. Subpixel stroke widths cause anti-aliasing artifacts at line intersections visible on both standard and retina displays.
7. **Lint before commit, not just at push:** Run `golangci-lint run` (backend) and `npx tsc -b` (frontend) before committing. The pre-push hook catches these, but late failures waste time on fix-up commits. Two Phase 2 commits were purely lint fixes that could have been avoided.
8. **Float API params: always test NaN and Inf:** When adding float parameters to APIs, explicitly test `NaN` and `Inf` inputs. `strconv.ParseFloat` accepts these as valid, and compound range checks like `x < 0 || x > 1` evaluate to false for NaN, letting it through. Use `math.IsNaN` explicitly.
9. **Validate URL params before type assertion:** When frontend reads URL params and uses them as typed values (enums, numbers), validate against known values before type assertion. URL params are always `string | null` — invalid values passed unchecked will reach the API.
10. **DynamoDB `Limit` applies before `FilterExpression`.** When using `Query` with both `Limit` and `FilterExpression`, DynamoDB reads up to Limit items *then* filters. `Limit=1` with a status filter can return 0 results even when matching items exist further in the partition. Either omit Limit (for small partitions) or paginate.
11. **Sub-agents must run lint and fmt before committing.** Backend agents: run `golangci-lint run` (or `go vet ./...` if unavailable). Devops agents: run `terraform fmt -recursive -check`. Frontend agents already run `tsc -b`. Explicit instructions in agent prompts are required — sub-agents don't read pre-push hooks.
12. **Fetch before reporting git state:** Before reporting branch status, upstream existence, "PR exists?", or ahead/behind counts, run `git fetch --prune` first. Stale refs (especially after a branch is deleted post-merge) produce confidently wrong analysis and push scoping decisions down the wrong path. Treat local refs as cache that needs invalidating, not source of truth.
13. **Run review-local before `gh pr create`, not after. No exceptions for small PRs.** Every PR, including 1-commit surgical changes, gets the 4-agent review loop first. If the diff is truly trivial, the review costs 60 seconds; if it isn't, the review was needed. Phase 4.5 caught a MAJOR Taskfile bug this way. R-067a (PR #39) skipped the review because "the change felt small" — it happened to be clean, but that was luck, not process. "It's too small to review" is never a valid reason to skip.
14. **Path/URL/env renames need a full-repo grep.** When renaming a route, endpoint, environment variable, port, or config key, grep the *entire* repo — `Taskfile.yml`, `.github/workflows/**`, `docker-compose.yml`, all `*.md` docs, `CLAUDE.md`, `PROJECT_STRUCTURE.md`, `ROADMAP.md`, shell scripts — not just the obvious source files. Dev tooling and docs silently drift out of sync otherwise. Phase 4.5's `/health` → `/api/health` migration missed the Taskfile readiness probe on the first pass; only review-local's sweep caught it.
15. **Trust the git hooks — do not re-run what they cover.** `.githooks/pre-commit` runs `gofmt`/`golangci-lint` (on staged Go) and `tsc -b` (on staged TS). `.githooks/pre-push` runs full `golangci-lint`, `go test ./...`, `terraform fmt -recursive -check`, `npm run build` (includes tsc), `npx vitest run`, `npm audit --audit-level=moderate`, and `gitleaks detect --source .`. After writing a change, go straight to `git add && git commit && git push` — if hooks fail, fix the specific issue they reported. Running these tools manually before the hooks duplicates work, clutters terminal output, and wastes wall-clock time. It is appropriate to run them directly during TDD iterations or to debug a specific failure; it is not appropriate as a routine pre-commit/pre-push ritual.
16. **Persisted data shapes live in `storage/` (frontend) or `repository/` (backend), not in the consumer.** If a type is going to be saved to IndexedDB, DynamoDB, or any store, define it *once* in the storage/repository module and import it from every consumer (hooks, services, handlers). Do not redeclare a shape like `History` in a hook and `GameHistory` in storage — they will drift. Phase 4.6 had to unify `History` / `GameHistory` mid-review for exactly this reason.
17. **Slice completion includes flipping the `tasks.md` status row to `[x]`.** OpenSpec's `tasks.md` status table is the single source of truth for slice state. A slice is not done until the code ships AND the row is updated in the same branch. Phase 5's pre-main cleanup had to refresh 7 of 11 rows that stayed `[ ]` for weeks after their deliverables had shipped — the status column became useless for planning. Update the row as a required artifact in the slice's PR, not as a post-hoc sweep.
18. **Before opening an OpenSpec change, grep ROADMAP for ID collisions.** New slice IDs often collide with pre-declared IDs in later phase blocks. When creating a change that claims IDs like `R-062..R-06D`, first `grep -n "R-06[0-9A-F]" ROADMAP.md` — if any of those IDs are already reserved for a future phase, renumber one side before the slice lands. Phase 5 shipped R-062..R-06D while ROADMAP Phase 6/7/8 still pre-declared R-063..R-068; the collision went untreated through 14 slices and KI-007's "Related" column ended up citing a nonexistent ID.
19. **Never guess dependency versions from memory — check the registry.** When adding or bumping an npm / Go / Maven / PyPI dependency, the version number must come from the live registry (`npm view <pkg> version`, `go list -m -versions <module>`, etc.) or the package's current docs page — never from recollection. Training data goes stale; packages are yanked, re-numbered, or change major versions without us knowing. Spec docs and code that name a specific version (e.g. `@clerk/clerk-react@x.y.z`, `github.com/clerk/clerk-sdk-go/v2`) should be verified at the moment they're written, and re-verified when the slice that installs them starts. Applies to both implementation and design-phase artifacts that commit to a particular SDK surface.

### Human-in-the-Loop Rule (CRITICAL)

**NEVER:**
- Answer your own design questions or auto-approve decisions
- Assume you know what the human would choose
- Skip asking the human because the answer seems obvious

**ALWAYS:**
- Present decisions and options directly to the human
- Wait for the human's explicit response before proceeding
- Confirm alignment before moving to the next phase

### Available Agents

| Agent | Role | When to Use |
|-------|------|-------------|
| `product-owner` | Vision guardian, acceptance criteria, scope decisions, prioritization | Before implementation — validates what to build and why |
| `design-flow` | Full design phase: explore, stress-test, glossary alignment, spec generation | Before implementation — when a feature needs design |
| `workflow-orchestrator` | Pipeline orchestration, team coordination, glossary enforcement | Full Pipeline Mode — orchestrates all other agents, enforces glossary term consistency |
| `backend-dev` | Go implementation, API design, DynamoDB, Lambda handlers + TDD | Any back-end work |
| `frontend-dev` | React/TS implementation, PWA, responsive UI + TDD | Any frontend work |
| `devops-engineer` | Terraform, GitHub Actions, AWS architecture, monitoring | Infrastructure and CI/CD work |
| `ui-ux-designer` | Wireframes, interaction design, brand guidelines, Nano Banana 2 prompts | Visual design phases |
| `tester` | E2E test plans, edge cases, regression hunting, Playwright | After implementation — verify features work |
| `code-review-final` | Code quality review of PRs | After all implementation is complete |
| `security-review-final` | Security review of PRs (conditional) | Only when diff touches security-sensitive files |

### Security: Baseline Gates (MANDATORY — every cycle)

These checks run on EVERY change, no exceptions:

1. **Secret scanning (pre-commit):** Run `gitleaks detect --source .` before every commit. If secrets are found, the commit MUST be blocked. Never commit API keys, tokens, passwords, or high-entropy strings.
2. **Dependency audit (CI):** `govulncheck ./...` (backend) and `npm audit --audit-level=moderate` (frontend) run on every PR. Known vulnerabilities block merge.
3. **review-local security agent:** The security agent in `review-local` runs on every change. CRITICAL or HIGH findings from this agent block merge — they must be fixed before proceeding.

### Security: Deep Review Trigger (conditional)

Run the full `security-review-final` agent when the diff includes files matching ANY of:
- `**/auth/**`, `**/middleware/**`
- `**/handler/*.go` (new or modified Lambda handlers — API attack surface)
- `go.mod`, `go.sum`, `package.json`, `package-lock.json` (dependency changes — supply chain risk)
- `infra/**/*.tf` (infrastructure changes — IAM, networking, encryption)
- `docker-compose.yml`, `Dockerfile` (container security)
- `.github/workflows/**` (CI/CD pipeline changes)
- Any file with `password`, `secret`, `token`, `credential`, `key` in its path or content

Skip deep security review when the diff only touches: service logic, models, frontend components, tests, docs, OpenSpec artifacts.

### Change Workflow (MANDATORY for all changes)

Every change — feature, fix, or refactor — follows this pipeline:

```
1. OpenSpec Explore    → parallel-plan + design-grill skills (understand the problem)
2. OpenSpec Propose    → spec artifacts (define the solution)
3. UI/UX Design        → wireframes + Nano Banana 2 prompts (if visual change)
4. Implementation      → red/green TDD, feature branch, commit per artifact
5. Security Scan       → gitleaks + dependency audit + review-local security agent (every cycle)
6. Self-Review         → reviewer + writer iterate until consensus (escalate to human if stuck)
7. OpenSpec Archive    → sync artifacts with final implementation
8. Retro               → retrospective on the change
```

**TDD is non-negotiable** for both backend and frontend. Write a failing test first, then make it pass, then refactor. No exceptions.

**Self-review** continues until both the reviewer agent and implementation agent agree the code is ready. The reviewer must be critical but pragmatic — if there's a good reason to deviate from convention, that's acceptable with justification. If there's genuinely no consensus after two rounds, escalate to the human.

**Commits** happen after every artifact delivery: specs, wireframes, images, and completed code. Not just at the end.

**Feature branches** for all work. Never commit directly to main.

### How to Use the Agents

#### Assisted Mode (small changes, bug fixes)

For changes under ~5 files or single-module work, use agents directly without orchestration:

1. Spawn the appropriate implementation agent (e.g., `backend-dev`)
2. Agent implements + tests + commits
3. Run build verification
4. Run `gitleaks detect --source .` — block if secrets found
5. Read `skills/review-local/SKILL.md` and follow its instructions on the changed code
6. If review-local security agent finds CRITICAL/HIGH → fix before continuing
7. Spawn `code-review-final` agent (+ `security-review-final` if deep review triggered)
8. Fix review comments, push, merge

#### Full Pipeline Mode (features, multi-module changes)

For larger features, spawn the `workflow-orchestrator` agent. It will:

- Distribute tasks to implementation agents in parallel
- Run build verification, pre-commit quality checks, and local review
- Create the PR and spawn `code-review-final` (and `security-review-final` if the diff touches security-sensitive files)
- Spawn Playwright e2e test agents if the project has a frontend
- For optional or ambiguous steps, ask the human before executing

See the `workflow-orchestrator` agent definition for the full pipeline details.

### Available Skills

Skills are invoked by reading their SKILL.md file and following the instructions. Available in `.claude/skills/`:

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

- `frontend-design` — Component-level design guidance (plugin: `frontend-design@claude-plugins-official`)
- `ui-ux-pro-max` — UX patterns, interaction design, design system (plugin: `ui-ux-pro-max@ui-ux-pro-max-skill`)

### Key Rules

- **Sweep enforcement**: Every review finding includes a grep command. Fix agents MUST run the sweep and fix ALL matches -- not just the reported file
- **Cross-stack contract alignment**: When backend and frontend run in parallel, the frontend API service task MUST read the actual backend DTOs before writing interfaces
- **PR Key Decisions**: Include a "Key Decisions" section in PR descriptions listing intentional design choices. Prevents review agents from flagging them as bugs

## Code Review Workflow

- When reviewing PRs, structure findings by category: security, efficiency, code quality, reuse
- Post findings directly to PR via `gh` CLI
- Two review passes max -- after pass 2, the PR is considered ready for merge

## Database (DynamoDB)

- On-demand (pay-per-request) billing mode — no provisioned capacity
- Single-table design where practical, separate tables when access patterns diverge significantly
- Partition key design must avoid hot partitions (e.g., daily puzzle leaderboards need careful key design)
- Local development: use LocalStack or DynamoDB Local for testing
- No ORM — use the AWS SDK for Go v2 directly
- All table definitions managed in Terraform (infra/modules/database/)
