# Spec: CI/CD Pipelines and Dev Workflow

Covers R-005 (CI), R-006 (CD), and R-007 (Taskfile).

## CI Requirements

### CI-01: Workflow Trigger

- `.github/workflows/ci.yml` triggers on `pull_request` to `main`

### CI-02: Backend Job

- Uses Go 1.26
- Working directory: `backend/`
- Runs: `go build ./...`, `go test ./... -v`, `golangci-lint run`

### CI-03: Frontend Job

- Uses Node 24
- Working directory: `frontend/`
- Runs: `npm ci`, `npm run build`, `npm test`

### CI-04: Terraform Plan Job

- Working directory: `infra/`
- Runs: `terraform init` (with `-backend-config` flags from secrets/vars), `terraform validate`, `terraform fmt -check`, `terraform plan`
- Posts the plan output as a comment on the PR (via `github-script` action)
- Explicitly fails the job if the plan step fails (not swallowed by continue-on-error)
- Requires AWS credentials (from OIDC or GitHub secrets)

### CI-05: Security Job

- `gitleaks detect --source .` (repo root)
- `govulncheck ./...` (working directory: `backend/`)
- `npm audit --package-lock-only --audit-level=moderate` (working directory: `frontend/`, no install needed)
- All three checks in a single job

### CI-06: All Jobs Run to Completion

- All four jobs run in parallel — no job depends on another
- No job short-circuits other jobs on failure
- Each job reports its own pass/fail status independently

## CD Requirements

### CD-01: Workflow Trigger

- `.github/workflows/cd.yml` triggers on `push` to `main`

### CD-02: Build Artifacts

- Backend: `GOOS=linux GOARCH=amd64 go build` produces `bootstrap` binary, then zipped
- Frontend: `npm ci && npm run build` produces `dist/` directory
- Both built before Terraform apply

### CD-03: Deploy

- Runs `terraform init` (with `-backend-config` flags) + `terraform apply -auto-approve`
- Terraform handles: Lambda zip upload and infrastructure state
- S3 frontend sync via `aws s3 sync` (separate step — Terraform is not suited for syncing many static files)
- CloudFront cache invalidation via `aws cloudfront create-invalidation` (separate step)
- AWS credentials via OIDC or GitHub secrets

## Taskfile Requirements

### WF-01: Taskfile Exists

- `Taskfile.yml` at repository root
- Uses Task (taskfile.dev) syntax

### WF-02: Backend Tasks

- `task build:backend` — `go build ./...` in `backend/`
- `task test:backend` — `go test ./... -v` in `backend/`
- `task lint:backend` — `golangci-lint run` in `backend/`
- `task dev:backend` — `go run ./cmd/api` in `backend/`

### WF-03: Frontend Tasks

- `task build:frontend` — `npm run build` in `frontend/`
- `task test:frontend` — `npm test` in `frontend/`
- `task dev:frontend` — `npm run dev` in `frontend/`

### WF-04: Aggregate Tasks

- `task build` — runs `build:backend` and `build:frontend`
- `task test` — runs `test:backend` and `test:frontend`

### WF-05: Deploy Task

- `task deploy` — runs `terraform apply` in `infra/`
- Depends on `build` (both backend and frontend must be built first)

## Acceptance Criteria

All CI-*, CD-*, and WF-* requirements pass. CI runs four parallel jobs to completion on PRs. CD deploys via Terraform on merge. All Taskfile targets execute locally without error.
