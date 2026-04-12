# Reign

A grid-based puzzle game where you place markers on colored regions, subject to row, column, region, and adjacency constraints. Available as a Progressive Web App.

**Status:** Early development — project foundation and design phase.

## Game Modes

- **Standard** — One marker per row, column, and region. No two markers may be adjacent.
- **Double** — Two markers per row, column, and region. Same adjacency rules.
- **Daily Challenge** — Same puzzle for all players, speed-based leaderboard.

Grid sizes: 5x5 (Easy), 7x7 (Medium), 9x9 (Hard).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | React 18, TypeScript, Vite, Tailwind CSS, Workbox (PWA) |
| Backend | Go, AWS Lambda, API Gateway |
| Database | DynamoDB (on-demand) |
| Infrastructure | Terraform, AWS |
| CI/CD | GitHub Actions |

## Project Structure

```
reign-game/
├── frontend/          # React + TypeScript PWA
├── backend/           # Go serverless API
├── infra/             # Terraform infrastructure
├── design/            # Wireframes, image prompts, OpenSpec artifacts
├── .claude/           # AI agent definitions and skills
├── GAME_DESIGN.md     # Living design vision
├── ROADMAP.md         # Phased roadmap and known issues
└── GLOSSARY.md        # Domain vocabulary
```

## Prerequisites

- Go 1.22+
- Node.js 20+
- Terraform 1.5+
- Docker (for LocalStack)

## Getting Started

```bash
# Clone
git clone git@github.com:lesteenman/reign-game.git
cd reign-game

# Backend
cd backend
go mod download
go test ./... -v
go run ./cmd/api

# Frontend
cd frontend
npm install
npm run dev          # dev server at localhost:5173
npm run test         # unit tests
npm run build        # production build

# Local DynamoDB
docker compose up localstack

# Infrastructure (requires AWS credentials)
cd infra
terraform init
terraform plan
terraform apply
```

## Development

All work happens on feature branches, merged via PR to `main`. Merges to `main` trigger automatic deployment.

### Running Tests

```bash
# Backend
cd backend && go test ./... -v

# Frontend unit tests
cd frontend && npm run test

# Frontend e2e tests
cd frontend && npx playwright test

# Lint
cd backend && golangci-lint run
```

## Contributing

This project uses an open core model. The puzzle generator and app shell are open source (MIT). See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, including required tooling.

## License

TBD — Core engine and app shell will be MIT. Curated puzzle database and premium features are proprietary.
