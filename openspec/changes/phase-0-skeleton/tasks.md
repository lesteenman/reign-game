# Phase 0: Implementation Tasks

## Dependency Layers

```
Layer 0 (foundation) → Layer 1 (parallel scaffolds) → Layer 2 (dev workflow) → Layer 3 (infrastructure) → Layer 4 (parallel CI/CD)
```

## Tasks

### Layer 0: Foundation

#### T-001: Initialize Monorepo Structure

- **Roadmap:** R-001
- **Agent:** workflow-orchestrator (direct — directory creation only)
- **Spec:** Referenced across all spec files
- **Work:**
  - Create directories: `frontend/`, `backend/`, `infra/`, `infra/modules/frontend/`, `infra/modules/api/`, `design/`, `.github/workflows/`
  - Add `.gitkeep` where needed for empty directories
- **Acceptance:** All directories exist. Structure matches PROJECT_STRUCTURE.md skeleton.
- **Commit after completion.**

### Layer 1: Backend + Frontend Scaffolds (parallel)

#### T-002: Backend Scaffold

- **Roadmap:** R-002
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BS-01 through BS-05)
- **Work:**
  - `go mod init` with Go 1.26
  - Add dependencies: chi v5.2.5, aws-lambda-go v1.54.0, aws-lambda-go-api-proxy
  - Create `cmd/api/main.go` with Lambda/local detection, chi router
  - Create `internal/handler/health.go` — `GET /health` returning `{"status": "ok"}`
  - Create `internal/handler/health_test.go` — table-driven test
  - Create `.golangci.yml` with reasonable defaults
  - Verify: `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run`
- **Acceptance:** All BS-* requirements pass.
- **Depends on:** T-001
- **Commit after completion.**

#### T-003: Frontend Scaffold

- **Roadmap:** R-003
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FS-01 through FS-08)
- **Work:**
  - Initialize npm project, install React 19, Vite 8, TypeScript 6, Tailwind 4.2.2, Vitest
  - Create `vite.config.ts` with React plugin
  - Create `tsconfig.json` with `strict: true`
  - Set up Tailwind 4 CSS-first config (global CSS with `@import "tailwindcss"`)
  - Create `src/App.tsx` (placeholder heading), `src/main.tsx`, `index.html`
  - Create `src/App.test.tsx` with Vitest
  - Create `public/manifest.json` PWA stub
  - Verify: `npm run build`, `npm test`, `npm run dev`
- **Acceptance:** All FS-* requirements pass.
- **Depends on:** T-001
- **Note:** No frontend-design or ui-ux-pro-max skills needed. Phase 0 is a placeholder — no visual UI.
- **Commit after completion.**

### Layer 2: Dev Workflow

#### T-004: Taskfile

- **Roadmap:** R-007
- **Agent:** devops-engineer
- **Spec:** specs/cicd-and-workflow.md (WF-01 through WF-05)
- **Work:**
  - Create `Taskfile.yml` at repo root
  - Define all targets per WF-* requirements
  - Verify each target runs successfully
- **Acceptance:** All WF-* requirements pass.
- **Depends on:** T-002, T-003
- **Commit after completion.**

### Layer 3: Infrastructure

#### T-005: Terraform Modules

- **Roadmap:** R-004
- **Agent:** devops-engineer
- **Spec:** specs/infrastructure.md (TF-01 through TF-06)
- **Work:**
  - Create `infra/backend.tf` — S3 state backend config with variables
  - Create `infra/modules/frontend/` — S3 bucket + CloudFront + OAC
  - Create `infra/modules/api/` — API Gateway + Lambda + IAM role
  - Create `infra/main.tf`, `variables.tf`, `outputs.tf`
  - Add `*.tfvars` to `.gitignore`
  - Ensure no hardcoded AWS specifics
  - Verify: `terraform validate`, `terraform fmt -check`
- **Acceptance:** All TF-* requirements pass.
- **Depends on:** T-002 (Lambda handler must exist for packaging context)
- **Commit after completion.**

### Layer 4: CI + CD Workflows (parallel)

#### T-006: CI Workflow

- **Roadmap:** R-005
- **Agent:** devops-engineer
- **Spec:** specs/cicd-and-workflow.md (CI-01 through CI-06)
- **Work:**
  - Create `.github/workflows/ci.yml`
  - Four parallel jobs: backend, frontend, terraform-plan, security
  - All jobs run to completion (no early exit)
  - Terraform plan output posted as PR comment
- **Acceptance:** All CI-* requirements pass. Workflow YAML is valid.
- **Depends on:** T-005
- **Commit after completion.**

#### T-007: CD Workflow

- **Roadmap:** R-006
- **Agent:** devops-engineer
- **Spec:** specs/cicd-and-workflow.md (CD-01 through CD-03)
- **Work:**
  - Create `.github/workflows/cd.yml`
  - Build backend + frontend, then `terraform apply -auto-approve`
  - No separate deploy scripts
- **Acceptance:** All CD-* requirements pass. Workflow YAML is valid.
- **Depends on:** T-005
- **Commit after completion.**

## Execution Summary

| Layer | Tasks | Agent(s) | Parallel? |
|-------|-------|----------|-----------|
| 0 | T-001 | orchestrator | No |
| 1 | T-002, T-003 | backend-dev, frontend-dev | Yes |
| 2 | T-004 | devops-engineer | No |
| 3 | T-005 | devops-engineer | No |
| 4 | T-006, T-007 | devops-engineer | Yes |

## Notes

- T-006 and T-007 are both devops-engineer. Can be done as one agent session or two sequential tasks within the same session.
- No frontend design skills needed for Phase 0. Brand guidelines (R-008) are a separate OpenSpec change.
- PROJECT_STRUCTURE.md references have been updated: Makefile removed, Taskfile.yml added.
