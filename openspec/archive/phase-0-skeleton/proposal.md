# Phase 0: Skeleton + Deploy Pipeline

## What

Monorepo scaffold with a Go backend (health check Lambda), React 19 frontend (placeholder), Terraform infrastructure (S3 + CloudFront, API Gateway + Lambda), GitHub Actions CI/CD, and a Taskfile for local dev. No game logic. The empty app runs live on AWS.

## Why

Every future phase depends on a working build, test, and deploy pipeline. Shipping the skeleton first means Phase 1 starts with infrastructure solved, not a blank repo.

## Scope

- **R-001** — Monorepo directory structure (frontend/, backend/, infra/, design/)
- **R-002** — Go 1.26 module with chi router, single Lambda, health check endpoint
- **R-003** — React 19 + Vite 8 + TypeScript 6 + Tailwind 4.2.2 + Vitest scaffold
- **R-004** — Terraform 1.14.8: S3 + CloudFront module, API Gateway + Lambda module
- **R-005** — GitHub Actions CI: parallel jobs for backend, frontend, Terraform plan, security
- **R-006** — GitHub Actions CD: Terraform apply on merge to main
- **R-007** — Taskfile at repo root for local dev workflow

## Not in Scope

- Game logic, puzzle data model, or any gameplay code
- DynamoDB (Phase 4)
- Auth provider (Phase 5+)
- Custom domains (deferred — default CloudFront and API Gateway URLs)
- Playwright e2e tests (Phase 1)
- React Router (Phase 1)
- Brand guidelines — separate OpenSpec change (R-008, ui-ux-pro-max skill)

## References

- ROADMAP.md: R-001 through R-007
- design-grill-summary.md (this directory)
- GAME_DESIGN.md: Technical Architecture section
- PROJECT_STRUCTURE.md: canonical directory layout
