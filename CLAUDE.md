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
- **Per-step timing on multi-call handlers, by default.** Any handler that issues more than one downstream call (DDB + Clerk, multiple DDB queries, fan-out queries, etc.) logs per-step latency on every request. Format: `<subsystem>: total_ms=N step1_ms=N step2_ms=N`. Examples shipped with R-7-02:
  - `auth: allow path=/api/admin/pool sub=user_... verify_ms=12 get_user_ms=8`
  - `admin pool: total_ms=27 configs_ms=12 combos=3 count_breakdown=[7#standard=3ms 9#double=2ms 9#standard=3ms]`

  Cost: ~5 lines per handler. Value: the next slow request shows the bottleneck in one log line — no instrumentation pass under pressure. Treat this as default instrumentation, not a diagnostic afterthought. R-7-02 paid for the "diagnostic afterthought" model when an 8 s pool load forced a full instrumentation pass mid-debug; with the timing logs in place from the start, the same diagnosis would have been a single log line.

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

Role names are Title-Case in prose (`Anonymous` / `User` / `Admin`); the Clerk metadata claim values are lowercase (`'user'` / `'admin'`).

| Role | Identity | Access |
|------|----------|--------|
| Anonymous | No account; device-linked local identity | Practice puzzles, daily challenge, local stats, see own percentile |
| User | Signed-in via Clerk (Google OAuth); default role with no `publicMetadata.role` set or `'user'` | Same as Anonymous for now; reserved for later phases (leaderboard identity, stats sync, premium flip) |
| Admin | Signed-in via Clerk with `publicMetadata.role === 'admin'` (assigned manually in the Clerk dashboard) | All User access + `/admin` UI and `/api/admin/*` routes (curation, puzzle management, generation tooling) |

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

**Slice ID scheme.** New slices use `R-<phase>-<slice>` where `<phase>` is the integer phase number (no decimals, no letters) and `<slice>` is either the literal `exploration` or a strictly increasing 2-digit zero-padded number (`01`, `02`, …, `99`). Examples: `R-7-02`, `R-7-exploration`, `R-12-01`. Only the current phase is numbered; everything else lives on the ROADMAP backlog without an ID until we commit to starting it. Already-shipped slices keep their historical IDs (e.g., `R-067a`, `R-08C`, `R-081`) — those stay because they're baked into commit messages, PR titles, and archived OpenSpec artifacts that aren't worth churning. The lessons below reference historical IDs for that reason.

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
11. **Sub-agents must run lint and fmt before committing.** Backend agents: run `golangci-lint run` (or `go vet ./...` if unavailable). Devops agents: run `terraform fmt -recursive -check`. Frontend agents already run `tsc -b`. Explicit instructions in agent prompts are required — sub-agents don't read pre-push hooks. **TDD enforcement clause:** verify in the agent's output that test-file commits exist BEFORE production-file commits. If a multi-file production-code diff lands without a corresponding test-file edit, the agent skipped TDD — review the production code carefully, ideally by writing the tests yourself. Phase 7 R-081 paid for this: backend-dev produced 228 lines of repository code with one trivial test fixture edit, then stalled. The follow-up tests I wrote manually caught a real bug — `fmt.Sscanf("%[^#]#%s", ...)` silently fails because Go's scan family doesn't implement the C-style `%[^...]` character-class verb. Without the manual test pass, the bug ships.
12. **Fetch before reporting git state:** Before reporting branch status, upstream existence, "PR exists?", or ahead/behind counts, run `git fetch --prune` first. Stale refs (especially after a branch is deleted post-merge) produce confidently wrong analysis and push scoping decisions down the wrong path. Treat local refs as cache that needs invalidating, not source of truth.
13. **Run review-local before `gh pr create`, not after. No exceptions for small PRs.** Every PR, including 1-commit surgical changes, gets the 4-agent review loop first. If the diff is truly trivial, the review costs 60 seconds; if it isn't, the review was needed. Phase 4.5 caught a MAJOR Taskfile bug this way. R-067a (PR #39) skipped the review because "the change felt small" — it happened to be clean, but that was luck, not process. "It's too small to review" is never a valid reason to skip.
14. **Path/URL/env renames need a full-repo grep.** When renaming a route, endpoint, environment variable, port, or config key, grep the *entire* repo — `Taskfile.yml`, `.github/workflows/**`, `docker-compose.yml`, all `*.md` docs, `CLAUDE.md`, `PROJECT_STRUCTURE.md`, `ROADMAP.md`, shell scripts — not just the obvious source files. Dev tooling and docs silently drift out of sync otherwise. Phase 4.5's `/health` → `/api/health` migration missed the Taskfile readiness probe on the first pass; only review-local's sweep caught it. Same rule applies to **archive moves**: when `git mv`'ing an OpenSpec `changes/X/` directory to `archive/X/`, sweep every `.md`, `.go`, `.ts`, `.tsx`, `.tf`, `.yml`, and shell script for the old path. Source-code comments are easy to miss — Phase 7's archive of phase-6-admin-auth needed a follow-up edit to `backend/internal/generator/probe_test.go`'s file header comment that cited the old path; phase-4/5 archive needed a `ROADMAP.md` ref fix. Each archive PR has historically required at least one mid-PR cross-doc fixup; the grep eliminates them.
15. **Trust the git hooks — do not re-run what they cover.** `.githooks/pre-commit` runs `gofmt`/`golangci-lint` (on staged Go) and `tsc -b` (on staged TS). `.githooks/pre-push` runs full `golangci-lint`, `go test ./...`, `terraform fmt -recursive -check`, `npm run build` (includes tsc), `npx vitest run`, `npm audit --audit-level=moderate`, and `gitleaks detect --source .`. After writing a change, go straight to `git add && git commit && git push` — if hooks fail, fix the specific issue they reported. Running these tools manually before the hooks duplicates work, clutters terminal output, and wastes wall-clock time. It is appropriate to run them directly during TDD iterations or to debug a specific failure; it is not appropriate as a routine pre-commit/pre-push ritual.
16. **Persisted data shapes live in `storage/` (frontend) or `repository/` (backend), not in the consumer.** If a type is going to be saved to IndexedDB, DynamoDB, or any store, define it *once* in the storage/repository module and import it from every consumer (hooks, services, handlers). Do not redeclare a shape like `History` in a hook and `GameHistory` in storage — they will drift. Phase 4.6 had to unify `History` / `GameHistory` mid-review for exactly this reason.
17. **Slice completion includes flipping the `tasks.md` status row to `[x]`.** OpenSpec's `tasks.md` status table is the single source of truth for slice state. A slice is not done until the code ships AND the row is updated in the same branch. Phase 5's pre-main cleanup had to refresh 7 of 11 rows that stayed `[ ]` for weeks after their deliverables had shipped — the status column became useless for planning. Update the row as a required artifact in the slice's PR, not as a post-hoc sweep. (Parallel slices off the same `tasks.md` will produce a merge conflict on the second-to-merge branch — each side flipped its own row to `[x]`. Resolution is mechanical: keep both `[x]` flips, no semantic content change. Phase 6 hit this on R-089 / R-08A / R-08B / integration; don't be surprised by it and don't accidentally drop someone else's flip when resolving.)
18. **Before opening an OpenSpec change, grep ROADMAP for ID collisions.** New slice IDs often collide with pre-declared IDs in later phase blocks. When creating a change that claims IDs like `R-062..R-06D`, first `grep -n "R-06[0-9A-F]" ROADMAP.md` — if any of those IDs are already reserved for a future phase, renumber one side before the slice lands. Phase 5 shipped R-062..R-06D while ROADMAP Phase 6/7/8 still pre-declared R-063..R-068; the collision went untreated through 14 slices and KI-007's "Related" column ended up citing a nonexistent ID.
19. **Never guess dependency versions from memory — check the registry.** When adding or bumping an npm / Go / Maven / PyPI dependency, the version number must come from the live registry (`npm view <pkg> version`, `go list -m -versions <module>`, etc.) or the package's current docs page — never from recollection. Training data goes stale; packages are yanked, re-numbered, or change major versions without us knowing. Spec docs and code that name a specific version (e.g. `@clerk/clerk-react@x.y.z`, `github.com/clerk/clerk-sdk-go/v2`) should be verified at the moment they're written, and re-verified when the slice that installs them starts. Applies to both implementation and design-phase artifacts that commit to a particular SDK surface.
20. **Long agent prompts stall — plan checkpoints, take manual control after two stalls.** Agent prompts requiring more than ~20 tool uses or multi-file coordinated output are at high risk of stream-idle timeout. Plan from the start to split the work into discrete chunks (design phase → test scaffold → implementation → sweep), each independently committable. If a single chunk stalls twice, take manual control rather than attempt a third resume — manual implementation often ships faster AND catches bugs the agent missed. Phase 7 R-081 saw this directly: design-flow timed out after 25 min on PR #70 (recovered cleanly via `SendMessage`), backend-dev timed out twice on PR #71 — the second stall left 228 lines of untested repository code that I had to test by hand, which surfaced a `fmt.Sscanf` bug that would otherwise have shipped silently. Manual takeover finished the slice in the same conversation that the second resume would have spent stalling.
21. **ROADMAP.md tracks codebase work, not user-side dashboard tasks.** Code, infra, docs, scripts, tests — all in scope. Clicking through Clerk dashboards, AWS console one-offs, manual approvals, third-party-UI configurations — out of scope. The action being "click in a third-party UI" is a strong signal it's not a slice. Where the operational detail still matters, capture it as a one-line callout in the relevant runbook (e.g. `docs/runbooks/admin-auth-setup.md`), not as a tracked ROADMAP slice with an ID and a checkbox. Phase 6's R-08D ("custom domain + production Clerk tenant swap") sat in ROADMAP for ~5 weeks before being dropped because "I'll do that from my side in Clerk when I get to it" isn't a slice — the *infra side* (custom-domain provisioning) survived as a backlog entry, but the dashboard rotation didn't.
22. **Docker images in dev tooling pin to a specific stable tag, never `:latest`.** Auto-update on `docker compose pull` is silent and untraceable; treat `:latest` like `npm install <pkg>` without a lockfile or `go get` without a version (lesson 19) — same risk, same mitigation. When pinning, document the bump deliberately in the file (one-line comment naming the version + reason). The R-7-02 cycle paid for this when `localstack/localstack:latest` pulled a `2026.3.1.dev4973` build with broken SQS `SendMessage` and an empty exception body in the LocalStack logs. ~1 hour to diagnose because the symptom (replenish 500s) didn't point at the image tag. Pinning to `localstack/localstack:4.14.0` resolved it instantly.
23. **Standalone reproducer first when perf-bisecting an SDK or framework issue.** When a symptom could be caused by any of N layers (DNS, connection pool, retry backoff, SDK init, library interaction), each guess at the wrong layer costs at least one restart-and-measure cycle (~30 s) plus the cognitive overhead of building a wrong mental model. A 30-line standalone Go (or equivalent) program that varies one suspect at a time produces concrete numbers in one run. The R-7-02 perf hunt spent ~30 min guessing at IPv6 fallback / IMDS retry / connection-pool corruption / stale DNS before a tiny standalone Go program that tested 4 HTTP-client strategies side-by-side identified `clerk.SetKey` mutating `http.DefaultTransport` in 5 minutes. Files live at `/tmp` during the investigation; delete after, or commit if they become a regression test. Rule: when bisecting, write the probe FIRST, not after the third guess.
24. **Go SDKs that mutate `http.DefaultTransport` contaminate every other SDK in the same process.** Clerk's Go SDK v2's `clerk.SetKey()` wraps `http.DefaultClient` (or installs middleware along that path); the AWS Go SDK inheriting the default pays a multi-second cost on its first call as a result. Insulate each SDK at construction time by giving it a dedicated `http.Client` backed by `http.DefaultTransport.(*http.Transport).Clone()` — the clone snapshots the underlying TCP transport into an independent state that is detached from subsequent global mutations. Documented in `backend/cmd/api/main.go::loadAWSConfig` for the AWS + Clerk pairing. Measured cost: ~1.8 s vs ~9 ms on the first DDB Query when both SDKs share the default transport. When integrating any new third-party Go SDK, audit whether it mutates the default transport; if yes, isolate every other SDK explicitly.
25. **Non-slice perf fixes that block local testability of the slice attach to the slice's PR with explicit commit-body justification.** When the perf fix isn't part of the slice's stated scope but is needed to make the slice testable end-to-end locally, the pragmatic move is to commit it on the same branch with the rationale stated explicitly in the commit body ("R-7-02 was effectively un-playtest-able locally without this fix"). The reviewer sees the slice + perf fixes as a unit and decides whether the scope creep is acceptable. Do NOT split into a separate PR if it would block testing the slice — that creates merge-order pain. Do split if the perf fix can wait until after the slice merges. R-7-02 ended up with 4 backend perf commits on a frontend-only slice (auth GetUser cache, `localhost`→`127.0.0.1`, `http.DefaultTransport.Clone` + startup warm-up, LocalStack image pin); each commit body justified inclusion explicitly and all merged cleanly.
26. **Investigate latest versions for ALL external dependencies — generalizes lesson 19 across categories.** Lesson 19 covers npm / Go / Maven / PyPI registry-pinned packages. The same rule applies to: Docker images (lesson 22 — registry on Docker Hub / ECR), GitHub Actions (`@v4` style — check `marketplace.github.com` / repo's release tags), Terraform providers and modules (registry.terraform.io), Homebrew formulae, system packages, Clerk / AWS / any SaaS SDK with versioned API surfaces, and IaC base images. Whatever the dependency, the version number comes from the live source of truth at the moment of writing — never from memory or training data. Training data goes stale; packages are yanked, deprecated, renumbered, or change major versions without us knowing. Re-verify when the slice that installs the dependency starts (in addition to design-phase verification). Applies to spec docs that commit to a particular SDK / image / action surface.

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
