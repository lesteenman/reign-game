# Phase 0 Skeleton + Deploy Pipeline: Design Decisions

Phase 0 delivers an empty app live on AWS with CI/CD working. No game logic. One OpenSpec change covers R-001 through R-007. R-008 (brand guidelines) is a separate change using the ui-ux-pro-max skill.

## Decisions

### Backend Scaffold

**Go 1.26 with chi v5.2.5 router.** Single Lambda function behind API Gateway. The chi router handles all routes via aws-lambda-go-api-proxy (v1.54.0). For local dev, the same code starts a plain HTTP server. Phase 0 ships one handler: `GET /health` returning `{"status": "ok"}`.

**Zip deployment.** Go compiles to a static binary. No container image, no ECR. Terraform manages the zip upload.

### Frontend Scaffold

**React 19, Vite 8, TypeScript 6, Tailwind 4.2.2, Node 24 LTS.** Minimal scaffold — placeholder App.tsx, Vitest config with a dummy test, PWA manifest stub. No React Router, no Playwright, no components. Phase 1 is where the frontend gets real.

**Vitest now, Playwright Phase 1.** Vitest is cheap to configure and lets CI run `npm test` from day one. Playwright needs real UI to test.

### Infrastructure

**Terraform 1.14.8.** Two modules: `frontend/` (S3 + CloudFront with OAC) and `api/` (API Gateway + Lambda + IAM). State backend is a pre-existing S3 bucket referenced via variables. No custom domains — CloudFront and API Gateway default URLs for now.

**No DynamoDB until Phase 4.** No auth module until Phase 5+.

### CI/CD

**GitHub Actions from day one.** Two workflows:

CI (on PR): parallel jobs for backend (build, test, golangci-lint), frontend (build, test), Terraform plan (output posted as PR comment), and security (gitleaks, govulncheck, npm audit). All jobs run to completion — no early exit on failure.

CD (on merge to main): `terraform apply` manages everything — Lambda code, S3 frontend sync, CloudFront invalidation. No separate deploy scripts. If state drifts from code, the deploy corrects it.

**No AWS specifics in the repo.** Account, role, and domain values injected via GitHub secrets/variables.

### Dev Workflow

**Taskfile at repo root.** YAML-based task runner. Targets for building, testing, running dev servers, and deploying. Readable syntax, dependency tracking, cross-platform.

### Ordering

R-001 (monorepo dirs) → R-002 + R-003 in parallel (backend + frontend scaffolds) → R-007 (dev workflow / Taskfile) → R-004 (Terraform) → R-005 + R-006 in parallel (CI + CD workflows).

## Deferred

- **Custom domains**: added when public-facing URLs matter (Phase 4/5)
- **Playwright**: added in Phase 1 when there's UI to test
- **React Router**: added in Phase 1 when there are multiple pages
- **DynamoDB module**: Phase 4 with curation pipeline
- **Auth module**: Phase 5+ when accounts exist

## Assumptions

- The pre-existing S3 state bucket is available and the deploying IAM role has access to it.
- GitHub Actions runners have AWS credentials configured via OIDC or secrets.
- `provided.al2023` Lambda runtime is used for Go custom runtimes.
- Tailwind 4 CSS-first configuration (no `tailwind.config.ts`).
