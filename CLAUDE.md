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

The frontend follows the Bulletproof React feature-folder layout and styles with Tamagui primitives +
theme tokens. See `frontend/CLAUDE.md` and the `architecture` skill.

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
surgical — every site references the renamed identifier, so grep the whole repo (Taskfile, workflows,
docs, scripts), not just source. Cleaning up unrelated dead code you happened to see is not.

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

## Change Workflow

Every change — feature, fix, or refactor — moves through two phases, with the supervisor's involvement
batched into the first:

- **Refinement** (batched, human-in-the-loop) turns backlog issues into Ready specs. The supervisor
  (team lead + Product Owner) makes the design calls here. Output: a Definition-of-Ready (DoR) comment
  on the GitHub issue — no branch, no commits. GitHub is the source of truth.
- **Implementation** runs against the Ready specs, one PR per issue, gated end to end. Autonomy covers
  execution decisions inside a Ready issue, never design forks or scope.

Feature branches or worktrees for all work — never commit to main. Commit after every artifact (plan,
task, completed feature). Reference the issue in the commit message and PR description (e.g. `(#326)` in
the subject, `Closes #326` in the body) — that is where provenance lives; never put issue/PR references
in inline code comments. Run git from the repo root (`git -C <root>`); fetch before reporting branch
state. A fix needed to test the change end to end ships on the same branch with rationale in the commit
body, not a separate PR.

Pick the flow by size:

- **Assisted** (small changes, under ~5 files): one subagent implements + tests + commits, then
  `superpowers:verification-before-completion`, then `superpowers:requesting-code-review` (+ the
  `security-review` skill if the trigger applies), fix findings, merge.
- **Full pipeline** (features, multi-area changes): the refinement → implementation steps below.

### Refinement workflow

1. **Triage** — pick or open a GitHub issue.
2. **Refine to Ready** — the `preparing-ready-issues` skill (batch pickup, dependency sequencing,
   one-issue-at-a-time driving) drives the `refining-an-issue` skill per issue (the design conversation
   + the full DoR checklist). `refining-an-issue` is also invoked alone to refine a single issue further.

### Implementation workflow

Once issues are Ready, working a batch of them autonomously — a PR per issue, notify-and-hold on
surprises, autonomous merge for low-risk work, hold-open for the risk set, end-of-session digest — is
the `executing-ready-issues` skill. Each issue runs the per-PR pipeline below:

3. **Branch** — `superpowers:using-git-worktrees` or a feature branch, at implementation start.
4. **Plan** — `superpowers:writing-plans` (decompose into 2–5 min tasks), the first step on the branch.
   For multi-approach exploration, run `parallel-plan` first (compare 5 approaches), then decompose the
   winner.
5. **TDD execution** — `superpowers:subagent-driven-development`, gated by
   `superpowers:test-driven-development` (RED-GREEN-REFACTOR, non-negotiable). It dispatches generic
   subagents that auto-load `/CLAUDE.md` plus the subdirectory `CLAUDE.md` for the files they touch.
   Subagents commit to whichever branch they're checked out on, so each one runs
   `git branch --show-current` and asserts it matches before reporting DONE.
6. **Integration verification** — exercise the real wire on any cross-boundary change (frontend↔backend,
   backend↔DB, backend↔SQS, frontend↔SW, frontend↔CloudFront edge). Unit tests on both sides don't
   prove the contract: a mocked `fetch` and a direct `httptest` handler call both pass while the wire
   format diverges (#179's HEAD/405 bug shipped exactly that way). Pick the lightest form —
   `playwright-cli` against `task dev:up` for a one-off check, or a durable Playwright e2e spec under
   `frontend/playwright/e2e/` when the contract is worth long-term regression coverage. Skip only when
   the diff stays within one layer.
7. **Code review** — `superpowers:requesting-code-review` + the `architecture` skill's drift greps.
   Run it before `gh pr create`, not after — every PR, including one-commit changes.
8. **Security gate** — the `security-review` skill when the Deep Review Trigger applies (see Security).
9. **Finish branch** — `superpowers:finishing-a-development-branch`. HARD GATE: steps 6+7+8 must have
   run on the branch diff first. Then `superpowers:verification-before-completion` — run build + tests
   and confirm output before claiming done. Evidence before assertions.
10. **CD** — merge deploys to `acc` (the acceptance environment). CD/deploy failures surface through the
    post-deploy verification gate (GitHub Environments + Deployments, #241) — don't inline-watch with
    `gh run watch`. CI green is not CD green: TF state can fail on drift, IAM can fail to attach, frontend
    sync can fail post-build. The scheduled routine now watches only Dependabot (critical/high alerts →
    push notification, not a tracked issue — Dependabot's own PR + Security tab hold the record).
11. **Retro** — the `retro` skill on shipped features.

### Pipeline rules

- **Sweep enforcement.** Every review finding carries a grep command. Fix agents fix ALL matches, not
  just the reported file.
- **Cross-stack contract alignment.** When backend and frontend run in parallel, the frontend API task
  reads the actual backend DTOs before writing interfaces.
- **Architecture drift blocks merge.** An `architecture: drift in <file>` finding blocks merge unless
  the PR documents the exception in Key Decisions.
- **Two review passes max.** After pass 2 the PR is ready to merge, once no CRITICAL or HIGH findings
  remain (those always block merge — see Security).

### PR description

Keep it lean — target 40–60 lines:

- **Summary** — 2–4 bullets on what changed and net LOC.
- **Key Decisions** — max 3–5 items. Include a decision only if a reviewer would question its absence,
  it documents a non-obvious tradeoff worth grepping for later, or it captures a HITL fork the user
  chose. Skip anything already in CLAUDE.md (reference the doc), patterns from prior PRs, or details the
  diff makes obvious.
- **Test plan** — one line of outcomes. "415/415 vitest, 17/17 Playwright, smoke verified X+Y+Z."
- Drop "remaining work" sections — file follow-up issues instead.

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

**Notification channel:** use the `PushNotification` tool when blocked or uncertain. Silent stalling is
worse than asking.

**Manual lead-agent takeover:** allowed only to commit work an agent finished but timed out before
committing. Never to write application code, tests, or bug fixes.

**Agent stall protocol:**

- First stall on a chunk → re-dispatch it once.
- Second stall on the same chunk → split into smaller batches (~3–5 tool uses each).
- Recurring stall after splitting → escalate via `PushNotification`. Never re-dispatch a third time.
- If a stalled agent had finished its work in the tree, the lead commits it. That is the only takeover.

## Commands

### Build & test

```bash
task build              # Build backend + frontend
task test               # Run all tests
task build:backend      # Build Go backend
task test:backend       # Run backend tests
task lint:backend       # Run golangci-lint
task build:frontend     # Build frontend
task test:frontend      # Run frontend unit tests

# E2E lifecycle
task e2e:up              # Full e2e stack (LocalStack + e2e backend + e2e frontend + seeded fixtures)
task e2e:down            # Tear down the e2e stack (LocalStack stays up — shared with dev)
task e2e:seed            # Re-seed committed fixture puzzles into puzzle-pool-e2e (idempotent)
task e2e:status          # Show e2e backend status + fixture count
task e2e:genfixtures     # Regenerate committed fixture puzzles (deterministic from fixed seeds)
task e2e:up:generator    # Start the e2e generator worker (puzzle-generation-e2e → puzzle-pool-e2e)
task e2e:down:generator  # Stop the e2e generator worker
```

Running a single test is area-specific — see `backend/CLAUDE.md` / `frontend/CLAUDE.md`.

### Dev stack

The dev stack (LocalStack + backend + generator + frontend) runs as background processes, logs streamed
to `./logs/*.log`. **Always use these tasks** — never `go run ./cmd/api` or `npm run dev` directly, which
breaks the shared logging/lifecycle contract and leaves orphan processes.

```bash
task dev:up             # Start everything (waits for readiness)
task dev:down           # Stop everything
task dev:restart        # Stop + start
task dev:status         # Show what's running (PIDs + ports)
task dev:logs           # Stream backend + generator + frontend logs (Ctrl+C to exit)
#   …:logs:backend / :logs:generator / :logs:frontend stream one service
#   …:up:<svc> / :down:<svc> / :restart:<svc> control one service
#   <svc> ∈ backend | generator | frontend | localstack
```

After Go changes: `task dev:restart:backend` and/or `:generator` (neither hot-reloads). After frontend
changes: Vite HMR handles most; restart only after editing `vite.config.ts` or similar.

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
`localhost:5181`. All backend routes live under `/api/`. How the lifecycle tasks work internally is in
`scripts/CLAUDE.md`; the shell pitfalls to know before editing them are in a comment block at the head
of `Taskfile.yml`.

## Git Hooks

Pre-push (`.githooks/pre-push`) runs before every push: backend lint + tests, frontend build + tests +
npm audit, and `gitleaks` secret scan. Pre-commit (`.githooks/pre-commit`) runs gofmt/golangci-lint on
staged Go and tsc on staged TS. Install once per clone via `scripts/install-hooks.sh` (see SETUP.md).
Trust the hooks — don't re-run what they cover. After a change: `git add && git commit && git push`.

## Roles

Role names are Title-Case in prose (`Anonymous` / `User` / `Admin`); the Clerk metadata claim values
are lowercase (`'user'` / `'admin'`).

| Role | Identity | Access |
|------|----------|--------|
| Anonymous | No account; device-linked local identity | Practice puzzles, daily challenge, local stats, own percentile |
| User | Signed in via Clerk (Google OAuth); default role | Same as Anonymous for now; reserved for later phases |
| Admin | Signed in via Clerk with `publicMetadata.role === 'admin'` | All User access + `/admin` UI and `/api/admin/*` routes |

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
those directories: `backend/CLAUDE.md` (Go + logging + DynamoDB access + testing), `frontend/CLAUDE.md`
(React + TypeScript + Tamagui + TanStack + testing), `infra/CLAUDE.md` (Terraform, GitHub Actions, AWS),
`scripts/CLAUDE.md` (dev-stack lifecycle). The `architecture` skill holds the per-area design-time +
review-time rules.

## Security

**Baseline gates (every change):**

1. **Secret scanning (pre-commit):** `gitleaks detect --source .` blocks the commit if secrets found.
2. **Dependency audit (CI):** `govulncheck ./...` (backend) and `npm audit --audit-level=moderate`
   (frontend) — known vulnerabilities block merge. When adding or bumping any dependency, take the
   version from the live registry or current docs, never from memory.
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
