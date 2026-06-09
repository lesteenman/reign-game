# Infrastructure — Terraform + AWS Conventions

This file is auto-loaded by Claude Code when working on files under `infra/`. The project-wide rules live in `/CLAUDE.md`; this file is additive.

## Layered Structure

The `architecture` skill enforces these rules at design time and review time.

| Layer | Directory | Rule |
|---|---|---|
| **Modules** | `infra/modules/*/` | Self-contained, reusable. **Must NOT reference each other directly.** Composition happens at the env level. |
| **Environments / root** | `infra/` (root), `infra/envs/` (future) | Calls modules and wires them together. **Must NOT define inline resources that belong in a module** (a stray `resource "aws_lambda_function" "..."` at root is a smell). |

Drift detection: `grep -rn 'module\.' infra/modules/` must return nothing. Any module referencing another module is a violation.

## Cost Optimization (CRITICAL)

This is a serverless-first project targeting minimal cost at low traffic.

- **Lambda**: minimize memory allocation, optimize cold starts (Go binary size).
- **DynamoDB**: on-demand billing, efficient key design to avoid hot partitions.
- **S3 + CloudFront**: appropriate cache policies to minimize origin requests.
- **No always-on resources** unless absolutely necessary.
- **Tag all resources** for cost tracking.

## Security

- **Least-privilege IAM policies.** Each Lambda gets only the permissions it needs. No wildcard `Action: "*"` or `Resource: "*"` unless thoroughly justified.
- **No hardcoded secrets** — use SSM Parameter Store (with `aws/ssm` KMS) or Secrets Manager.
- **Encryption at rest** explicitly configured (DynamoDB and S3 — even though AWS defaults to SSE-S3, compliance requires explicit config; see issue #115).
- **CloudFront HTTPS-only**, with security response headers (HSTS, X-Content-Type-Options, CSP — see issue #114).
- **API Gateway**: throttling and request validation.

## CI/CD Symmetry (CRITICAL)

CI's `terraform plan` and CD's `terraform apply` must pass the SAME variable set. Mismatched vars cause phantom plan diffs that scare reviewers and mask real changes. The 5-day silent-CD incident (PRs #102/103/104, 2026-05-08) and the alias-removal phantom on PR #153 both traced to this. **Always check `.github/workflows/cd.yml` and `.github/workflows/ci.yml` together when adding a new `TF_VAR_*` or `-var=`.**

## Terraform Review Checklist

Before committing Terraform code, verify:

1. **CloudFront uses managed cache policies** (e.g., `CachingOptimized`), not deprecated `forwarded_values` blocks.
2. **API Gateway stages have throttling configured** via `aws_api_gateway_method_settings`.
3. **`terraform init` in CI/CD workflows includes `-backend-config` flags** for the S3 state backend.
4. **GitHub Actions that reference `terraform plan` with `continue-on-error: true`** also have a subsequent step that fails explicitly on plan error.
5. **Any `file*()` function** (e.g., `filebase64sha256`) taking a variable path must be wrapped in a `fileexists()` guard — CI plan passes dummy paths that don't exist on disk.
6. **Never use `count` with module outputs.** `count = var.x != "" ? 1 : 0` fails when the variable comes from another module's output that isn't known until apply. Either make the resource unconditional or use `for_each` with a known set. Phase 3 CI failed with `Invalid count argument` on two IAM policies.
7. **`lifecycle { ignore_changes = [value] }` makes the Terraform-managed resource the source of truth — input variables become initial-only.** Once such a resource exists, future `terraform apply` runs will not overwrite the value field even if the upstream input variable changes. Rotating the upstream CI secret (e.g. a GitHub `secrets.X` that flows in as `TF_VAR_x`) has zero effect on the deployed value. To rotate: update the resource directly out of band (`aws ssm put-parameter --overwrite`, `aws secretsmanager update-secret`, etc.) and re-trigger any consumer that reads it. Update the CI secret too, but only so the *next* clean-room initial-apply matches. Phase 6 lost ~one CD rerun and ~10 min cognitive overhead because `gh secret set CLERK_PUBLISHABLE_KEY` was assumed to flow into the next build — it didn't, because `/reign/prod/clerk-publishable-key` had `ignore_changes = [value]` and the CD reads from SSM, not directly from the GitHub secret. The runbook (`docs/runbooks/admin-auth-setup.md` §8) already documented the SSM-as-source-of-truth flow; the lesson is to make this the *default* mental model for any Terraform-managed value with `ignore_changes`.
8. **A new AWS *resource type* may need a new action on the deploy role — which is bootstrapped OUTSIDE this repo.** The `github-actions-deploy` role's permission policy is not in `infra/`, so adding a resource type the role has never created blocks the CD apply with `AccessDenied` (and, since the resource is now in `main`, blocks *every* subsequent deploy until the role is fixed). #163's `aws_cloudfront_origin_request_policy` failed on `cloudfront:CreateOriginRequestPolicy`. When a PR introduces a resource type not already used elsewhere in `infra/`, flag that the external deploy role likely needs the matching `Create/Get/Update/Delete/List` actions, and hold the PR for the supervisor to grant them before merge. **File the grant as an issue on `lesteenman/aws-personal`** (the repo that owns the role — `accounts/root/github-actions-oidc.tf`) with the exact policy statement + a link to the reign PR, then link that issue back from the PR and hold until it's granted. The same convention covers **OIDC trust subjects**: a new GitHub Environment (e.g. prod's `repo:lesteenman/reign-game:environment:prod`) needs its `sub` added to the deploy-role trust policy in `aws-personal` — file that on `lesteenman/aws-personal` too. (Both are out-of-repo changes; filing them as issues there is how they get picked up, rather than living only as a reign PR flag.)

## CI/CD Cross-Validation Checklist

Before merging CI or CD workflow changes, verify:

1. **Every `terraform output` referenced in CD steps** (e.g., `output -raw frontend_bucket_name`) exists in root `outputs.tf` and is wired to a module output.
2. **Every `secrets.*` and `vars.*` reference matches** what is actually configured in GitHub (secret vs variable namespace matters — `secrets.X` returns empty if X is a variable).
3. **CD reads deploy credentials from the `acc` Environment; CI reads `CI_`-prefixed repo secrets/vars** — deliberately *separate* namespaces as of #241 (see Deployment Environments below). A CI job references `CI_<NAME>`; the CD deploy job references the bare `<NAME>`, resolved from its environment. (This reverses the older "same namespace" rule so the deploy target and CI's credentials can diverge — e.g. when prod lands.)
4. **Action versions are consistent** across CI and CD workflows.
5. **Production request flow traced end-to-end before shipping a deploy PR.** Walk every URL the frontend calls and confirm it reaches the correct backend in production. Check CloudFront behaviors, API Gateway routes, and proxy config. Dev proxies mask routing issues that break in production. When adding new HTTP methods (PUT, POST) or new path prefixes (`/admin`), verify the API Gateway Terraform has matching method + integration resources — the dev proxy masks missing routes.
6. **`go test` callers pass `-short` where appropriate.** `testing.Short()` skips in Go tests are a contract with the caller. When a test is guarded by `if testing.Short() { t.Skip(...) }`, every place that runs `go test` must pass `-short` unless it's specifically meant to run the full gate: the pre-push hook, the Taskfile, any CI workflow that isn't a dedicated soak/long-running job. A skip with no caller passing `-short` is dead code and the guarded test silently runs in every context that was supposed to be fast. Phase 5's CI ran without `-short` and blew the 10-min timeout because nothing was passing `-short` into `ci.yml`'s backend step.
7. **Hook wall-time is a diagnostic signal, not noise.** A local pre-push that takes more than ~2 minutes is telling you something is wrong in the suite — most likely a `testing.Short()`-guarded test isn't being skipped. When a hook or CI step runs unexpectedly long, diagnose the *slow step* before retrying the outer command.
8. **Pre-push hook's optional-tool checks fail loudly, not silently.** `.githooks/pre-push` exits with an error message + install instruction when any of `golangci-lint`, `terraform`, or `gitleaks` is missing on PATH. Previously these were `WARN: ... not found, skipping ...` lines that quietly let lint-broken / unformatted / secret-bearing commits ship to CI. The contract: if the hook tells you a tool is missing, install it — don't comment the check out. CI's pinned versions (`golangci-lint v2.11.4` per `.github/workflows/ci.yml`) are the install target.

## Deployment Environments (acc) — #241

- The CD `deploy` job runs under the **`acc` GitHub Environment** (`environment: { name: acc, url: https://reign.acc.steenman.me }`). This (a) resolves the deploy secrets/vars from the environment and (b) makes GitHub **auto-create a Deployment record + status** for acc (Environments view / PR timeline) — no script needed, because the verification step is the job's last step.
- **CI and CD credentials are modelled as separate sets, even though their values coincide today** (the coincidence is incidental — keeping them separate lets prod diverge cleanly):
  - **`acc` Environment** holds the clean, full deploy set: secrets `AWS_ROLE_ARN`, `TF_VAR_TERRAFORM_STATE_BUCKET`, `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`; vars `AWS_REGION`, `TF_VAR_TERRAFORM_STATE_PREFIX`, `DOMAIN_ALIASES`, `ACM_CERTIFICATE_ARN`.
  - **CI** reads `CI_`-prefixed **repo-level** copies (CI isn't a deployment, so it gets no Environment — that would log every PR run as environment activity). CI needs the full set because CI's `terraform plan` must pass the same vars as CD's apply (CI/CD Symmetry).
  - Cost: ~4 secrets + ~4 vars exist in two places → **two rotation sites**. Deliberate, so prod (#132) is a pure addition. `E2E_CLERK_TEST_*`, `GITLEAKS_LICENSE`, `GITHUB_TOKEN` stay plain repo-level (CI-only / auto).
- **OIDC:** `environment: acc` changes the OIDC token `sub` to `repo:lesteenman/reign-game:environment:acc`. The deploy role's trust policy (bootstrapped outside this repo) **must allow that subject**, or `configure-aws-credentials` fails.
- **Post-deploy verification gate:** the deploy job's last step runs `frontend/scripts/verify-deploy.mjs` against the live site — 6 security headers + `/api/health` 200 + a real same-origin `/api/*` response `<400` (the #225 stale-bundle catch) + no **same-origin** console/page errors. A failure fails the job → the Deployment auto-marks `failure`. Third-party CDN failures (e.g. Google Fonts from the runner) are ignored — they're network-dependent, not deploy defects.

## Dev & E2E Tooling Checklist

Before changes to `docker-compose.yml`, `Taskfile.yml` (e2e blocks), `.localstack/init-aws.sh`, or e2e fixture seeding:

1. **Docker images pin to a specific stable tag, never `:latest`.** Auto-update on `docker compose pull` is silent and untraceable; treat `:latest` like an unlocked dependency. Document the bump deliberately in the file (one-line comment naming the version + reason). This rule was paid for when `localstack/localstack:latest` pulled a build with broken SQS `SendMessage` and an empty exception body; ~1 hour to diagnose because the symptom didn't point at the image tag. Pinning to `localstack/localstack:4.14.0` resolved it instantly.
2. **E2E environments must seed every operational state the prod runtime would have populated, not just the source-of-truth rows.** Approved-pool fixtures alone aren't enough when scheduled-state rows (candidate slots, schedule rows, leaderboard partitions) are normally written by crons or workers that don't run in e2e. Two CI cycles burned because a daily-flow happy-path test had a 9×9 fixture but no `DAILY-CANDIDATE` row pre-seeded — the sync fallback returned `ErrPoolExhausted` and the endpoint 500'd. Walk every code path that reads from a row family and ensure `task e2e:seed` populates not just the source-of-truth rows but the *intermediate* rows that crons / workers / EventBridge would normally write. The seed recipe is the right home for one-shot pre-seeds, not the LocalStack init script (which is wiped by per-run `flush-pool.sh`).

## Verify Before Reporting Done

1. `terraform validate` passes
2. `terraform plan` shows expected changes (no surprises)
3. GitHub Actions workflows have correct syntax (use actionlint if available)
4. IAM policies follow least privilege
5. No hardcoded secrets in any file
6. All items in the Terraform Review Checklist above are satisfied
7. **Open the deployed URL after CD succeeds and verify the user-visible feature actually works** — for any slice that touches build-time env injection, infrastructure, CDN, auth, or any other path where local dev diverges from prod. CI green means "the build compiled and tests passed against local stubs," not "the running site works." Local dev paths often use different env vars, different secret sources, or different DNS than prod. Run a headless playwright check against the deployed origin or load it in a browser; look for console errors, failed network requests, and the visible UI element the slice was supposed to ship. The admin-auth Phase 6 slice declared "done" all-CI-green, but the deployed page was broken from day one — Clerk JS couldn't load because the `pk_live_*` embedded an unreachable Frontend API host. Caught manually 24 h+ later. A 60-second post-deploy check would have caught it inside the same slice.

## Human-in-the-Loop

Always confirm before:
- Applying Terraform changes to production
- Modifying IAM policies
- Changing domain/DNS configuration
- Any action that incurs recurring AWS costs
