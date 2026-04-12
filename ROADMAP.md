# Queens Game — Roadmap & Known Issues

Living document tracking planned work and known problems. Each item has an ID for reference in commits and discussions.

Status key: `[ ]` todo, `[~]` in progress, `[x]` done, `[!]` blocked

---

## Phase 0: Project Foundation

Goal: Repository structure, CI/CD pipeline, infrastructure scaffolding. No game logic yet.

- [ ] **R-001** — Initialize monorepo structure (frontend/, backend/, infra/, design/)
- [ ] **R-002** — Set up Go module in backend/ with basic Lambda handler (health check)
- [ ] **R-003** — Set up React + TypeScript + Vite in frontend/ with PWA scaffolding
- [ ] **R-004** — Terraform foundation: S3 bucket, CloudFront distribution, API Gateway, Lambda, DynamoDB tables
- [ ] **R-005** — GitHub Actions CI: lint + test on PR for both frontend and backend
- [ ] **R-006** — GitHub Actions CD: deploy to AWS on merge to main
- [ ] **R-007** — Basic dev workflow: local frontend dev server, Go backend runner, LocalStack for DynamoDB
- [ ] **R-008** — Brand guidelines generation (ui-ux-pro-max skill)

## Phase 1: Core Puzzle Engine

Goal: Playable Standard Mode puzzles in the browser. No server interaction yet — puzzles bundled in frontend.

- [ ] **R-010** — Puzzle data model: grid, regions, solution representation
- [ ] **R-011** — Puzzle solver: verify solution correctness, check uniqueness
- [ ] **R-012** — Puzzle generator: produce valid Standard Mode puzzles for 5x5, 7x7, 9x9
- [ ] **R-013** — Difficulty rating algorithm
- [ ] **R-014** — Theme architecture: ThemeContext, theme data structure, component token consumption
- [ ] **R-015** — Minimalist default theme: piece icons, color palette, grid styling, animations (placeholder art OK)
- [ ] **R-016** — Queens classic theme: chess queen icons, parchment aesthetic, crown animations (placeholder art OK)
- [ ] **R-017** — Interactive grid component: render regions, place/remove markers, highlight conflicts (theme-aware)
- [ ] **R-018** — Game state management: timer, move tracking, completion detection
- [ ] **R-019** — Practice mode UI: difficulty selector, puzzle loading, solve flow, completion screen
- [ ] **R-01A** — Seed puzzle database: generate + manually curate initial puzzle set (at least 20 per difficulty)
- [ ] **R-01B** — Bundle seed puzzles in frontend for offline-first play

## Phase 2: Double Queens Mode

Goal: Full Double Queens support in generator, solver, and UI.

- [ ] **R-020** — Extend solver for Double Queens constraints (2 per row/column/region)
- [ ] **R-021** — Extend generator for Double Queens puzzles
- [ ] **R-022** — Update difficulty rating for Double Queens
- [ ] **R-023** — UI: mode toggle (Standard / Double Queens)
- [ ] **R-024** — Generate + curate Double Queens puzzle set (at least 20 per difficulty)

## Phase 3: Backend API + Daily Puzzles

Goal: Server-side puzzle serving, daily challenge, leaderboard.

- [ ] **R-030** — Puzzle API: GET puzzle by ID, GET daily puzzle (by date + mode + difficulty)
- [ ] **R-031** — Daily puzzle scheduling: assign puzzles to dates, serve the right one per day
- [ ] **R-032** — Completion API: POST completion (puzzle ID, time), compute leaderboard position
- [ ] **R-033** — Leaderboard API: GET leaderboard for daily puzzle (percentile + absolute rank)
- [ ] **R-034** — Anonymous device identity: generate + store device ID, associate completions
- [ ] **R-035** — Frontend: daily challenge flow (fetch puzzle, timer, submit completion, show results)
- [ ] **R-036** — Frontend: leaderboard view (percentile, rank, total players)

## Phase 4: PWA & Offline

Goal: Installable PWA with offline practice mode.

- [ ] **R-040** — Service worker: cache app shell, practice puzzle set, static assets
- [ ] **R-041** — Offline detection: graceful degradation (practice available, daily requires connection)
- [ ] **R-042** — Install prompt: guide users to add to home screen
- [ ] **R-043** — Background sync: queue daily completions when offline, submit when reconnected

## Phase 5: User Accounts & Premium

Goal: Optional accounts, stats sync, premium subscription.

- [ ] **R-050** — Cognito setup: Google + Apple OAuth
- [ ] **R-051** — User API: create account, link device, get profile
- [ ] **R-052** — Stats sync: merge local stats with server on account creation
- [ ] **R-053** — Premium subscription: Stripe integration, subscription management API
- [ ] **R-054** — Premium features: full puzzle archive access, detailed stats
- [ ] **R-055** — Premium theme infrastructure: theme store, unlock/lock logic, theme preview
- [ ] **R-056** — Premium themes: Gems, Garden, Neon, Cosmos (at least 2 at launch) — Nano Banana 2 final art
- [ ] **R-057** — Frontend: account settings, login/logout, subscription management
- [ ] **R-058** — Frontend: detailed stats views (time trends, difficulty progression, achievements)

## Phase 6: Polish & Launch

Goal: Production readiness, performance, monitoring.

- [ ] **R-060** — Performance audit: Core Web Vitals, Lambda cold starts, DynamoDB latency
- [ ] **R-061** — Monitoring: CloudWatch dashboards, error alerting
- [ ] **R-062** — Dev/prod environment split (Terraform workspaces)
- [ ] **R-063** — Rate limiting and abuse prevention on completion API
- [ ] **R-064** — Accessibility audit: WCAG 2.1 AA compliance
- [ ] **R-065** — Colorblind-friendly region palettes
- [ ] **R-066** — Landing page / marketing site
- [ ] **R-067** — Open source preparation: LICENSE files, CONTRIBUTING.md, README for generator

---

## Known Issues

*No known issues yet. Add them here as they're discovered during development.*

| ID | Severity | Description | Related |
|----|----------|-------------|---------|
| — | — | — | — |

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
| Minimalist default theme | Brand identity from clean grid + region colors, not a specific metaphor. Queens is a secondary theme. | 2026-04-12 | R-015, R-016 |
| Theme system baked into Phase 1 | Must be part of component architecture from the start, not bolted on later | 2026-04-12 | R-014 |

---

*This document is updated after each phase completion and when new issues are discovered.*
