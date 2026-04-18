# Reign — Roadmap & Known Issues

Living document tracking planned work and known problems. Each item has an ID for reference in commits and discussions.

Status key: `[ ]` todo, `[~]` in progress, `[x]` done, `[!]` blocked

---

## Phase 0: Skeleton + Deploy Pipeline

Goal: Empty app live on AWS, CI/CD working. No game logic yet.

- [x] **R-001** — Initialize monorepo structure (frontend/, backend/, infra/, design/)
- [x] **R-002** — Set up Go module in backend/ with basic Lambda handler (health check)
- [x] **R-003** — Set up React + TypeScript + Vite in frontend/ (minimal scaffold)
- [x] **R-004** — Terraform: S3, CloudFront, API Gateway, Lambda (no DynamoDB yet)
- [x] **R-005** — GitHub Actions CI: lint + test on PR for both frontend and backend
- [x] **R-006** — GitHub Actions CD: deploy to AWS on merge to main
- [x] **R-007** — Basic dev workflow: local frontend dev server, Go backend runner
- [x] **R-008** — Brand guidelines generation (ui-ux-pro-max skill)

Note: Terraform and GitHub Actions contain no AWS account, role, or domain specifics. All injected via GitHub configuration (secrets/variables).

## Phase 1: First Playable (5x5 Standard)

Goal: Play randomly generated 5x5 Standard Mode puzzles in the browser, hosted on AWS.

- [x] **R-010** — Puzzle data model: grid, regions, solution representation (Go)
- [x] **R-011** — Puzzle solver: constraint-based deduction, verify uniqueness (Go, 5x5 Standard)
- [x] **R-012** — Puzzle generator: produce valid 5x5 Standard Mode puzzles (Go)
- [x] **R-013** — Generate endpoint: stateless, returns a fresh puzzle on each call (no DB)
- [x] **R-014** — Theme architecture: ThemeContext, theme data structure, component token consumption
- [x] **R-015** — Tactile default theme: piece icons, color palette, grid styling, animations
- [x] **R-016** — Interactive grid component: render regions, place/remove markers, exclusion marks, highlight conflicts (theme-aware)
- [x] **R-017** — Solution validation in TypeScript (constraint check, no solver)
- [x] **R-018** — Game state in IndexedDB: placements, exclusion marks, timer, completion status. Persist every move.
- [x] **R-019** — Game flow UI: puzzle loading, timer, solve flow, completion screen
- [x] **R-01A** — PWA basics: service worker (app shell caching), manifest, install prompt

## Phase 2: All Grid Sizes + Double Queens

Goal: 7x7 and 9x9 puzzles playable in both Standard and Double Queens modes.

- [x] **R-020** — Extend generator + solver for 7x7 and 9x9 grids
- [x] **R-022** — UI: grid size selector
- [x] **R-030** — Extend solver for Double Queens constraints (2 per row/column/region)
- [x] **R-031** — Extend generator for Double Queens puzzles
- [x] **R-033** — UI: mode toggle (Standard / Double Queens)

## Phase 3: Puzzle Pool + Generation Pipeline

Goal: Pre-generate puzzles into a pool, serve from the pool instead of generating on the fly. Measure generation performance across engines and grid sizes.

- [x] **R-040** — DynamoDB `puzzle-pool` table (PK=size#mode, SK=puzzleId, status/verdict/metadata)
- [x] **R-041** — Terraform: DynamoDB table, SQS queue + DLQ, Generator Lambda (15min timeout)
- [x] **R-042** — SQS-based generation: API Lambda publishes messages, Generator Lambda consumes and writes puzzles to DB
- [x] **R-043** — `POST /admin/replenish` endpoint: check pool levels, publish SQS messages for gaps (pool size = 3 per size+mode)
- [x] **R-044** — `GET /puzzles/next?size=N&mode=M` endpoint: serve next ready puzzle with generation metadata, mark as served
- [x] **R-045** — LocalStack SQS setup: local dev parity with two processes (API server + SQS consumer)
- [x] **R-046** — Frontend: replace `generatePuzzle()` with `/puzzles/next`, remove advanced options from PuzzleSelector
- [x] **R-047** — Frontend: show generation metadata (pipeline, solver, duration) subtly on puzzle page
- [x] **R-048** — Frontend: "no puzzles available" state with retry button when pool is empty

## Phase 4: Admin Pool Management

Goal: Admin UI to view pool status per size+mode and tune generation settings.

- [x] **R-050** — DynamoDB CONFIG items in `puzzle-pool` table (PK=CONFIG, SK={size}#{mode})
- [x] **R-051** — `GET /admin/pool` endpoint: merged config + ready counts per combo
- [x] **R-052** — `PUT /admin/config/{size}/{mode}` endpoint: update combo config
- [x] **R-053** — `POST /admin/config` endpoint: create new combo
- [x] **R-054** — Refactor replenish: dynamic config-driven combo discovery + per-combo threshold/params
- [x] **R-055** — Replenish filter: optional `?size=X&mode=Y` for per-combo replenish
- [x] **R-056** — Frontend: `/admin` page — pool table, config editing (modal), replenish controls
- [x] **R-057** — Frontend: admin link in PageShell header
- [x] **R-058** — LocalStack seed: initial CONFIG items for local dev

## Phase 4.5: API Prefix

Goal: Prefix all backend API routes with `/api` to cleanly separate API traffic from frontend routes. Eliminates proxy/CloudFront path conflicts (e.g., `/admin` page vs `/admin/*` API).

- [x] **R-059** — Backend: mount all routes under `/api` prefix (`/api/puzzles/*`, `/api/admin/*`, `/api/health`)
- [x] **R-05A** — Frontend: update API base path, Vite proxy, and service calls to use `/api` prefix
- [x] **R-05B** — Infra: replace per-path CloudFront behaviors + API Gateway resources with single `/api/*` pattern
- [x] **R-05C** — Verify production request flow end-to-end after migration

## Phase 4.6: Undo / Redo

Goal: Let players step backwards and forwards through their move history on a puzzle.

- [ ] **R-060** — Frontend: undo/redo buttons in the puzzle UI. Stack-based history of placements/exclusions/clears; redo stack cleared on new action. Keyboard shortcuts (Ctrl/Cmd+Z, Ctrl/Cmd+Shift+Z). Persists with the rest of the game state in IndexedDB so undo survives reload. No server changes.

## Phase 5: Generator Rework

Goal: Rework the puzzle generator based on new designs (to be detailed via design-flow). Expected motivations: faster generation (especially Double Queens — see KI-007), higher-quality region shapes, more reliable deducibility guarantees. Scope and task breakdown come from the design session.

- [ ] **R-061** — Design-flow: capture the new generator design, decide algorithm(s), and split into implementation tasks (R-062, R-069, R-06A+ as needed)

## Phase 6: Verdict System

Goal: Rate puzzles after playing them — upvote, downvote, or skip.

- [ ] **R-063** — `PUT /puzzles/:id/verdict` endpoint: upvote/downvote/skip
- [ ] **R-064** — Frontend: verdict buttons on puzzle completion/skip

## Phase 7: Puzzle Replay

Goal: Admin can browse played puzzles and replay them to review quality.

- [ ] **R-065** — `GET /puzzles/:id` endpoint: load any puzzle by ID for replay
- [ ] **R-066** — Frontend: played puzzle list in admin UI, replay by ID

## Phase 8: Puzzle Analysis Agent

Goal: Automated analysis of played puzzles — generation performance, verdict patterns, engine comparison.

- [ ] **R-067** — Analysis agent: dedicated agent for querying and interpreting puzzle generation data
- [ ] **R-068** — Analysis endpoint(s) as needed by the agent

## Phase 9: Difficulty Rating

Goal: Difficulty rating for all grid sizes and modes, with user-facing difficulty selector.

- [ ] **R-021** — Difficulty rating algorithm (region shape complexity, deduction chain depth)
- [ ] **R-032** — Update difficulty rating for Double Queens
- [ ] **R-034** — UI: difficulty selector (Easy / Medium / Hard)

## Phase 10+: Future (scoped when we get there)

Candidate items — not yet committed or ordered:

- [ ] **R-070** — Separate production puzzle table: approved puzzles copied from pool for numbered/daily serving
- [ ] **R-071** — Daily puzzle scheduling: assign puzzles to dates, serve by date + mode + difficulty
- [ ] **R-072** — Daily challenge flow: fetch puzzle, timer, submit completion, show percentile
- [ ] **R-073** — Anonymous completion submission + percentile calculation (stateless for free players)
- [ ] **R-074** — Leaderboards: premium members visible, daily and overall
- [ ] **R-075** — Auth provider setup (not Cognito) — Google + Apple OAuth
- [ ] **R-076** — One-time premium purchase flow
- [ ] **R-077** — Premium: full puzzle archive access, detailed stats, cross-device sync
- [ ] **R-078** — Premium themes: Queens Classic (free), Gems, Garden, Neon, Cosmos
- [ ] **R-079** — Performance audit: Core Web Vitals, Lambda cold starts, DynamoDB latency
- [ ] **R-07A** — Monitoring: CloudWatch dashboards, error alerting
- [ ] **R-07B** — Dev/prod environment split (Terraform workspaces)
- [ ] **R-07C** — Accessibility audit: WCAG 2.1 AA compliance
- [ ] **R-07D** — Colorblind-friendly region palettes
- [ ] **R-07E** — Rate limiting and abuse prevention on completion API
- [ ] **R-07F** — Landing page / marketing site
- [ ] **R-07G** — Open source preparation: LICENSE files, CONTRIBUTING.md, README for generator
- [ ] **R-07H** — Frontend: curation UI — visual solver, "pick best of N" comparison mode
- [ ] **R-07I** — Frontend: offline practice from curated pool (IndexedDB caching)
- [ ] **R-07J** — Offline detection: graceful degradation

---

## Known Issues

| ID | Severity | Description | Related |
|----|----------|-------------|---------|
| KI-001 | Medium | GitHub Actions use major version tags, not SHA pins. Pin before handling sensitive data (auth, payments). | R-005, R-006 |
| KI-002 | Low | CloudFront missing security response headers (HSTS, X-Content-Type-Options, CSP). Add before production. | R-004 |
| KI-003 | Low | S3 bucket has no explicit server-side encryption config (AWS defaults to SSE-S3, but should be explicit). | R-004 |
| KI-004 | Low | PWA service worker disabled — vite-plugin-pwa lacks Vite 8 support. Track [vite-pwa/vite-plugin-pwa#923](https://github.com/vite-pwa/vite-plugin-pwa/issues/923). Manifest + icons in place; re-add plugin when compatible. | R-01A |
| KI-005 | ~~Low~~ Fixed | ~~Timer does not start immediately when opening a new puzzle.~~ Fixed: immediate tick on timer start. | R-019 |
| KI-006 | ~~Medium~~ Fixed | ~~Every CD deploy updates the Lambda function even when there are no backend changes.~~ Fixed: reproducible zip (touch + zip -X) means identical source produces identical hash. | R-006 |
| KI-007 | High | Double Queens puzzle generation too slow (12+ min for 7x7) with deducibility check. Disabled in replenish and UI. Needs generator algorithm optimization before re-enabling. | R-030, R-031 |
| KI-008 | ~~Medium~~ Fixed | ~~"Play Again" and "Retry" buttons don't work.~~ Fixed: buttons now trigger re-fetch via state reset instead of URL navigation. | R-044 |
| KI-009 | **Critical** (pre-production) | `/admin/*` routes (`GET /admin/pool`, `PUT /admin/config/{size}/{mode}`, `POST /admin/config`, `POST /admin/replenish`) have no authentication in the backend, API Gateway (`authorization = "NONE"`), or CloudFront. Any anonymous caller can read pool state, mutate generation configs, and trigger replenish. Must be gated before exposing the admin UI to any non-trusted network. Pairs with auth rollout in R-075. Interim mitigation landed in this cycle: threshold capped at 50 to blunt SQS amplification; CloudFront now forwards `Authorization` so the future token flow is ready. | R-051, R-052, R-053, R-054, R-075 |
| KI-010 | Medium | `GET /admin/pool` (`backend/internal/handler/admin_pool.go`) does 1 Query for configs plus 1 per-combo `CountReady` Query serially — N+1 on Lambda cold-start path. Fix with `errgroup` (bounded) or a single pre-aggregated count attribute. | R-051 |
| KI-011 | Medium | `repository.CountReady` and `repository.NextReady` (`backend/internal/repository/puzzle.go`) use a `FilterExpression` on `status = "ready"`, which forces DynamoDB to read the full partition (including historical served/solved/skipped puzzles) before filtering. Cost + latency grow with lifetime pool volume, not ready inventory. Fix with a sparse GSI keyed on `{size}#{mode}#ready` that only ready items populate (writers add attributes on put, `MarkServed` / `UpdateStatus` remove them). | R-040, R-044 |
| KI-012 | Medium | `ReplenishHandler` (`backend/internal/handler/replenish.go`) publishes one SQS message per unit of `threshold - count` in a serial loop. For 5 combos at threshold 10 with an empty pool that is 50 sequential `SendMessage` calls inside an HTTP handler. Switch to `SendMessageBatch` (up to 10/call) and/or parallelize across combos. Add a `PublishBatch` method to `queue.Publisher`. | R-042, R-043, R-054 |
| KI-013 | Low | The config payload shape is re-declared four times: `repository.ConfigRecord`, `handler.configRequest`, `handler.configResponse`, and the hand-rolled `handler.buildConfigResponseMap`. A new field must be added in four places. Unify by adding JSON tags to `ConfigRecord` and encoding it directly, or by extracting a shared DTO. | R-050, R-052 |
| KI-014 | Low | `frontend/src/services/api.ts` has `apiFetch`, `apiPut`, `apiPost` as three near-identical functions (~25 lines each) differing only by HTTP method and whether a body is sent. Empty-body response handling is also inconsistent across them. Collapse into one `apiRequest(method, path, opts)` with thin wrappers. | R-046, R-056 |
| KI-015 | Low | `AdminPage.tsx` `ConfigForm` takes 9 primitive props, four of which (`createSize`, `createMode`, `onCreateSizeChange`, `onCreateModeChange`) are dead weight in the edit case and passed as defaulted no-ops. Split into `<EditConfigForm>` + `<CreateConfigForm>` sharing a `<ConfigFields>` child, or pass a discriminated `createMeta?` prop. | R-056 |
| KI-016 | Low | Pipeline/solver/regions/mode literals in `AdminPage.tsx` (`PIPELINE_OPTIONS`, `SOLVER_OPTIONS`, `REGIONS_OPTIONS`, `MODE_OPTIONS`) are `string[]`, and `ConfigData.pipeline` etc. are `string`. Backend has typed constants (`PipelineRegionFirst`, `SolverBacktrack`, …). Export equivalents from `adminService.ts` as `as const` unions so an invalid literal is a compile error. | R-056 |
| KI-017 | Low | `Taskfile.yml` `dev:up:backend` / `dev:up:frontend` and `dev:down:backend` / `dev:down:frontend` are near-duplicate shell blocks differing only by port, command, and log file. ~100 lines could collapse to an internal helper task with `vars:`. Low priority because divergence risk is small. | — |
| KI-018 | Low | `AdminPage.tsx` re-fetches the whole pool after every mutation (`loadPool()` called from `replenish` / `update` / `create`). Single-combo edits should update local state from the response and only re-read that combo's `readyCount`. Also, `fetchPoolStatus` has no `AbortSignal` plumbing, so unmounting mid-load sets state on an unmounted component. | R-056 |
| KI-019 | Low | Config validation rules are duplicated between `backend/internal/handler/admin_config.go:validateConfigFields` and `backend/internal/handler/pipeline.go:ParseGenerateParams`. Error messages are byte-for-byte identical in places, and the two paths already disagree on edge cases (admin checks `Inf` explicitly, generate-params doesn't). Extract per-field validators (`validatePipeline`, `validateRegionVariance`, …) and call from both sites. | R-013, R-052, R-053 |
| KI-020 | Low | `Taskfile.yml` `dev:up:localstack` polls `docker compose exec -T localstack awslocal ...` twice per second while waiting for init, incurring ~200-500ms container-exec overhead per probe (up to 60 iterations × 2 probes). Call the localstack endpoints directly from the host (`aws --endpoint-url http://localhost:4566 ...`) to skip the exec hop. | — |

---

## Design Decisions Log

Key decisions made during development that affect the roadmap. Linked to the relevant roadmap item.

| Decision | Rationale | Date | Related |
|----------|-----------|------|---------|
| Open source puzzle generator (MIT) | Builds community trust, moat is curated DB + hosted service, discourages competitors from re-implementing | 2026-04-12 | R-012 |
| DynamoDB over Aurora Serverless | Zero-cost at idle, pay-per-request fits early stage, read-heavy workload maps well | 2026-04-12 | R-004 |
| Go over Kotlin/Rust for backend | Best Lambda cold start times, strongly typed, simple concurrency, lean dependencies | 2026-04-12 | R-002 |
| 6 daily puzzles (3 Standard + 3 Double Queens) | May be reduced if engagement data shows fatigue — starting ambitious | 2026-04-12 | R-031 |
| Freemium, no ads | Pay for content access, not ad removal | 2026-04-12 | R-053 |
| Working title: Reign | Evokes royalty/regions, not locked to chess theme, short and memorable | 2026-04-12 | — |
| Tactile default theme | Brand identity from tactile depth, warm ink palette + bold region colors. Queens is a secondary theme. | 2026-04-12 | R-015, R-016 |
| Theme system baked into Phase 1 | Must be part of component architecture from the start, not bolted on later | 2026-04-12 | R-014 |

---

*This document is updated after each phase completion and when new issues are discovered.*
