# module: frontend

## Responsibility

Provisions the public frontend delivery surface: a private S3 bucket for the SPA assets, a CloudFront distribution with two origins (S3 for the SPA, API Gateway for `/api/*`), an Origin Access Control + bucket policy that restricts S3 reads to CloudFront, SPA fallback rules (403/404 → `/index.html`), and the conditional ACM cert / domain-alias wiring.

## Inputs

| Variable | Type | Default | Meaning |
|---|---|---|---|
| `project_name` | string | (required) | Project name prefix; used in bucket name `<project>-<env>-frontend`. |
| `environment` | string | (required) | Deployment environment. |
| `api_gateway_domain` | string | (required) | API Gateway domain (without protocol or stage) — used as the `api-gateway` CloudFront origin. |
| `api_gateway_stage` | string | (required) | API Gateway stage — used as the `origin_path` on the API origin. |
| `domain_aliases` | list(string) | `[]` | Alternate domain names. Only applied when `acm_certificate_arn != ""`. |
| `acm_certificate_arn` | string | `""` | ARN of a `us-east-1` ACM cert covering every `domain_alias`. Empty keeps the default `*.cloudfront.net` cert. |

## Outputs

| Output | Consumer |
|---|---|
| `cloudfront_domain` | Root `outputs.tf` → `cloudfront_url`. |
| `s3_bucket_name` | Root `outputs.tf` → `frontend_bucket_name` → `cd.yml:107` (`aws s3 sync frontend/dist/`). |
| `cloudfront_distribution_id` | Root `outputs.tf` → `frontend_distribution_id` → `cd.yml:112` (`aws cloudfront create-invalidation`). |
## AWS resources created

| Resource | Purpose |
|---|---|
| `aws_s3_bucket.frontend` | Private bucket for the SPA assets. `force_destroy = true` so env renames can succeed. |
| `aws_s3_bucket_public_access_block.frontend` | All four public-access settings = `true`. |
| `aws_cloudfront_origin_access_control.frontend` | OAC: `s3` origin type, `always` signing, `sigv4` protocol — the modern replacement for OAI. |
| `aws_cloudfront_distribution.frontend` | Two origins (S3 + API Gateway), one ordered behavior (`/api/*` → API), default behavior → S3, two SPA-fallback custom error responses (403→/index.html, 404→/index.html), conditional aliases + viewer cert (default `*.cloudfront.net` when no ARN supplied; SNI-only `TLSv1.2_2021` when supplied). |
| `aws_s3_bucket_policy.frontend` | Grants `cloudfront.amazonaws.com` (with `AWS:SourceArn` condition matching this distribution) `s3:GetObject`. |

## Gotchas

- **`aliases` and `viewer_certificate` are conditionally gated on `acm_certificate_arn != ""`.** CloudFront rejects aliases without a matching cert. The empty-string default keeps a first-time apply working on `*.cloudfront.net`. This is the "intermediate state" pattern documented in the variable + resource comments — don't refactor away the conditional.
- **`/api/*` uses the deprecated `forwarded_values` block** instead of a modern managed-policy ID. The default behavior on line 98 uses `cache_policy_id = "658327ea-f89d-4fab-a63d-7e88639e58f6"` (Managed-CachingOptimized) — this asymmetry is called out in `infra/CLAUDE.md` Terraform Review Checklist item 1. Tracked as a P1 follow-up clean-code smell.
- **`/api/*` cookie forwarding is whitelist-only** (`__session`, `__client_uat`). Adding unrelated cookies would poison CloudFront's cache key for any future cacheable `/api/*` response. Today the TTLs are all 0 so caching is off, but the whitelist is defense in depth.
- **SPA fallback maps 403 + 404 → 200 / `/index.html`.** This is what makes client-side routes like `/admin/curate` work. Don't remove without coordinating with the frontend router.
- **No `response_headers_policy_id` attached** — issue #114. Missing HSTS / X-Content-Type-Options / CSP at the edge.
- **No tags on any resource in this module.** Inconsistent with the rest of the codebase (api/database/generation/daily-cron all tag). Tracked as a follow-up consistency fix.
- **The CloudFront distribution's `comment` is `"<project> <env> frontend"`** — fine for ops UX. Don't put PII or runtime data there.
