# Phase 0: Design Document

Authoritative design reference for Phase 0 implementation. See design-grill-summary.md for the full decision log and rationale.

## Monorepo Structure (R-001)

Root directories: `frontend/`, `backend/`, `infra/`, `design/`, `.github/workflows/`. `Taskfile.yml` at repo root. Layout follows PROJECT_STRUCTURE.md.

## Backend (R-002)

Go 1.26 with chi v5.2.5 router. Single Lambda function behind API Gateway using aws-lambda-go-api-proxy v1.54.0.

Directory layout:
```
backend/
├── cmd/api/main.go              # Lambda entry point + local HTTP server
├── internal/handler/health.go   # GET /health handler
├── internal/handler/health_test.go
├── go.mod
├── go.sum
└── .golangci.yml
```

`main.go` detects the Lambda environment. In Lambda: starts with chi adapter proxy. Locally: starts a plain `net/http` server on a configurable port (default 8080).

Health check: `GET /health` returns `200` with `{"status": "ok"}`.

Lambda runtime: `provided.al2023`. Deployment: zip archive of the Go binary.

## Frontend (R-003)

React 19, Vite 8, TypeScript 6, Tailwind 4.2.2, Node 24 LTS.

Minimal scaffold: `App.tsx` with placeholder text, `main.tsx` entry point, `index.html`. Vitest configured with one passing test. PWA manifest stub. Tailwind 4 CSS-first configuration (no `tailwind.config.ts`).

No React Router, no components, no pages, no Playwright. Phase 0 proves the build pipeline, not the UI.

## Infrastructure (R-004)

Terraform 1.14.8. Two modules:

**`infra/modules/frontend/`** — S3 bucket (private), CloudFront distribution with Origin Access Control (OAC). Output: CloudFront distribution URL, S3 bucket name.

**`infra/modules/api/`** — API Gateway REST API, single Lambda function, IAM execution role (least-privilege: CloudWatch Logs only). Lambda reads zip from a path managed by Terraform. Output: API Gateway invoke URL.

**Root config** — `main.tf` calls both modules. `variables.tf` defines all inputs (no defaults for AWS-specific values). `outputs.tf` exposes CloudFront URL and API Gateway URL. `backend.tf` references pre-existing S3 state bucket via variables.

No custom domains. No DynamoDB module. No auth module.

## CI Pipeline (R-005)

GitHub Actions workflow: `.github/workflows/ci.yml`, triggers on PR to main.

Four parallel jobs, all run to completion (no early exit):

| Job | Steps |
|-----|-------|
| backend | `go build ./...`, `go test ./... -v`, `golangci-lint run` |
| frontend | `npm ci`, `npm run build`, `npm test` |
| terraform-plan | `terraform init`, `terraform validate`, `terraform plan` — output posted as PR comment |
| security | `gitleaks detect --source .`, `govulncheck ./...` (backend/), `npm audit --audit-level=moderate` (frontend/) |

## CD Pipeline (R-006)

GitHub Actions workflow: `.github/workflows/cd.yml`, triggers on push to main.

Steps: build Go binary and zip, build frontend, `terraform init` + `terraform apply -auto-approve`. Terraform manages everything: Lambda code update, S3 frontend sync, CloudFront cache invalidation.

AWS credentials via OIDC or GitHub secrets. No AWS specifics in the repo.

## Dev Workflow (R-007)

`Taskfile.yml` at repo root. Targets:

| Task | Command |
|------|---------|
| `build:backend` | `go build ./...` in backend/ |
| `test:backend` | `go test ./... -v` in backend/ |
| `lint:backend` | `golangci-lint run` in backend/ |
| `dev:backend` | `go run ./cmd/api` in backend/ |
| `build:frontend` | `npm run build` in frontend/ |
| `test:frontend` | `npm test` in frontend/ |
| `dev:frontend` | `npm run dev` in frontend/ |
| `build` | `build:backend` + `build:frontend` |
| `test` | `test:backend` + `test:frontend` |
| `deploy` | `terraform apply` in infra/ (depends on `build`) |

## Assumptions

- Pre-existing S3 state bucket is available and the deploying IAM role has access.
- GitHub Actions runners have AWS credentials configured via OIDC or secrets.
- `provided.al2023` Lambda runtime supports Go custom runtimes.
- Tailwind 4 CSS-first configuration works with Vite 8.

## Deferred

- Custom domains (Phase 4/5)
- Playwright e2e testing (Phase 1)
- React Router (Phase 1)
- DynamoDB module (Phase 4)
- Auth module (Phase 5+)
- Brand guidelines (separate OpenSpec change, R-008)
