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

- [x] **R-060** — Frontend: undo/redo buttons in the puzzle UI. Stack-based history of placements/exclusions/clears; redo stack cleared on new action. Keyboard shortcuts (Ctrl/Cmd+Z, Ctrl/Cmd+Shift+Z). Persists with the rest of the game state in IndexedDB so undo survives reload. No server changes.

## Phase 5: Generator Rework

Goal: Rework the puzzle generator based on new designs (to be detailed via design-flow). Expected motivations: faster generation (especially Double Queens — see KI-007), higher-quality region shapes, more reliable deducibility guarantees. Scope and task breakdown come from the design session.

- [x] **R-061** — Design-flow: capture the new generator design, decide algorithm(s), and split into implementation tasks

Phase 5 implementation slices R-062..R-06D are tracked in
`openspec/changes/phase-5-generator-rework/tasks.md`. All shipped: generator
scaffold (R-062), sampler (R-063), deductive solver (R-064), region grower
cheap + solver-guided (R-065 / R-066), mutator + consumer cleanup (R-067),
measurement + soak + distribution (R-068), cutover (R-069), post-cutover
dynamic modes (R-06A), e2e harness (R-06B), seed capture (R-06C), logging +
dev-stack lifecycle (R-06D). Two quality deferrals carry over into Phase 6b
below.

## Phase 6: Verdict System

Goal: Rate puzzles after playing them — upvote, downvote, or skip.

- [ ] **R-081** — `PUT /puzzles/:id/verdict` endpoint: upvote/downvote/skip
- [ ] **R-082** — Frontend: verdict buttons on puzzle completion/skip

## Phase 6b: Generator quality deferrals

Two follow-ups from Phase 5's measurement pass (R-068). Both need the audit-loop tooling (Phase 6 verdicts, Phase 7 replay, Phase 8 analysis) to resolve. They are explicitly out of Phase 5 scope — capture here so the next audit pass picks them up.

- [ ] **R-083** — **Dead-rule investigation (R6, R8, R9).** R-068b's property corpus found R6 (Tier 3), R8 (Tier 4), R9 (Tier 4) never fire across 500 generated puzzles. Per input-spec §7.2 a dormant rule is redundant or buggy. Hand-craft a minimal `solverState` fixture per rule. If a fixture exists the rule is reachable and the generator must be retuned to produce such puzzles — tie-in for the audit-loop's "what kinds of puzzles do we actually produce" analysis. If no fixture can be built, retire the rule from `rules.go` and drop the classifier's tier-max accordingly. Currently tracked in code via `propertyCorpusKnownDead` in `backend/internal/generator/property_test.go`. Outcome also determines whether `WithDifficulty(Expert)` is ever shippable (see `backend/internal/generator/bench/step11-handoff.md` §2).

- [ ] **R-084** — **Medium / Hard blind calibration test.** R-068d's distribution shows every generated puzzle is Medium or Hard by the classifier, with zero Easy and zero Expert. The classifier split at N=12 k=1 is ~55% Medium / ~45% Hard. Intuition says the split is plausible (the mutator explicitly seeks stalled states), but the label boundary is unverified: a "Medium" may play harder than a "Hard" or vice versa. Requires Phase 6 verdict capture for play-time and user-rated difficulty across a labeled corpus, then a blind-test statistical check on whether the two tiers are actually perceptibly different. If they aren't, either collapse the tiers or retune the classifier thresholds.

## Phase 7: Puzzle Replay

Goal: Admin can browse played puzzles and replay them to review quality.

- [ ] **R-085** — `GET /puzzles/:id` endpoint: load any puzzle by ID for replay
- [ ] **R-086** — Frontend: played puzzle list in admin UI, replay by ID

## Phase 8: Puzzle Analysis Agent

Goal: Automated analysis of played puzzles — generation performance, verdict patterns, engine comparison.

- [ ] **R-087** — Analysis agent: dedicated agent for querying and interpreting puzzle generation data
- [ ] **R-088** — Analysis endpoint(s) as needed by the agent

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
- [ ] **R-080** — **R-06B follow-up — full e2e coverage.** R-06B shipped the e2e infrastructure plus two validating tests (Standard 5×5 play-through with undo + dynamic-modes wiring). Expand coverage to: Double 9×9 play-through, serve-then-mark-served lifecycle (seed two puzzles, confirm different ones served and both marked `served` after), pool-empty UI state, generation-path tests once the generator is exercised through `task e2e:up` rather than pre-seeded fixtures. Each new flow adds one spec file under `frontend/playwright/e2e/`.

---

## Known Issues

**Tagging conventions.** `[workaround]` at the start of the description means the KI was shipped with a band-aid, not a root-cause fix — the real fix is still open. `[blocks-prod: yes]` / `[blocks-prod: no]` means the KI must / need not be resolved before the first production exposure. Absent tags = normal backlog item. Phase PRs surface every `[workaround]` KI in a "Workarounds shipped" section for review visibility.

| ID | Severity | Description | Related |
|----|----------|-------------|---------|
| KI-001 | Medium | GitHub Actions use major version tags, not SHA pins. Pin before handling sensitive data (auth, payments). | R-005, R-006 |
| KI-002 | Low | CloudFront missing security response headers (HSTS, X-Content-Type-Options, CSP). Add before production. | R-004 |
| KI-003 | Low | S3 bucket has no explicit server-side encryption config (AWS defaults to SSE-S3, but should be explicit). | R-004 |
| KI-004 | Low | PWA service worker disabled — vite-plugin-pwa lacks Vite 8 support. Track [vite-pwa/vite-plugin-pwa#923](https://github.com/vite-pwa/vite-plugin-pwa/issues/923). Manifest + icons in place; re-add plugin when compatible. | R-01A |
| KI-005 | ~~Low~~ Fixed | ~~Timer does not start immediately when opening a new puzzle.~~ Fixed: immediate tick on timer start. | R-019 |
| KI-006 | ~~Medium~~ Fixed | ~~Every CD deploy updates the Lambda function even when there are no backend changes.~~ Fixed: reproducible zip (touch + zip -X) means identical source produces identical hash. | R-006 |
| KI-007 | ~~High~~ Fixed | ~~Double Queens puzzle generation too slow (12+ min for 7x7) with deducibility check. Disabled in replenish and UI.~~ Fixed by Phase 5 generator rework (R-062..R-067). N=9 k=2 generation at 100% success; end-to-end Generate ~3 ms/op. 7x7 Double turned out to be infeasible (N=7 k=2 has 0 solutions under 8-neighbor adjacency + 2 marks/row). 9x9 Double re-enabled in LocalStack seed and the frontend PuzzleSelector. | R-030, R-031, R-066, R-067c |
| KI-008 | ~~Medium~~ Fixed | ~~"Play Again" and "Retry" buttons don't work.~~ Fixed: buttons now trigger re-fetch via state reset instead of URL navigation. | R-044 |
| KI-009 | **Critical** (pre-production) | `/api/admin/*` routes (`GET /api/admin/pool`, `PUT /api/admin/config/{size}/{mode}`, `POST /api/admin/config`, `POST /api/admin/replenish`) have no authentication in the backend, API Gateway (`authorization = "NONE"`), or CloudFront. Any anonymous caller can read pool state, mutate generation configs, and trigger replenish. Must be gated before exposing the admin UI to any non-trusted network. Pairs with auth rollout in R-075. Interim mitigation: threshold capped at 50 to blunt SQS amplification; CloudFront forwards `Authorization` so the future token flow is ready. | R-051, R-052, R-053, R-054, R-075 |
| KI-010 | Medium | `GET /api/admin/pool` (`backend/internal/handler/admin_pool.go`) does 1 Query for configs plus 1 per-combo `CountReady` Query serially — N+1 on Lambda cold-start path. Fix with `errgroup` (bounded) or a single pre-aggregated count attribute. | R-051 |
| KI-011 | Medium | `repository.CountReady` and `repository.NextReady` (`backend/internal/repository/puzzle.go`) use a `FilterExpression` on `status = "ready"`, which forces DynamoDB to read the full partition (including historical served/solved/skipped puzzles) before filtering. Cost + latency grow with lifetime pool volume, not ready inventory. Fix with a sparse GSI keyed on `{size}#{mode}#ready` that only ready items populate (writers add attributes on put, `MarkServed` / `UpdateStatus` remove them). | R-040, R-044 |
| KI-012 | Medium | `ReplenishHandler` (`backend/internal/handler/replenish.go`) publishes one SQS message per unit of `threshold - count` in a serial loop. For 5 combos at threshold 10 with an empty pool that is 50 sequential `SendMessage` calls inside an HTTP handler. Switch to `SendMessageBatch` (up to 10/call) and/or parallelize across combos. Add a `PublishBatch` method to `queue.Publisher`. | R-042, R-043, R-054 |
| KI-013 | ~~Low~~ Fixed | ~~The config payload shape is re-declared four times: `repository.ConfigRecord`, `handler.configRequest`, `handler.configResponse`, and the hand-rolled `handler.buildConfigResponseMap`.~~ Fixed in R-06A by introducing a shared `handler.ConfigBody` plus explicit `ConfigView` (flat response) + `ConfigCreateRequest` + `ConfigUpdateRequest` DTOs, with mapping functions at the handler boundary. The `configRequest` / `configResponse` / `buildConfigResponseMap` triplet is gone; `admin_pool` reuses `ConfigBody` as the nested config. | R-050, R-052, R-06A |
| KI-014 | Low | `frontend/src/services/api.ts` has `apiFetch`, `apiPut`, `apiPost` as three near-identical functions (~25 lines each) differing only by HTTP method and whether a body is sent. Empty-body response handling is also inconsistent across them. Collapse into one `apiRequest(method, path, opts)` with thin wrappers. | R-046, R-056 |
| KI-015 | ~~Low~~ Fixed | ~~`AdminPage.tsx` `ConfigForm` takes 9 primitive props, four of which (`createSize`, `createMode`, `onCreateSizeChange`, `onCreateModeChange`) are dead weight in the edit case and passed as defaulted no-ops.~~ Fixed in R-06A by splitting into `<EditConfigForm>` + `<CreateConfigForm>`, both wrapping a shared `<FormShell>` (chrome) + `<ConfigFields>` (body inputs). Each form takes exactly what it needs. | R-056, R-06A |
| KI-016 | ~~Low~~ Fixed | ~~Pipeline/solver/regions/mode literals in `AdminPage.tsx` (`PIPELINE_OPTIONS`, `SOLVER_OPTIONS`, `REGIONS_OPTIONS`, `MODE_OPTIONS`) are `string[]`.~~ Pipeline / solver / regions options removed in R-067 consumer cleanup. `MODE_OPTIONS` fixed in R-06A: `MODES` is now `['standard', 'double'] as const` exported from `adminService.ts`, with a `Mode` type union and an `isMode()` type guard. Every `mode` field in the service types is now typed as `Mode`, so invalid literals are compile errors. | R-056, R-067, R-06A |
| KI-017 | Low | `Taskfile.yml` `dev:up:backend` / `dev:up:frontend` and `dev:down:backend` / `dev:down:frontend` are near-duplicate shell blocks differing only by port, command, and log file. ~100 lines could collapse to an internal helper task with `vars:`. Low priority because divergence risk is small. | — |
| KI-018 | Low | `AdminPage.tsx` re-fetches the whole pool after every mutation (`loadPool()` called from `replenish` / `update` / `create`). Single-combo edits should update local state from the response and only re-read that combo's `readyCount`. Also, `fetchPoolStatus` has no `AbortSignal` plumbing, so unmounting mid-load sets state on an unmounted component. | R-056 |
| KI-019 | Low | Config validation rules are duplicated between `backend/internal/handler/admin_config.go:validateConfigFields` and `backend/internal/handler/pipeline.go:ParseGenerateParams`. Error messages are byte-for-byte identical in places, and the two paths already disagree on edge cases (admin checks `Inf` explicitly, generate-params doesn't). Extract per-field validators (`validatePipeline`, `validateRegionVariance`, …) and call from both sites. | R-013, R-052, R-053 |
| KI-020 | Low | `Taskfile.yml` `dev:up:localstack` polls `docker compose exec -T localstack awslocal ...` twice per second while waiting for init, incurring ~200-500ms container-exec overhead per probe (up to 60 iterations × 2 probes). Call the localstack endpoints directly from the host (`aws --endpoint-url http://localhost:4566 ...`) to skip the exec hop. | — |
| KI-021 | ~~Medium~~ Fixed | ~~`task dev:down:backend` reported `backend stopped (was X)` while leaving PID X alive — the kill path ran but the success print wasn't gated on verification. Combined with the generator's independent PID-file tracking, this allowed multiple pre-R-067b workers to accumulate over days, one of which generated a 9x9 Standard puzzle with a 1-cell region on 2026-04-22.~~ Fixed in R-06D by (a) wrapping down tasks in bash heredocs, (b) verifying the port/PID is gone before claiming success, (c) adding a `dev:up` preflight that refuses to start if any `cmd/api` process exists that isn't the expected backend or generator. The R-06C "safety net" in `Generate()` is now belt-and-suspenders — the root cause was operational, not a generator rule leak. | R-06D |
| KI-022 | Low | `[blocks-prod: yes]` Backend API calls on local dev are inconsistently slow. Most requests return instantly; a subset (observed on `/api/puzzles/next`, `/api/admin/pool`, `/api/config/modes`) hang for several seconds before completing. Pattern feels random — not tied to pool size, recent generator activity, or LocalStack uptime. Possibly DynamoDB/SQS client connection-pool reuse, LocalStack cold-path, or chi middleware — needs investigation with a request-timing middleware or `go tool trace` before guessing. Prod not yet exposed to this — prod env doesn't exist; must resolve before prod launch. | — |
| KI-023 | Low | After solving a puzzle, clicking Home shows "No puzzles available right now. Try again in a moment." for a beat before the real buttons render. `LandingPage` uses `modes ?? []` when the fetch is still in flight, so `PuzzleSelector` enters its empty-state branch. Fix: distinguish loading (null) from empty ([]) — show a loading indicator while `modes === null`, and the empty-state UI only after a resolved fetch that returned zero combos. | R-06A |
| KI-024 | Low | `[workaround]` `[blocks-prod: no]` `GamePage`'s fetch-on-mount effect fires twice in dev (React StrictMode) because the `cancelled` flag only suppresses state updates, not the side effect itself. The first mount's `/api/puzzles/next` call succeeds and marks the fixture `served` before the cleanup runs; the second mount then gets 404. R-06B works around this by committing two identical fixtures so both mounts succeed. Proper fix: split serve-and-mark into two backend calls, or add an `AbortSignal` to `fetchNextPuzzle` and honor it on the backend. | R-06B |

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
