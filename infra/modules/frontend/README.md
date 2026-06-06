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
| `tags` | map(string) | `{}` | Applied to every **taggable** resource in this module (the S3 bucket + the CloudFront distribution). |

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
| `aws_cloudfront_distribution.frontend` | Two origins (S3 + API Gateway), three ordered behaviors (`/api/*` → API; `/sw.js` + `/workbox-*.js` → S3 with `CachingDisabled`), default behavior → S3, two SPA-fallback custom error responses (403→/index.html, 404→/index.html), conditional aliases + viewer cert (default `*.cloudfront.net` when no ARN supplied; SNI-only `TLSv1.2_2021` when supplied). |
| `aws_s3_bucket_policy.frontend` | Grants `cloudfront.amazonaws.com` (with `AWS:SourceArn` condition matching this distribution) `s3:GetObject`. |

## Gotchas

- **`aliases` and `viewer_certificate` are conditionally gated on `acm_certificate_arn != ""`.** CloudFront rejects aliases without a matching cert. The empty-string default keeps a first-time apply working on `*.cloudfront.net`. This is the "intermediate state" pattern documented in the variable + resource comments — don't refactor away the conditional.
- **`/api/*` uses managed policies** — the AWS managed `CachingDisabled` cache policy plus a custom `aws_cloudfront_origin_request_policy.api`. The forward whitelist is narrow on purpose: headers `Authorization` + `x-api-key`, cookies `__session` + `__client_uat`, all query strings. `Host` is deliberately excluded — a forwarded `Host` would not match the API Gateway custom origin and would 403. A managed all-viewer policy would forward both, so the custom policy stays. (`aws_cloudfront_origin_request_policy` has no `tags` argument in the provider — see the tags gotcha.)
- **SPA fallback maps 403 + 404 → 200 / `/index.html`.** This is what makes client-side routes like `/admin/curate` work. Don't remove without coordinating with the frontend router.
- **No `response_headers_policy_id` attached** — issue #114. Missing HSTS / X-Content-Type-Options / CSP at the edge.
- **`var.tags` applies only to the two taggable resources** — `aws_s3_bucket.frontend` and `aws_cloudfront_distribution.frontend`. The rest of the module's resources have no `tags` argument in the AWS provider and reject one (plan error): `aws_cloudfront_origin_access_control`, `aws_cloudfront_response_headers_policy`, `aws_cloudfront_origin_request_policy`, `aws_s3_bucket_server_side_encryption_configuration`, `aws_s3_bucket_policy`, `aws_s3_bucket_public_access_block`.
- **Two ordered cache behaviors target `/sw.js` and `/workbox-*.js`** with the AWS managed `CachingDisabled` policy (`4135ea2d-6df8-44a3-9df3-4b5a84be39ad`) so service-worker updates propagate without waiting for edge TTL. Added in #116.
- **The CloudFront distribution's `comment` is `"<project> <env> frontend"`** — fine for ops UX. Don't put PII or runtime data there.
