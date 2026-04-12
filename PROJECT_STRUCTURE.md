# Project Structure

Authoritative reference for this project's directory layout and API endpoints.

Monorepo layout. Search by domain keyword (e.g., `puzzle`, `grid`, `leaderboard`) to locate files.

---

## Repository Root

```
queens-game/
├── frontend/              # React + TypeScript PWA
├── backend/               # Go serverless API (Lambda handlers)
├── infra/                 # Terraform infrastructure-as-code
├── design/                # UI/UX assets, wireframes, OpenSpec artifacts
│   └── openspec/          # OpenSpec change artifacts (symlinked or moved from root)
├── .claude/               # Agent definitions, skills, settings
│   ├── agents/            # Agent .md files
│   └── skills/            # Skill directories with SKILL.md files
├── .github/
│   └── workflows/         # GitHub Actions CI/CD pipelines
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
│   └── api/
│       └── main.go              # Lambda entry point
├── internal/
│   ├── handler/                 # API Gateway Lambda handlers
│   │   ├── puzzle.go            # GET /puzzles/:id, GET /daily
│   │   ├── completion.go        # POST /completions
│   │   ├── leaderboard.go       # GET /leaderboard/:puzzleId
│   │   └── health.go            # GET /health
│   ├── model/                   # Domain types
│   │   ├── puzzle.go            # Puzzle, Region, Grid
│   │   ├── completion.go        # Completion record
│   │   └── user.go              # User, DeviceIdentity (Phase 2)
│   ├── service/                 # Business logic
│   │   ├── puzzle.go            # Puzzle retrieval, daily scheduling
│   │   ├── leaderboard.go       # Percentile + rank computation
│   │   └── completion.go        # Record + validate completions
│   ├── repository/              # DynamoDB data access
│   │   ├── puzzle.go
│   │   ├── completion.go
│   │   └── leaderboard.go
│   ├── generator/               # Puzzle generation (open source)
│   │   ├── generator.go         # Main generation loop
│   │   ├── solver.go            # Constraint solver + uniqueness check
│   │   ├── difficulty.go        # Difficulty rating algorithm
│   │   └── region.go            # Region shape generation
│   └── middleware/              # Shared middleware (auth, logging, cors)
├── go.mod
├── go.sum
├── Makefile
└── README.md                    # Open source README for generator
```

## Frontend

```
frontend/
├── public/
│   ├── manifest.json            # PWA manifest
│   ├── sw.js                    # Service worker (Workbox generated)
│   └── icons/                   # App icons (various sizes)
├── src/
│   ├── components/
│   │   ├── grid/                # Grid rendering, cell interaction
│   │   │   ├── Grid.tsx
│   │   │   ├── Cell.tsx
│   │   │   ├── Queen.tsx
│   │   │   └── RegionOverlay.tsx
│   │   ├── game/                # Game flow components
│   │   │   ├── Timer.tsx
│   │   │   ├── DifficultySelector.tsx
│   │   │   ├── ModeToggle.tsx
│   │   │   └── CompletionScreen.tsx
│   │   ├── leaderboard/         # Leaderboard display
│   │   │   ├── LeaderboardView.tsx
│   │   │   └── RankBadge.tsx
│   │   └── common/              # Shared UI components
│   │       ├── Layout.tsx
│   │       ├── Navigation.tsx
│   │       └── OfflineBanner.tsx
│   ├── pages/
│   │   ├── HomePage.tsx         # Landing / mode selection
│   │   ├── PracticePage.tsx     # Practice puzzle play
│   │   ├── DailyPage.tsx        # Daily challenge
│   │   ├── LeaderboardPage.tsx  # Daily leaderboard results
│   │   └── StatsPage.tsx        # Personal stats
│   ├── services/                # API client layer
│   │   ├── api.ts               # Base API client
│   │   ├── puzzleService.ts     # Puzzle endpoints
│   │   └── completionService.ts # Completion + leaderboard endpoints
│   ├── hooks/                   # Custom React hooks
│   │   ├── useGame.ts           # Game state management
│   │   ├── useTimer.ts          # Timer logic
│   │   ├── usePuzzle.ts         # Puzzle loading + caching
│   │   └── useOffline.ts        # Offline detection
│   ├── engine/                  # Client-side puzzle logic
│   │   ├── validator.ts         # Solution validation (runs locally)
│   │   ├── constraints.ts       # Row/column/region/adjacency checks
│   │   └── types.ts             # Puzzle type definitions
│   ├── styles/                  # Global styles, Tailwind config
│   ├── App.tsx
│   └── main.tsx
├── index.html
├── vite.config.ts
├── tailwind.config.ts
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
│   ├── api/                     # API Gateway + Lambda
│   ├── database/                # DynamoDB tables
│   └── auth/                    # Auth provider TBD (Phase 5+)
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

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /health | No | Health check |
| GET | /puzzles/:id | No | Get puzzle by ID (region map, no solution) |
| GET | /daily | No | Get today's daily puzzles (6 total: 3 Standard + 3 Double Queens) |
| POST | /completions | Device ID | Submit puzzle completion (puzzle ID, time) |
| GET | /leaderboard/:puzzleId | No | Get leaderboard for a daily puzzle (percentile, rank, total) |
| POST | /auth/register | No | Create user account via OAuth (Phase 2) |
| GET | /users/me | JWT | Get current user profile + stats (Phase 2) |
| PUT | /users/me/device | JWT | Link device to account (Phase 2) |
| GET | /puzzles/archive | JWT + Premium | Browse full puzzle archive (Phase 2) |

---

*Update this file when adding new directories, endpoints, or significant structural changes.*
