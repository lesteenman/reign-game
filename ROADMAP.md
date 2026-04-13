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

- [ ] **R-010** — Puzzle data model: grid, regions, solution representation (Go)
- [ ] **R-011** — Puzzle solver: constraint-based deduction, verify uniqueness (Go, 5x5 Standard)
- [ ] **R-012** — Puzzle generator: produce valid 5x5 Standard Mode puzzles (Go)
- [ ] **R-013** — Generate endpoint: stateless, returns a fresh puzzle on each call (no DB)
- [ ] **R-014** — Theme architecture: ThemeContext, theme data structure, component token consumption
- [ ] **R-015** — Minimalist default theme: piece icons, color palette, grid styling, animations (placeholder art OK)
- [ ] **R-016** — Interactive grid component: render regions, place/remove markers, pencil marks, highlight conflicts (theme-aware)
- [ ] **R-017** — Solution validation in TypeScript (constraint check, no solver)
- [ ] **R-018** — Game state in IndexedDB: placements, pencil marks, timer, completion status. Persist every move.
- [ ] **R-019** — Game flow UI: puzzle loading, timer, solve flow, completion screen
- [ ] **R-01A** — PWA basics: service worker (app shell caching), manifest, install prompt

## Phase 2: All Grid Sizes

Goal: 7x7 and 9x9 puzzles playable, with difficulty rating.

- [ ] **R-020** — Extend generator + solver for 7x7 and 9x9 grids
- [ ] **R-021** — Difficulty rating algorithm (region shape complexity, deduction chain depth)
- [ ] **R-022** — UI: difficulty selector (Easy / Medium / Hard)

## Phase 3: Double Queens Mode

Goal: Both Standard and Double Queens modes playable at all grid sizes.

- [ ] **R-030** — Extend solver for Double Queens constraints (2 per row/column/region)
- [ ] **R-031** — Extend generator for Double Queens puzzles
- [ ] **R-032** — Update difficulty rating for Double Queens
- [ ] **R-033** — UI: mode toggle (Standard / Double Queens)

## Phase 4: Curation + Puzzle Database

Goal: Curated puzzles served from backend. Curation tooling with visual solver. Offline practice.

- [ ] **R-040** — DynamoDB table for puzzles (candidates, curated, metadata)
- [ ] **R-041** — Terraform: add DynamoDB
- [ ] **R-042** — Admin generation endpoint: generate N candidates, rank by interest heuristic, return with solver steps
- [ ] **R-043** — Curation endpoints: list candidates, approve/reject, get solver steps
- [ ] **R-044** — Puzzle serving API: GET puzzles by mode/difficulty (curated pool)
- [ ] **R-045** — Frontend: curator mode route — generate, play, watch visual solver, upvote/downvote, approve/reject
- [ ] **R-046** — Frontend: "pick best of N" comparison mode
- [ ] **R-047** — Frontend: fetch curated puzzles from API, cache all accessible puzzles in IndexedDB
- [ ] **R-048** — Frontend: practice mode serves from curated pool (offline after first load)
- [ ] **R-049** — Offline detection: graceful degradation (practice available offline, curation requires connection)

## Phase 5+: Future (scoped when we get there)

Candidate items — not yet committed or ordered:

- [ ] **R-050** — Daily puzzle scheduling: assign puzzles to dates, serve by date + mode + difficulty
- [ ] **R-051** — Daily challenge flow: fetch puzzle, timer, submit completion, show percentile
- [ ] **R-052** — Anonymous completion submission + percentile calculation (stateless for free players)
- [ ] **R-053** — Leaderboards: premium members visible, daily and overall
- [ ] **R-054** — Auth provider setup (not Cognito) — Google + Apple OAuth
- [ ] **R-055** — One-time premium purchase flow
- [ ] **R-056** — Premium: full puzzle archive access, detailed stats, cross-device sync
- [ ] **R-057** — Premium themes: Queens Classic (free), Gems, Garden, Neon, Cosmos
- [ ] **R-058** — Performance audit: Core Web Vitals, Lambda cold starts, DynamoDB latency
- [ ] **R-059** — Monitoring: CloudWatch dashboards, error alerting
- [ ] **R-05A** — Dev/prod environment split (Terraform workspaces)
- [ ] **R-05B** — Accessibility audit: WCAG 2.1 AA compliance
- [ ] **R-05C** — Colorblind-friendly region palettes
- [ ] **R-05D** — Rate limiting and abuse prevention on completion API
- [ ] **R-05E** — Landing page / marketing site
- [ ] **R-05F** — Open source preparation: LICENSE files, CONTRIBUTING.md, README for generator

---

## Known Issues

| ID | Severity | Description | Related |
|----|----------|-------------|---------|
| KI-001 | Medium | GitHub Actions use major version tags, not SHA pins. Pin before handling sensitive data (auth, payments). | R-005, R-006 |
| KI-002 | Low | CloudFront missing security response headers (HSTS, X-Content-Type-Options, CSP). Add before production. | R-004 |
| KI-003 | Low | S3 bucket has no explicit server-side encryption config (AWS defaults to SSE-S3, but should be explicit). | R-004 |

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
