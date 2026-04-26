# Project Structure

Authoritative reference for this project's directory layout and API endpoints.

Monorepo layout. Search by domain keyword (e.g., `puzzle`, `grid`, `leaderboard`) to locate files.

---

## Repository Root

```
reign-game/
├── frontend/              # React + TypeScript PWA
├── backend/               # Go serverless API (Lambda handlers)
├── infra/                 # Terraform infrastructure-as-code
├── design/                # UI/UX assets, wireframes, OpenSpec artifacts
│   └── openspec/          # OpenSpec change artifacts (symlinked or moved from root)
├── docs/
│   └── runbooks/          # Operational runbooks
│       └── admin-auth-setup.md  # Clerk + GCP OAuth setup, key rotation, admin role grants (Phase 6, R-089)
├── scripts/               # Ops helpers (flush-pool, seed-configs)
├── .claude/               # Agent definitions, skills, settings
│   ├── agents/            # Agent .md files
│   └── skills/            # Skill directories with SKILL.md files
├── .github/
│   └── workflows/         # GitHub Actions CI/CD pipelines
├── Taskfile.yml           # Task runner for build, test, dev, deploy
├── CLAUDE.md              # Project config for Claude Code agents
├── GAME_DESIGN.md         # Living game design vision document
├── ROADMAP.md             # Phased roadmap + known issues
├── GLOSSARY.md            # Ubiquitous language glossary
├── PROJECT_STRUCTURE.md   # This file
└── BRAND_GUIDELINES.md    # Generated design system (created during first frontend work)
```

## Backend

```
backend/
├── cmd/
│   ├── api/
│   │   └── main.go              # Lambda entry + local dev entry (GENERATOR_MODE=sqs flips to consumer)
│   ├── genfixtures/
│   │   └── main.go              # Deterministic Playwright fixture generator (R-06B)
│   └── reproduce/
│       └── main.go              # Regenerate a puzzle from (seed, size, mode) for debugging (R-06C)
├── internal/
│   ├── auth/                    # Clerk session middleware (Phase 6, R-08A)
│   │   ├── doc.go               # Package doc: RequireAuth + RequireAdmin contract
│   │   ├── middleware.go        # RequireAuth / RequireAdmin chi middleware + UserFromContext
│   │   └── secret.go            # sync.Once CLERK_SECRET_KEY bootstrap (env var or SSM)
│   ├── httperr/                 # Shared JSON error-response writer (Phase 6, R-08A)
│   │   └── httperr.go           # WriteError(w, status, code, message) — used by handler + auth
│   ├── handler/                 # Chi-mux HTTP handlers, /api/* routes
│   │   ├── admin_config.go      # PUT /api/admin/config/{size}/{mode}, POST /api/admin/config
│   │   ├── admin_pool.go        # GET /api/admin/pool
│   │   ├── auth_test.go         # Admin-route auth-matrix helpers (anonymous/user/admin)
│   │   ├── config_dto.go        # ConfigBody + ConfigView + request DTOs (R-06A)
│   │   ├── config_modes.go      # GET /api/config/modes (public, R-06A)
│   │   ├── generate.go          # GET /api/puzzles/generate (legacy, slow)
│   │   ├── health.go            # GET /api/health
│   │   ├── params.go            # Shared handler helpers
│   │   ├── replenish.go         # POST /api/admin/replenish
│   │   ├── serve.go             # GET /api/puzzles/next
│   │   ├── status.go            # PUT /api/puzzles/{id}/status
│   │   └── verdict.go           # PUT /api/admin/puzzles/{id}/verdict (Phase 7, R-081)
│   ├── model/                   # Legacy domain types (Phase 5 kept puzzle.go only)
│   │   └── puzzle.go
│   ├── repository/              # DynamoDB data access
│   │   └── puzzle.go            # ConfigRecord + PuzzleRecord + CRUD (ErrPuzzleNotFound)
│   ├── queue/                   # SQS publisher
│   │   └── publisher.go
│   ├── worker/                  # SQS consumer — generates puzzles into the pool
│   │   └── generator.go         # Seed capture + SafetyNetTrips WARN logging (R-06C/D)
│   └── generator/               # Phase 5 rework — sampler + solver + grower + mutator + classifier
│       ├── doc.go
│       ├── generator.go         # Top-level Generate orchestrator + min-size safety net (R-06C)
│       ├── sample.go            # Solution Sampler (row-by-row bitmask)
│       ├── kcombos.go           # k-combination enumerator
│       ├── pair.go              # Seed pairing (k=2)
│       ├── grower.go            # Cheap region grower
│       ├── grower_scored.go     # Solver-guided grower (R-066)
│       ├── neighbors.go         # 4-neighbor helpers + BFS
│       ├── mutate.go            # Boundary-swap mutator + R-067c acceptance tuning
│       ├── solver.go            # Deductive solve loop
│       ├── solver_state.go      # solverState + ruleID + ruleTrace
│       ├── rules.go             # R1..R9 deductive rules
│       ├── classify.go          # Tier / difficulty classification
│       ├── brute.go             # bruteSolveAll — uniqueness cross-check
│       ├── output.go            # regionOf → [][]int conversion
│       ├── bench/               # Committed measurement artifacts
│       │   ├── baseline.txt     # go test -bench output (R-068a)
│       │   ├── latency-distribution.md  # p50/p99 per (N, k) (R-068a)
│       │   ├── difficulty-distribution.md  # tier histogram per (N, k) (R-068d)
│       │   ├── step11-handoff.md         # Step 11 decisions (R-068d)
│       │   ├── n-feasibility.md
│       │   └── n-feasibility-deep.md
│       └── testdata/            # Corpus + brute fixtures
├── go.mod
├── go.sum
└── Taskfile.yml                 # task build:backend / test:backend / lint:backend
```

The generator package ships several non-`_test.go` helper files not broken
out in the tree (`solver_cross_test.go`, `property_test.go`,
`distribution_test.go`, `latency_distribution_test.go`, `step7_test.go`,
`soak_test.go`, `mutate_connectivity_test.go`, `generator_bench_test.go`,
`diag_test.go`, plus per-rule unit tests). All live next to their
subjects; `go test ./internal/generator/...` runs them.

## Frontend

```
frontend/
├── public/
│   ├── manifest.json            # PWA manifest
│   └── icons/                   # App icons
├── src/
│   ├── App.tsx / App.test.tsx
│   ├── main.tsx
│   ├── components/
│   │   ├── auth/                # Clerk sign-in surface (Phase 6, R-08B)
│   │   │   ├── ClerkAvailability.tsx     # Renders children only when ClerkProvider is mounted
│   │   │   ├── ProtectedAdminRoute.tsx   # Route guard: anonymous → sign-in, non-admin → forbidden
│   │   │   ├── SignInButton.tsx          # Header sign-in CTA
│   │   │   ├── UserMenu.tsx              # Avatar + sign-out menu (signed-in users)
│   │   │   └── role.ts                   # Role helpers (publicMetadata.role)
│   │   ├── common/              # Button, PageShell, press helpers, button styles
│   │   ├── grid/                # Cell, Grid, Marker, ExclusionMark, RegionBorderOverlay
│   │   └── landing/
│   │       └── PuzzleSelector.tsx  # Dynamic mode buttons (R-06A)
│   ├── pages/
│   │   ├── AdminLandingPage.tsx # Forbidden state for signed-in non-admin users on /admin (R-08B)
│   │   ├── AdminPage.tsx        # Pool management UI (admin-only behind ProtectedAdminRoute)
│   │   ├── GamePage.tsx         # Active-puzzle view
│   │   └── LandingPage.tsx      # Entry point / resume / new puzzle
│   ├── services/
│   │   ├── api.ts               # Base apiFetch / apiPut / apiPost wrappers
│   │   ├── adminService.ts      # MODES, ConfigView, CRUD calls (R-06A)
│   │   ├── landingService.ts    # fetchEnabledModes (R-06A)
│   │   └── puzzleService.ts     # /api/puzzles/next
│   ├── hooks/
│   │   ├── useGame.ts
│   │   ├── useGameStorage.ts
│   │   └── useTimer.ts
│   ├── engine/                  # Client-side solution validation
│   │   ├── constraints.ts
│   │   ├── types.ts
│   │   └── validator.ts
│   ├── storage/                 # IndexedDB game state
│   │   ├── db.ts
│   │   ├── types.ts
│   │   └── utils.ts
│   ├── theme/
│   ├── test-setup.ts
│   ├── test-utils.tsx
│   ├── index.css
│   └── vite-env.d.ts
├── playwright/                  # Playwright suites (R-06B)
│   ├── README.md                # How to run integration vs e2e suites
│   ├── integration/             # Mocked-backend specs — run against dev Vite on :5180
│   │   └── grid-interaction.spec.ts
│   └── e2e/                     # Real-backend specs — run against :5183 → :5182 → puzzle-pool-e2e
│       ├── play-to-completion.spec.ts
│       ├── dynamic-modes.spec.ts
│       └── fixtures/
│           └── puzzles/         # Deterministic fixtures generated by `task e2e:genfixtures`
├── index.html
├── vite.config.ts
├── playwright.config.ts
├── tsconfig.json
├── vitest.config.ts
├── package.json
└── package-lock.json
```

## Infrastructure

```
infra/
├── modules/
│   ├── frontend/                # S3 + CloudFront
│   ├── api/                     # API Gateway + Lambda + Clerk SSM keys + IAM (Phase 6 admin auth lives here, R-089)
│   ├── database/                # DynamoDB tables
│   └── generation/              # SQS puzzle-generation queue + DLQ (Phase 4)
├── environments/
│   └── prod/                    # Production tfvars (single env initially)
├── main.tf
├── variables.tf
├── outputs.tf
└── backend.tf                   # Terraform state backend (S3)
```

## Design

```
design/
├── wireframes/                  # UI wireframes (placeholder art initially)
├── image-prompts/               # Nano Banana 2 image generation prompts
└── openspec/                    # OpenSpec change artifacts
    └── changes/
        └── <change-name>/
            ├── proposal.md
            ├── design.md
            ├── specs/
            └── tasks.md
```

## API Endpoints

### Implemented

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/health | No | Health check |
| GET | /api/puzzles/generate | No | On-demand puzzle generation (legacy, slow for large grids) |
| GET | /api/puzzles/next | No | Serve next ready puzzle from pool by size + mode |
| PUT | /api/puzzles/{id}/status | No | Update puzzle status (solved/skipped) |
| GET | /api/config/modes | No | Public list of enabled (size, mode) combos for the landing page (R-06A) |
| GET | /api/admin/pool | Admin | All combos with merged config + ready counts |
| PUT | /api/admin/config/{size}/{mode} | Admin | Update config for an existing combo |
| POST | /api/admin/config | Admin | Create a new combo config |
| POST | /api/admin/replenish | Admin | Replenish pools (optional ?size=X&mode=Y filter) |
| PUT | /api/admin/puzzles/{id}/verdict | Admin | Submit up/down verdict on a played puzzle (Phase 7, R-081) |

*Admin-marked endpoints sit behind `RequireAuth` + `RequireAdmin` (Phase 6, R-08A): anonymous → 401, signed-in non-admin → 403, signed-in with `publicMetadata.role === 'admin'` → 200. `/api/config/modes` is the public alternative the landing page calls so it never has to touch any admin endpoint.*

### Future (not yet implemented)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/puzzles/{id} | No | Load puzzle by ID for replay (Phase 8, R-085) |
| GET | /api/daily | No | Get today's daily puzzles (Phase 9+) |
| POST | /api/completions | Device ID | Submit puzzle completion (Phase 9+) |
| GET | /api/leaderboard/{puzzleId} | No | Leaderboard for a daily puzzle (Phase 9+) |

---

*Update this file when adding new directories, endpoints, or significant structural changes.*
