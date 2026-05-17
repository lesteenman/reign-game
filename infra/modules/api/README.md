# module: api

## Responsibility

Provisions the public HTTPS API: a single Go Lambda (`cmd/api`), a REST API Gateway with `ANY /api/{proxy+}` integration, all IAM policies the Lambda needs (CloudWatch Logs, SQS publish, DynamoDB access, SSM read, KMS decrypt), and two SSM parameters holding the Clerk publishable + secret keys. The Lambda receives DynamoDB + SQS coordinates as env vars and the SSM parameter name (not the secret itself) for the Clerk secret.

## Inputs

| Variable | Type | Default | Meaning |
|---|---|---|---|
| `project_name` | string | (required) | Project name prefix. |
| `environment` | string | (required) | Deployment environment. |
| `lambda_zip_path` | string | (required) | Path on disk to `cmd/api`'s reproducible zip (built by `task build:lambda`). |
| `puzzle_table_name` | string | `""` | DynamoDB table name (composed from `module.database`); set as `PUZZLE_TABLE_NAME` env var. |
| `puzzle_table_arn` | string | `""` | DynamoDB table ARN; scopes the Lambda's DynamoDB IAM policy. |
| `sqs_queue_url` | string | `""` | Generation queue URL (composed from `module.generation`); set as `SQS_QUEUE_URL` env var. |
| `sqs_queue_arn` | string | `""` | Generation queue ARN; scopes the Lambda's SQS publish IAM policy. |
| `clerk_publishable_key` | string (sensitive) | `""` | Initial value for the publishable-key SSM parameter (browser-safe). Subsequent rotations happen directly in SSM (lifecycle ignore_changes). |
| `clerk_secret_key` | string (sensitive) | `""` | Initial value for the secret-key SSM parameter. Same rotation pattern. |

## Outputs

| Output | Consumer |
|---|---|
| `api_gateway_invoke_url` | Root `outputs.tf` → `api_gateway_url`. |
| `api_gateway_domain` | Root `main.tf` → `frontend` module (`api_gateway_domain`) — used as the API origin in CloudFront. |
| `api_gateway_stage` | Root `main.tf` → `frontend` module (`api_gateway_stage`) — used as the API origin's `origin_path`. |
| `clerk_publishable_key_param_name` | Root `outputs.tf` → `cd.yml:89` reads this with `terraform output -raw` to fetch the publishable key from SSM at frontend-build time. |
| `clerk_secret_key_param_name` | (not consumed — candidate cleanup) |

## AWS resources created

| Resource | Purpose |
|---|---|
| `aws_iam_role.lambda_exec` | Execution role for the API Lambda; trust policy allows `lambda.amazonaws.com`. |
| `aws_iam_role_policy.lambda_logs` | CloudWatch Logs CreateLogGroup/Stream + PutLogEvents, scoped to `/aws/lambda/<function-name>:*`. |
| `aws_iam_role_policy.lambda_sqs` | `sqs:SendMessage` only, scoped to the generation queue ARN. |
| `aws_iam_role_policy.lambda_dynamodb` | `GetItem`, `PutItem`, `UpdateItem`, `DeleteItem`, `Query`, `TransactWriteItems` on the puzzle-pool table. Comment in code says "match the call sites in `internal/repository/{puzzle,daily}.go`" — drift between policy + code is silent and only fails at runtime. |
| `aws_iam_role_policy.lambda_ssm_clerk` | `ssm:GetParameter` scoped to the two Clerk SSM parameter ARNs. |
| `aws_iam_role_policy.lambda_kms_ssm` | `kms:Decrypt` on the `aws/ssm` AWS-managed KMS key, with `kms:ViaService = ssm.<region>.amazonaws.com` condition (defense-in-depth). |
| `aws_lambda_function.api` | The Go Lambda. `provided.al2023` runtime, `bootstrap` handler, 29 s timeout, 512 MB memory. `source_code_hash` uses `fileexists()` guard so CI plan works without the zip. Env vars: `PUZZLE_TABLE_NAME`, `SQS_QUEUE_URL`, `CLERK_SECRET_PARAM_NAME` (the *name*, not the secret). |
| `aws_api_gateway_rest_api.api` | REST API container. |
| `aws_lambda_permission.api_gateway` | Permits API Gateway to invoke the Lambda. |
| `aws_api_gateway_resource.api_root` | `/api` path. |
| `aws_api_gateway_resource.api_proxy` | `/api/{proxy+}` path — catches every backend route. |
| `aws_api_gateway_method.api_proxy_any` | `ANY` HTTP method on `/api/{proxy+}` with no authz (Lambda enforces Clerk auth in middleware). |
| `aws_api_gateway_integration.api_proxy_lambda` | `AWS_PROXY` integration → API Lambda. |
| `aws_api_gateway_deployment.api` | Forced redeploy on changes to resources/methods/integration (`triggers` hashes resource ids + integration URI to catch Lambda renames — defends against the PR #102 incident where a rename left the stage invoking a deleted Lambda). |
| `aws_api_gateway_stage.api` | Stage named after `var.environment`. |
| `aws_api_gateway_method_settings.api` | Throttling: burst 50, rate 100 (per the AWS API Gateway throttle model). |
| `aws_ssm_parameter.clerk_publishable_key` | Type `String`, name `/reign/<env>/clerk-publishable-key`. `lifecycle { ignore_changes = [value] }`. |
| `aws_ssm_parameter.clerk_secret_key` | Type `SecureString`, name `/reign/<env>/clerk-secret-key`. `lifecycle { ignore_changes = [value] }`. |
| `data.aws_kms_key.ssm` | Lookup for the `aws/ssm` managed key ARN. |
| `data.aws_region.current` | Used for the `kms:ViaService` condition and the API Gateway domain output. |

## Gotchas

- **SSM parameters use `lifecycle { ignore_changes = [value] }`.** Once created, the deployed value is the source of truth. Rotating the upstream `secrets.CLERK_*` GitHub secret does NOT change the live value — the next clean-room re-apply would pick it up, but routine applies will not. Rotate keys via `aws ssm put-parameter --overwrite` (and update the GitHub secret to keep clean-room state correct). See `infra/CLAUDE.md` Terraform Review Checklist item 7.
- **`source_code_hash` uses `fileexists(var.lambda_zip_path) ? filebase64sha256(...) : null`.** CI plan runs without the zip on disk; without this guard, plan would fail. The build step (`task build:lambda`) runs before plan/apply in both CI and CD.
- **The `triggers` block on `aws_api_gateway_deployment` hashes `integration.uri` as well as the resource ids.** This catches Lambda function renames that change the invoke ARN; without it, a rename leaves the stage pointing at a deleted Lambda. Don't remove the URI from the hash.
- **CLOUDWATCH log group is auto-created.** No `aws_cloudwatch_log_group` is defined here, so AWS auto-creates one on first invocation with `Never expire` retention. Tracked as a P1 cost-smell follow-up.
- **`PUZZLE_TABLE_NAME`, `SQS_QUEUE_URL`, `CLERK_SECRET_PARAM_NAME` Lambda env vars are readable by anyone with `lambda:GetFunctionConfiguration`.** The Clerk secret itself is in SSM (SecureString) — only the param NAME is in env. Don't move secrets into Lambda env.
- **DynamoDB IAM policy is wider than strictly needed by code today** — tracked as a P2 IAM-tightening follow-up.
