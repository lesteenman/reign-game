# Production Launch

How to take the prod environment (`reign.steenman.me`) from "capability exists" to "live and serving real users."

Issue #132 shipped the **capability**: the `infra/envs/prod/` Terraform root, the manual `cd-prod.yml` workflow, and the `prod` GitHub Environment (required-reviewer protection). The first real apply was **deliberately deferred** to this runbook, because it depends on out-of-band prereqs (a cert, a Clerk prod tenant, DNS, populated secrets, and a deploy-role trust update) that must exist before the pipeline can succeed.

Work top to bottom. Steps 1–6 are prereqs; step 7 is the first apply; step 8 verifies. Until the prereqs are done, a `cd-prod` dispatch fails cleanly (empty `AWS_ROLE_ARN` → `configure-aws-credentials` errors) rather than half-applying.

## Account model recap

- prod resources are `reign-game-prod-*` in the **same AWS account** as acc, isolated by name + the `reign-game/prod` state prefix.
- The deploy role's **permissions** already cover every resource type prod uses (same module set as acc). Only the role's **trust policy** needs the prod OIDC subject (step 5).
- The ACM cert + DNS for `reign.steenman.me` are cross-account and live in the **separate** `accounts/reign-game` Terraform project (same split as acc — see `custom-domain.md`).

---

## 1. Provision the ACM certificate (accounts/reign-game)

The cert must be in **us-east-1** (CloudFront only reads ACM there). Follow `custom-domain.md` steps 1–3 for `reign.steenman.me`:

1. In `accounts/reign-game`, request/extend an ACM cert covering `reign.steenman.me` (a new cert, or add the SAN to the existing one). DNS validation.
2. Create the validation CNAME at the DNS host (TransIP). Wait for status `ISSUED`.
3. Record the **full cert ARN** (e.g. `arn:aws:acm:us-east-1:236320489345:certificate/...`). It feeds step 4 (`ACM_CERTIFICATE_ARN`).

**Do not proceed to the apply until the cert is `ISSUED`** — the CloudFront apply fails otherwise.

## 2. Stand up the Clerk prod tenant

prod users must NOT share acc's Clerk tenant. Create a **separate prod Clerk instance** (production tier — `pk_live_*` / `sk_live_*`):

1. Clerk dashboard → create the production instance for the prod app.
2. Configure Google OAuth (the project's sign-in method) on the prod instance.
3. Set `publicMetadata.role = "admin"` on any admin users (same Role model as acc — see `/CLAUDE.md` Roles).
4. Add the prod origin to **Allowed Origins**: `https://reign.steenman.me` (do this now or in step 8 — without it sign-in throws an origin error).
5. Record `pk_live_*` (publishable, browser-safe) and `sk_live_*` (secret) for step 4.

## 3. DNS for reign.steenman.me

The user-facing CNAME points at the prod CloudFront distribution. The distribution domain isn't known until the first apply (step 7) creates it, so split this:

- The **cert validation** CNAME is step 1.
- The **user-facing** CNAME (`reign.steenman.me CNAME dXXX.cloudfront.net.`) is created **after** step 7 — use `terraform -chdir=infra/envs/prod output -raw cloudfront_url` for the target. TTL 300 while testing, raise to 3600 once stable.

## 4. Populate the prod GitHub Environment

The `prod` Environment already exists with a required-reviewer rule (created in #132); its secrets/vars are **empty**. Fill them — Settings → Environments → `prod`:

**Secrets** (Environment-scoped):

| Name | Value |
|---|---|
| `AWS_ROLE_ARN` | The `github-actions-deploy` role ARN (same role acc uses — the trust update in step 5 is what lets prod assume it). |
| `TF_VAR_TERRAFORM_STATE_BUCKET` | The S3 state bucket (same bucket as acc; isolation is by **key** `reign-game/prod`, not bucket). |
| `CLERK_PUBLISHABLE_KEY` | `pk_live_*` from step 2 (initial-apply-only seed into SSM; rotate later in SSM, not here — see `infra/CLAUDE.md` lesson 7). |
| `CLERK_SECRET_KEY` | `sk_live_*` from step 2 (same initial-apply-only caveat). |

**Variables** (Environment-scoped):

| Name | Value |
|---|---|
| `AWS_REGION` | `eu-west-1` (match acc). |
| `TF_VAR_TERRAFORM_STATE_PREFIX` | `reign-game/prod` (the backend state key prefix — MUST be `reign-game/prod`, never `reign-game/acc`, or prod would clobber acc state). |
| `ACM_CERTIFICATE_ARN` | The us-east-1 cert ARN from step 1. |

Notes:
- `DOMAIN_ALIASES` is **not** an Environment var for prod — the prod domain alias (`["reign.steenman.me"]`) is committed in `infra/envs/prod/terraform.tfvars` (public, fixed config). `cd-prod.yml` does not pass `TF_VAR_domain_aliases`, so there is a single source of truth and no phantom-diff hazard.
- Names mirror acc's Environment set so prod is a pure addition (per #241's design).

## 5. Add the prod OIDC subject to the deploy-role trust policy

`environment: prod` makes the OIDC token `sub` resolve to `repo:lesteenman/reign-game:environment:prod`. The `github-actions-deploy` role's **trust policy** (bootstrapped **outside this repo** — not in `infra/`) must allow that subject, or `configure-aws-credentials` fails.

- Add `repo:lesteenman/reign-game:environment:prod` to the role's trust-policy `sub` condition (alongside the existing `...:environment:acc` and any CI subject).
- This is the **only** deploy-role change prod needs — its permission policy already covers prod's resource types.
- **Do not** make this change in this repo (the trust policy isn't here). Make it where the role is bootstrapped.

## 6. Pre-flight check

- Cert `ISSUED` (step 1).
- Clerk prod instance live, origin allowed (step 2).
- All prod Environment secrets + vars populated (step 4).
- Deploy-role trust policy includes the prod subject (step 5).
- Decide the `ref` to promote — a known-good `main` SHA or a release tag (default input is `main` HEAD).

## 7. First apply (workflow_dispatch)

1. GitHub → Actions → **CD Prod** → **Run workflow**.
2. Set the `ref` input to the commit/tag to promote (default `main`).
3. Run. The job pauses on the **required-reviewer gate** — approve it (the approver is the prod Environment reviewer set in #132).
4. The job builds the Lambdas, `terraform init` against `reign-game/prod`, `terraform apply`, fetches the Clerk publishable key + API key from SSM/TF, builds + syncs the frontend, invalidates CloudFront, then runs post-deploy verification.

First-apply specifics:
- prod has **no `imports.tf`** (its Lambda log groups have never existed) — the apply creates everything fresh. This is expected; acc's import blocks are an acc-only artifact of #162.
- After the apply succeeds, create the user-facing DNS CNAME (step 3) using `terraform -chdir=infra/envs/prod output -raw cloudfront_url`.

## 8. Verify end-to-end

After CD Prod succeeds AND DNS has propagated, open `https://reign.steenman.me`:

- Padlock green (cert matches).
- Landing page renders.
- DevTools → Network: `/api/health` returns 200; a real `/api/*` call returns < 400 (same-origin).
- Sign-in works against the **prod** Clerk tenant (no origin/CORS error in console).
- No same-origin console/page errors.

The `cd-prod` post-deploy verification step (`frontend/scripts/verify-deploy.mjs https://reign.steenman.me`) automates most of this and gates the prod Deployment status. CI green is not CD green — do the manual browser check too (`infra/CLAUDE.md` → Verify Before Reporting Done).

## Rollback

- To revert the domain to the bare CloudFront URL: this would require editing `infra/envs/prod/terraform.tfvars` (`domain_aliases = []`) and clearing `ACM_CERTIFICATE_ARN`, then re-running CD Prod. Prefer fixing forward.
- To roll back the app: dispatch CD Prod again with an earlier known-good `ref`.
- The cert in `accounts/reign-game` is cheap and re-attachable — leave it.

## Ongoing

- prod monitoring composes at go-live (#133's monitoring module is env-agnostic; add it to `infra/envs/prod/main.tf` when prod monitoring is wanted).
- Rotations of Clerk keys happen in **SSM** (source of truth), not in the GitHub Environment — see `infra/CLAUDE.md` lesson 7 and `admin-auth-setup.md`.
