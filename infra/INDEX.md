# infra/ — Index

Phase 0 read-only sweep — Track 3 cleanup of the Reign puzzle-game codebase.
Snapshot as of branch `fix/ci-plan-tf-vars` (2026-05-15).

## What this subproject is

Terraform code that provisions every AWS resource the Reign game runs on:

| Service | Purpose |
|---|---|
| S3 | Frontend static assets (`reign-game-<env>-frontend` bucket, private) |
| CloudFront | CDN in front of S3 + reverse proxy to API Gateway for `/api/*` |
| API Gateway (REST) | Public HTTPS entry to the backend Lambda; ANY on `/api/{proxy+}` |
| Lambda (3 functions) | `api` (HTTP handler), `generator` (SQS consumer), `daily-cron` (EventBridge target) |
| SQS | `puzzle-generation` queue + DLQ for async puzzle generation |
| DynamoDB | Single-table `puzzle-pool` (PK/SK), PAY_PER_REQUEST |
| EventBridge | Two daily cron rules (T-6h ensure 18:00 UTC, T=0 finalize 00:00 UTC) |
| SSM Parameter Store | Clerk publishable + secret keys, KMS-encrypted via `aws/ssm` |
| KMS | AWS-managed `aws/ssm` key only (no customer-managed keys) |

There is NO VPC, NO RDS, NO ECS — pure serverless.

## Environment story

- Two thin per-env roots over shared `modules/*`: **acc** (`infra/envs/acc/`) and **prod** (`infra/envs/prod/`). Each root calls `../../modules/*`, owns its own backend state key, and pins its env identity in a committed `terraform.tfvars` (`environment` + the public domain alias — no secrets).
  - **acc** (state key `reign-game/acc`) serves the live acceptance domain `reign.acc.steenman.me` (Phase 8, 2026-05-08). Auto-deploys on merge to main via `cd.yml`.
  - **prod** (state key `reign-game/prod`) serves `reign.steenman.me`. Capability only — the pipeline exists (`cd-prod.yml`, manual `workflow_dispatch`) but the first real apply is a runbook step (`docs/runbooks/prod-launch.md`); the prod Environment secrets/cert/Clerk/DNS are go-live prereqs (#132).
- Per-env isolation is by construction: separate state keys + env-suffixed resource names (`reign-game-{acc,prod}-*`) in the **same AWS account**. Cross-account stays only for DNS/cert.
- The module block addresses (`module.database`, `module.generation`, `module.api`, `module.frontend`, `module.daily_cron`) are identical across both roots and unchanged from the previous flat root, so the acc state rebound with zero resource churn during the #132 restructure.
- ACM cert + DNS for the custom domain live in a **separate Terraform project** (`accounts/reign-game`) because they cross AWS-account boundaries. This repo only references the ARN via `var.acm_certificate_arn`.

## File tree

```
infra/
├── INDEX.md                  ← this file (Phase 0 deliverable)
├── CLAUDE.md                 ← infra-specific conventions (cost, security, CI/CD symmetry)
├── .gitignore                ← excludes .terraform/, *.tfstate; un-ignores envs/*/terraform.tfvars
├── envs/
│   ├── acc/                  ← thin acc root (state key reign-game/acc)
│   │   ├── main.tf           ← module composition (database → generation → api → frontend → daily_cron); sources ../../modules/*
│   │   ├── variables.tf      ← root variables (aws_region, project_name, environment, lambda zip paths, Clerk keys, domain aliases, ACM cert ARN)
│   │   ├── outputs.tf        ← root outputs (cloudfront_url, api_gateway_url, frontend_bucket_name, frontend_distribution_id, puzzle_table_name, sqs_queue_url, generator_function_name, clerk_publishable_key_param_name, client_api_key_value)
│   │   ├── versions.tf       ← terraform >= 1.14, AWS provider ~> 5.0
│   │   ├── backend.tf        ← S3 backend with placeholders (real values via -backend-config in CI/CD)
│   │   └── terraform.tfvars  ← committed env identity (environment="acc")
│   └── prod/                 ← thin prod root (state key reign-game/prod); same structure
│       └── terraform.tfvars  ← committed env identity (environment="prod", domain_aliases=["reign.steenman.me"])
└── modules/
    ├── api/                  ← REST API: Lambda + API Gateway + IAM + Clerk SSM params  (see modules/api/README.md)
    ├── database/             ← Single DynamoDB table `puzzle-pool` (PK/SK, on-demand)  (see modules/database/README.md)
    ├── daily-cron/           ← Daily-puzzle EventBridge Lambda (T-6h ensure + T=0 finalize)  (see modules/daily-cron/README.md)
    ├── frontend/             ← S3 + CloudFront + OAC + bucket policy  (see modules/frontend/README.md)
    ├── generation/           ← Generator Lambda + SQS queue + DLQ + event source mapping  (see modules/generation/README.md)
    └── monitoring/           ← CloudWatch dashboard + alarms + SNS alerts topic (no subscriber; see docs/runbooks/monitoring.md)
```

### Workflows that consume this infra

```
.github/workflows/
├── ci.yml                    ← PR: backend test/lint, frontend test, integration + e2e Playwright, terraform validate (envs/prod) + plan (envs/acc, CI_ creds) + recursive fmt-check, security (gitleaks/govulncheck/npm audit). All actions SHA-pinned (#113).
├── cd.yml                    ← merge to main: build Lambdas → terraform apply (envs/acc) → fetch publishable key from SSM → build frontend with VITE_CLERK_PUBLISHABLE_KEY → sync to S3 → invalidate CloudFront
├── cd-prod.yml               ← manual workflow_dispatch (ref input) → terraform apply (envs/prod) under the prod Environment (required reviewer). Mirrors cd.yml; never auto-runs. First apply = runbook (docs/runbooks/prod-launch.md).
└── generator-check.yml       ← non-blocking soak + property tests on PRs touching backend/internal/generator/**
```

### Local + e2e supporting files

```
Taskfile.yml                  ← task runner; dev:*/e2e:* lifecycle (LocalStack + 2 backends + 2 generators + 2 Vite servers)
docker-compose.yml            ← LocalStack pinned to 4.14.0; HOST_REPO_PATH var for devcontainer
.localstack/init-aws.sh       ← idempotent: creates puzzle-pool + puzzle-pool-e2e tables, queue pairs (dev + e2e), seeds CONFIG rows
```

## Top-level composition (envs/*/main.tf)

Each env root wires five modules together, passing outputs through as inputs (identical composition in acc and prod — only the env identity + per-env tfvars differ):

```
database  (puzzle_table_name, puzzle_table_arn)
   ↓
generation  (queue_url, queue_arn)         ←┐
   ↓                                        │
api  (api_gateway_domain, api_gateway_stage)│
   ↓                                        │
frontend                                    │
                                            │
daily_cron  ←─── puzzle_table_*  + generation_queue_* (independent of api + frontend)

monitoring  ←─── resource identifiers from database/generation/api/daily_cron/frontend
                 (function names, api name+stage, queue+dlq names, table name, distribution id)
```

No module references another module — composition happens only at the root. This matches the architecture rule in `.claude/skills/architecture/SKILL.md` (modules-must-not-reference-modules).

## CI/CD ↔ infra contract

CD reads three outputs from `terraform output`:
- `clerk_publishable_key_param_name` → fetch publishable key from SSM → bake into VITE bundle
- `frontend_bucket_name` → `aws s3 sync frontend/dist/`
- `frontend_distribution_id` → `aws cloudfront create-invalidation`

CI plan passes the same `TF_VAR_*` set as CD apply: `clerk_publishable_key`, `clerk_secret_key`, `domain_aliases`, `acm_certificate_arn`. Issue #155 fixed by the current branch (`fix/ci-plan-tf-vars`); no other asymmetries.

## Per-module READMEs

- [`modules/api/README.md`](modules/api/README.md) — REST API surface, Clerk SSM, lifecycle ignore_changes gotchas
- [`modules/database/README.md`](modules/database/README.md) — single-table DynamoDB layout
- [`modules/daily-cron/README.md`](modules/daily-cron/README.md) — EventBridge cron + reactive SQS publish
- [`modules/frontend/README.md`](modules/frontend/README.md) — S3 + CloudFront + OAC; SPA fallbacks, /api/* caching, domain-aliases conditional
- [`modules/generation/README.md`](modules/generation/README.md) — Generator Lambda + SQS + DLQ + event source mapping

## Key references

- `infra/CLAUDE.md` — per-subproject conventions (cost, security, CI/CD symmetry, Terraform + CI/CD + dev-tooling checklists)
- `.claude/skills/architecture/SKILL.md` — canonical layered rules (modules-vs-envs, CI/CD symmetry)
- Per-module findings (cost smells, deprecated patterns, dead outputs) are documented inline in each module's `README.md`.
