# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## Project Overview

**Reign** (working title) is a puzzle game shipped as a Progressive Web App. Players place markers on
a colored grid under row, column, region, and adjacency constraints. The default look is minimalist
and abstract; alternative themes (including a classic "Queens" chess theme) are available. It offers
curated puzzles across difficulty levels, daily challenges with speed-based leaderboards, and a
freemium model (no ads).

Code lives in `backend/` (Go), `frontend/` (React PWA), and `infra/` (Terraform). Full tree and API
endpoints: PROJECT_STRUCTURE.md.

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

**Frontend transition state.** Tamagui + TanStack Query landed in Track 2; #176 migrated existing code
into the Bulletproof React feature-folder layout and the `LoadState` → TanStack hooks transition, and
retired Tailwind. New code uses Tamagui from the start; chrome styling is inline `style={}` + theme
tokens until the Tamagui kickoff slice lands. See `frontend/CLAUDE.md` and the `architecture` skill.

## Coding Principles

These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.** Before implementing:

- State your assumptions. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**The minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No flexibility or configurability that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask: "Would a senior engineer call this overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.** When editing existing code:

- Don't improve adjacent code, comments, or formatting.
- Don't refactor what isn't broken.
- Match existing style, even if you'd do it differently.
- If you spot unrelated dead code, mention it — don't delete it.
- Remove imports, variables, and functions that *your* changes orphaned. Leave pre-existing dead code.

The test: every changed line traces directly to the request. A rename that ripples across the repo IS
surgical — every site references the renamed identifier. Cleaning up unrelated dead code you happened
to see is not.

One exception: never write "test failed on a pre-existing issue, ignoring." If you hit broken code,
assume it's part of the required work. Check with the user before working around it.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.** Turn tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass."
- "Fix the bug" → "Write a test that reproduces it, then make it pass."
- "Refactor X" → "Ensure tests pass before and after."

For multi-step tasks, state a brief plan with a verify check per step:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
```

Strong success criteria let you loop on your own; weak criteria ("make it work") force constant
clarification.

**These principles work when:** fewer needless changes in diffs, fewer rewrites from overcomplication,
and clarifying questions land before implementation rather than after mistakes.

## Change Workflow (MANDATORY)

Every change — feature, fix, or refactor — runs this pipeline, driven by Superpowers skills (install
with `/plugin install superpowers@claude-plugins-official`). Feature branches or worktrees for all
work. Never commit to main. Commit after every artifact (plan, task, completed feature), not just at
the end.

```
1. Triage          → pick or open a GitHub issue
2. Refine to Ready → refinement workflow (below). Output: a Definition-of-Ready comment on the issue
3. Branch          → AT implementation start: `superpowers:using-git-worktrees` or a feature branch
4. Plan            → `superpowers:writing-plans` (decompose into 2–5 min tasks), the first step on the branch. For multi-approach exploration, run the `parallel-plan` skill first (compare 5 approaches), then decompose the winner
5. TDD execution   → `superpowers:subagent-driven-development`, gated by `superpowers:test-driven-development`
6. Integration     → exercise the real wire on any cross-boundary change (see Testing)
7. Code review     → `superpowers:requesting-code-review` + the `architecture` skill's drift greps
8. Security gate   → the `security-review` skill when the Deep Review Trigger applies (Security section)
9. Finish branch   → `superpowers:finishing-a-development-branch`. HARD GATE: steps 6+7+8 must have run first
10. CD/Dependabot   → watched by the scheduled monitor routine; no inline watch (see Cross-cutting lessons)
11. Retro          → `retro` skill on shipped features
```

TDD is non-negotiable: `superpowers:test-driven-development` enforces RED-GREEN-REFACTOR.
Verification before completion: `superpowers:verification-before-completion` requires running build +
tests and confirming output before claiming done. Evidence before assertions.

### Refinement workflow

Refinement turns backlog issues into Definition-of-Ready specs the implementation phase runs against
on its own. The supervisor (team lead + Product Owner) batches their involvement here — this is where
human-in-the-loop (HITL) decisions live. Refinement produces a **Definition-of-Ready (DoR) comment on
the GitHub issue and nothing else: no branch, no commits, no spec file.** GitHub is the source of truth.

- `refinement-session` skill — pick up a batch, sequence by dependency, take each to Ready, confirm.
- `refinement` skill — the per-issue work, including the full Definition-of-Ready checklist. Also
  invoked on its own to refine a specific issue further.

### Implementation workflow

Once issues are Ready, implementation runs against the written specs. Working a batch of Ready issues
autonomously — picking from the project board's "Up Next" column, a PR per issue, notify-and-hold on
any surprise, autonomous merge for low-risk work and hold-open for the risk set, then an
end-of-session digest — is the `autonomous-execution` skill. Autonomy covers execution decisions
inside a Ready issue, never design forks or scope.

### How the work runs

- **Implementation** runs via `superpowers:subagent-driven-development`, which dispatches generic
  subagents. Each subagent auto-loads `/CLAUDE.md` plus the subdirectory `CLAUDE.md`
  (`backend/`, `frontend/`, `infra/`) for the files it touches. There are no named implementation
  agents.
- **Reviews are skills, not agents.** Code review = `superpowers:requesting-code-review` + the
  `architecture` skill. Security = the `security-review` skill, gated by the Deep Review Trigger.
- **Other roles map to skills + HITL.** Product-owner calls → the supervisor, via refinement. Visual
  design → `frontend-design` / `ui-ux-pro-max`. Test planning and execution → `test-driven-development`
  + `playwright-cli`.

Two working modes:

- **Assisted** (small changes, under ~5 files): dispatch one subagent to implement + test + commit,
  run `superpowers:verification-before-completion`, then `superpowers:requesting-code-review` (+ the
  `security-review` skill if the trigger applies), fix findings, merge.
- **Full pipeline** (features, multi-area changes): the full Change Workflow above.

### Pipeline rules

- **Sweep enforcement.** Every review finding carries a grep command. Fix agents fix ALL matches, not
  just the reported file.
- **Cross-stack contract alignment.** When backend and frontend run in parallel, the frontend API task
  reads the actual backend DTOs before writing interfaces.
- **Architecture drift blocks merge.** An `architecture: drift in <file>` finding blocks merge unless
  the PR documents the exception in Key Decisions.
- **Two review passes max.** After pass 2 the PR is ready to merge, once no CRITICAL or HIGH findings
  remain (those always block merge — see Security).
- **PR descriptions stay lean** — target 40–60 lines:
  - **Summary** — 2–4 bullets on what changed and net LOC.
  - **Key Decisions** — max 3–5 items. Include a decision only if a reviewer would question its
    absence, it documents a non-obvious tradeoff worth grepping for later, or it captures a HITL fork
    the user chose. Skip anything already in CLAUDE.md (reference the doc), patterns from prior PRs,
    or details the diff makes obvious.
  - **Test plan** — one line of outcomes. "415/415 vitest, 17/17 Playwright, smoke verified X+Y+Z."
  - Drop "remaining work" sections — file follow-up issues instead.

### Workflow lessons

- **Run git from the repo root.** Use absolute paths or `git -C <root>` to avoid CWD bugs after `cd`.
- **Request code review before `gh pr create`, not after.** Every PR gets a review pass first,
  including one-commit changes. "Too small to review" is never a reason to skip.
- **Path/URL/env renames need a full-repo grep.** Renaming a route, endpoint, env var, port, or config
  key means grepping the whole repo — Taskfile, workflows, docs, scripts — not just source. This is
  surgical (every site references the rename).
- **Trust the git hooks; don't re-run what they cover.** Pre-commit runs gofmt/golangci-lint on staged
  Go + tsc on staged TS. Pre-push runs full golangci-lint, go test, terraform fmt, frontend
  build+vitest+npm audit, gitleaks. After a change: `git add && git commit && git push`.
- **A fix needed to test a change end-to-end ships on the same branch**, with rationale in the commit
  body. Don't split it into a separate PR if that would block testing the change locally.
- **Subagents commit to whichever branch they're checked out on.** Every implementer prompt must have
  the subagent run `git branch --show-current` as a self-check before reporting DONE, and assert it
  matches the named branch. The lead spot-checks `git log main..HEAD` between tasks to catch drift.

## Human-in-the-Loop + Notifications

This project is built by agents with a human supervisor who is NOT watching the screen.
Pause-and-ask is always cheaper than guessing wrong.

**HITL Rule (design forks):**

- Never answer your own design questions or auto-approve decisions.
- Never assume what the human would choose.
- Present options and wait for an explicit response. Confirm alignment before the next phase.
- The refinement workflow front-loads design forks so fewer surface mid-flight; residual unclarity
  during execution is notify-and-hold, never an assumption.

**Think Before Coding (implementation ambiguity):** the everyday form — Coding Principles §1. State
assumptions, ask if material, present competing interpretations rather than picking silently.

**Notification channel:** use `PushNotification` when blocked or uncertain. Silent stalling is worse
than asking.

**Manual lead-agent takeover:** allowed only to commit work an agent finished but timed out before
committing. Never to write application code, tests, or bug fixes.

**Agent stall protocol:**

- First stall on a chunk → re-dispatch it once.
- Second stall on the same chunk → split into smaller batches (~3–5 tool uses each).
- Recurring stall after splitting → escalate via `PushNotification`. Never re-dispatch a third time.
- If a stalled agent had finished its work in the tree, the lead commits it. That is the only takeover.

## Build Commands

```bash
task build              # Build backend + frontend
task test               # Run all tests
task build:backend      # Build Go backend
task test:backend       # Run backend tests
task lint:backend       # Run golangci-lint
task build:frontend     # Build frontend
task test:frontend      # Run frontend unit tests
task deploy             # Build + terraform apply

# E2E lifecycle
task e2e:up              # Full e2e stack (LocalStack + e2e backend + e2e frontend + seeded fixtures)
task e2e:down            # Tear down the e2e stack (LocalStack stays up — shared with dev)
task e2e:seed            # Re-seed committed fixture puzzles into puzzle-pool-e2e (idempotent)
task e2e:status          # Show e2e backend status + fixture count
task e2e:genfixtures     # Regenerate committed fixture puzzles (deterministic from fixed seeds)
task e2e:up:generator    # Start the e2e generator worker (puzzle-generation-e2e → puzzle-pool-e2e)
task e2e:down:generator  # Stop the e2e generator worker
```

## Running the Dev Stack (STANDARD — always use these)

The dev stack (LocalStack + backend + generator + frontend) runs as background processes, logs
streamed to `./logs/*.log`. **Always use these tasks.** Don't launch `go run ./cmd/api` or `npm run
dev` directly — that breaks the shared logging/lifecycle contract and leaves orphan processes.

```bash
task dev:up             # Start everything (waits for readiness)
task dev:down           # Stop everything
task dev:restart        # Stop + start
task dev:status         # Show what's running (PIDs + ports)

task dev:logs            # Stream backend + generator + frontend logs (Ctrl+C to exit)
task dev:logs:backend    # Stream only backend logs
task dev:logs:generator  # Stream only generator logs
task dev:logs:frontend   # Stream only frontend logs

# Individual services
task dev:up:backend         # Start backend only (brings up LocalStack via deps)
task dev:up:generator       # Start SQS generator worker only
task dev:up:frontend        # Start frontend only
task dev:up:localstack      # Start LocalStack only
task dev:down:backend       # Stop backend
task dev:down:generator     # Stop generator
task dev:down:frontend      # Stop frontend
task dev:down:localstack    # Stop LocalStack
task dev:restart:backend    # Restart backend (picks up Go changes)
task dev:restart:generator  # Restart generator (picks up Go changes)
task dev:restart:frontend   # Restart frontend
```

**How it works:** services run detached via `nohup ... &`, stdout+stderr to `./logs/`. `dev:up` polls
each service until healthy: backend hits `/api/health`, frontend waits for `:5180` to listen,
generator waits for its "starting local SQS poller" log line. Backend/frontend identity is tracked by
port (`lsof -ti:PORT`); the generator has no port, so its PID lives in `./logs/generator.pid` (a stale
PID is detected and cleaned up). LocalStack readiness waits for both `/_localstack/health` and the
`init-aws.sh` script (queue exists + table ACTIVE). `./logs/` is gitignored. LocalStack runs in Docker
via `docker-compose.yml`; `.localstack/init-aws.sh` creates the table, queues, and seed CONFIG items.

**After Go changes:** `task dev:restart:backend` and/or `task dev:restart:generator` (neither
hot-reloads). **After frontend changes:** Vite HMR handles most; restart only after editing
`vite.config.ts` or similar.

**Taskfile shell pitfalls (read before editing Taskfile.yml):** Task runs `cmds:` in its built-in
interpreter (`mvdan.cc/sh`), not bash. It mostly matches POSIX but diverges where it bites
process-lifecycle code: `$!` after a background job returns a goroutine handle, not an OS PID;
`kill -0` against external PIDs is unreliable; `disown` is unimplemented; `set -e` around command
substitution differs from bash. When a task tracks a backgrounded process by PID (anything without a
port), wrap the block in a bash heredoc:

```yaml
cmds:
  - |
    bash <<'BASH'
    set -e
    nohup long-running-cmd > log 2>&1 </dev/null &
    echo $! > pid
    BASH
```

Port-based lifecycle (`lsof -ti:PORT`) works fine in Task's shell and is preferred whenever a port
exists.

## Testing

- Run the full test suite after making changes. After fixing a bug, verify no regressions before
  moving on.
- **Run a single test** during a TDD loop: `go test -run TestName ./internal/<pkg>/...` (backend),
  `npx vitest run <path/to/file.test.ts>` (frontend, from `frontend/`).
- All unit tests use Arrange-Act-Assert structure with explicit `// Arrange`, `// Act`, `// Assert`
  comments.
- **Unit tests on both sides of a boundary don't prove the contract.** The frontend mocks `fetch` to
  return `{ ok: true }` and backend tests call handlers directly with `httptest` — both pass while the
  wire format diverges. #179's HEAD/405 bug shipped exactly this way: the frontend probe sent HEAD,
  chi's route was GET-only, the user saw "405 Method Not Allowed" the first time the dev stack ran.
  So **any cross-boundary change** (frontend↔backend, backend↔DB, backend↔SQS, frontend↔SW,
  frontend↔CloudFront edge) **exercises the real wire before a PR.** Pick the lightest form:
  - `playwright-cli` against `task dev:up` for one-off checks — does this endpoint accept the method I
    expect, does this CSS render right. Default to this.
  - A durable Playwright e2e spec under `frontend/playwright/e2e/` (run via `task e2e:up &&
    task test:e2e`) when the contract is worth long-term regression coverage — the same boundary keeps
    getting touched, or the failure mode is subtle (the #179 pattern).

  Skip only when the diff stays within one layer (a pure-engine refactor, a visual tweak with no new
  API calls).

## Git Hooks

Pre-push (`.githooks/pre-push`) runs before every push: backend lint + tests, frontend build + tests +
npm audit, and `gitleaks` secret scan. Pre-commit (`.githooks/pre-commit`) runs gofmt/golangci-lint on
staged Go and tsc on staged TS. Install once per clone via `scripts/install-hooks.sh` (see SETUP.md).

## Dev Server Ports

| Service       | Port | Started by                 |
|---------------|------|----------------------------|
| Frontend      | 5180 | `task dev:up:frontend`     |
| Backend       | 5181 | `task dev:up:backend`      |
| E2E Backend   | 5182 | `task e2e:up:backend`      |
| E2E Frontend  | 5183 | `task e2e:up:frontend`     |
| Generator     | —    | `task dev:up:generator` (SQS consumer, no HTTP port; PID in `logs/generator.pid`) |
| E2E Generator | —    | `task e2e:up:generator` (no port; PID at `logs/e2e-generator.pid`) |
| LocalStack    | 4566 | `task dev:up:localstack`   |

The frontend binds `--host 0.0.0.0` for mobile testing over LAN; the Vite proxy forwards `/api/*` to
`localhost:5181`. All backend routes live under `/api/`.

## Roles

Role names are Title-Case in prose (`Anonymous` / `User` / `Admin`); the Clerk metadata claim values
are lowercase (`'user'` / `'admin'`).

| Role | Identity | Access |
|------|----------|--------|
| Anonymous | No account; device-linked local identity | Practice puzzles, daily challenge, local stats, own percentile |
| User | Signed in via Clerk (Google OAuth); default role | Same as Anonymous for now; reserved for later phases |
| Admin | Signed in via Clerk with `publicMetadata.role === 'admin'` | All User access + `/admin` UI and `/api/admin/*` routes |

## Database (DynamoDB)

- On-demand (pay-per-request) billing — no provisioned capacity.
- Single-table design where practical; separate tables when access patterns diverge.
- All table definitions in Terraform (`infra/modules/database/`).
- No ORM — AWS SDK for Go v2 directly.
- Local development: LocalStack.

## Key References

- **SETUP.md** — first-checkout steps (hooks, dependencies, plugins).
- **PROJECT_STRUCTURE.md** — full project tree + all API endpoints.
- **GLOSSARY.md** — Ubiquitous Language glossary. Consult before using domain terms.
- **GAME_DESIGN.md** — living game design vision.
- **BRAND_GUIDELINES.md** — design system. Required before any frontend visual work.
- **[GitHub Issues](https://github.com/lesteenman/reign-game/issues)** — todos, backlog, known issues.
- **[`Reign` project board](https://github.com/users/lesteenman/projects/1)** — status by Kanban column.
- **[Wiki](https://github.com/lesteenman/reign-game/wiki)** — roadmap history, decisions log.

**Domain conventions** live in subdirectory `CLAUDE.md` files, auto-loaded when working on files in
those directories: `backend/CLAUDE.md` (Go + logging + DynamoDB access), `frontend/CLAUDE.md`
(React + TypeScript + Tamagui + TanStack), `infra/CLAUDE.md` (Terraform, GitHub Actions, AWS). The
`architecture` skill holds the per-area design-time + review-time rules.

## Security

**Baseline gates (every change):**

1. **Secret scanning (pre-commit):** `gitleaks detect --source .` blocks the commit if secrets found.
2. **Dependency audit (CI):** `govulncheck ./...` (backend) and `npm audit --audit-level=moderate`
   (frontend) — known vulnerabilities block merge.
3. **Code-review security pass:** `superpowers:requesting-code-review` runs on every change; CRITICAL
   or HIGH findings block merge.

**Deep Review Trigger (conditional).** Run the `security-review` skill when the diff includes any of:

- `**/auth/**`, `**/middleware/**`
- `**/handler/*.go` (new or modified Lambda handlers)
- `go.mod`, `go.sum`, `package.json`, `package-lock.json` (dependency changes)
- `infra/**/*.tf` (IAM, networking, encryption)
- `docker-compose.yml`, `Dockerfile`
- `.github/workflows/**`
- Any file with `password`, `secret`, `token`, `credential`, `key` in path or content

Skip when the diff only touches service logic, models, frontend components, tests, docs, or archived
OpenSpec artifacts.

**Dependabot PRs.** GitHub Actions are pinned to commit SHAs (#113), not tags. The pin only buys
safety if a human checks each bump: confirm the new SHA matches the new `# vX.Y.Z` comment against the
action's release notes linked in the PR body. **Do not auto-merge** `area:devops`+`type:infra`
Dependabot PRs touching `.github/workflows/**` — that collapses the trust model back to "whatever the
bot says is `v6.0.3`."

## Cross-cutting lessons

- **Fetch before reporting git state.** Run `git fetch --prune` before reporting branch status,
  ahead/behind counts, or PR existence. Stale refs produce confidently wrong analysis.
- **Verify dependency versions at the registry, not from memory.** When adding or bumping any dep (npm,
  Go module, Terraform provider, GitHub Action, Docker image, Clerk/AWS SDK), the version comes from
  the live registry or current docs — never recollection. Re-verify when the slice that installs it
  starts.
- **Lockstep config: capture every consumer.** When two services share an identifier (queue URL, table
  name, env var), the acceptance criteria must enumerate every site. Define shared constants once in
  `Taskfile.yml::vars:` and reference them from each env block.
- **CD + Dependabot are watched by a scheduled routine — don't inline-watch.** The `Reign CD +
  Dependabot monitor` routine fires twice daily (09:00 + 21:00 Europe/Amsterdam = `0 7,19 * * *` UTC)
  and opens a `priority:p0`+`area:devops`+`type:bug`+`status:blocks-prod` issue on any CD failure or
  critical/high Dependabot alert. Don't run `gh run watch` after a merge — let the routine surface
  failures. CI green is not CD green (merge deploys to `acc`, the acceptance environment): TF state
  can fail on drift, IAM policies can fail to attach, frontend sync can fail post-build.
- **Trace from the call site before claiming an implementation is buggy.** Reading a suspected file in
  isolation misleads — find where it's actually called and follow the path to the rendered behavior
  first. When a user says "I thought X already works", treat it as a hint to grep callers before
  re-reading internals. Same for "pre-existing failure" claims: check CI on the same SHA before
  propagating them.
