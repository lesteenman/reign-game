# module: daily-cron

## Responsibility

Provisions the daily-puzzle scheduler. A separate Go Lambda (`cmd/daily-cron`) is invoked by two EventBridge cron rules: T-6h ensure (18:00 UTC daily) pre-stages tomorrow's daily puzzle; T=0 finalize (00:00 UTC daily) flips today's puzzle to active. The Lambda also publishes reactive replenish messages to the generation queue when the approved pool drains.

## Inputs

| Variable | Type | Default | Meaning |
|---|---|---|---|
| `name_prefix` | string | (required) | Prefix for resource names — composed at root as `<project>-<env>`. |
| `puzzle_pool_table_name` | string | (required) | DynamoDB puzzle-pool table name; set as `PUZZLE_POOL_TABLE` env var. |
| `puzzle_pool_table_arn` | string | (required) | DynamoDB puzzle-pool table ARN; scopes the daily-cron's DynamoDB IAM policy. |
| `lambda_zip_path` | string | `""` | Path to the daily-cron Lambda zip (built by `task build:lambda`). Empty default keeps `terraform plan` working in CI without the artifact. |
| `generation_queue_arn` | string | (required) | Scopes the daily-cron's `sqs:SendMessage` policy to the single generation queue. |
| `generation_queue_url` | string | (required) | Passed as `SQS_QUEUE_URL` env var so the Lambda can publish reactive replenish messages. |
| `tags` | map(string) | `{}` | Tags applied to every resource created by this module. |

## Outputs

| Output | Consumer |
|---|---|
| `lambda_function_name` | (not consumed — see `FINDINGS.md`) |
| `lambda_function_arn` | (not consumed) |
| `t6h_rule_name` | (not consumed) |
| `t0_rule_name` | (not consumed) |

(All four are operator-facing names that could be consumed by future runbooks or CloudWatch dashboards. Keep until/unless explicitly pruned.)

## AWS resources created

| Resource | Purpose |
|---|---|
| `aws_iam_role.daily_cron_exec` | Execution role for the daily-cron Lambda. |
| `aws_iam_role_policy.daily_cron_logs` | CloudWatch Logs CreateLogGroup/Stream + PutLogEvents. |
| `aws_iam_role_policy.daily_cron_dynamodb` | `GetItem`, `PutItem`, `UpdateItem`, `DeleteItem`, `Query`, `TransactWriteItems` on the puzzle-pool table. |
| `aws_iam_role_policy.daily_cron_sqs` | `sqs:SendMessage` only, scoped to the generation queue ARN. |
| `aws_lambda_function.daily_cron` | Go Lambda. `provided.al2023` runtime, 60 s timeout, 256 MB memory. Env vars: `PUZZLE_POOL_TABLE`, `SQS_QUEUE_URL`. |
| `aws_cloudwatch_event_rule.daily_cron_t6h_ensure` | EventBridge rule `cron(0 18 * * ? *)` (18:00 UTC daily, T-6h pre-stage). |
| `aws_cloudwatch_event_rule.daily_cron_t0_finalize` | EventBridge rule `cron(0 0 * * ? *)` (00:00 UTC daily, T=0 finalize). |
| `aws_cloudwatch_event_target.daily_cron_t6h_ensure` | Targets the Lambda; `input = jsonencode({"detail-type" = "t-6h-ensure"})` so the Go dispatcher knows which mode to run. |
| `aws_cloudwatch_event_target.daily_cron_t0_finalize` | Same pattern with `"detail-type" = "t-0-finalize"`. |
| `aws_lambda_permission.allow_eventbridge_t6h_ensure` | Permits the T-6h rule to invoke the Lambda. |
| `aws_lambda_permission.allow_eventbridge_t0_finalize` | Permits the T=0 rule to invoke the Lambda. |

## Gotchas

- **DynamoDB IAM policy includes `DeleteItem` + `TransactWriteItems`.** Re-audit at slice close — the daily-cron may or may not actually delete rows in production. If it never deletes, drop `dynamodb:DeleteItem` to tighten least-privilege.
- **Event-bus dispatch via JSON literal `{"detail-type": "..."}`.** This is consumed by the Go dispatcher in `cmd/daily-cron`. Rename of the detail-type string is a cross-cutting rename (Lambda code + Terraform). Grep for `"t-6h-ensure"` / `"t-0-finalize"` together.
- **Tags come from `var.tags` (root passes `{Project, Environment}`).** Only role + lambda + the two rules are tagged; role policies + event targets + lambda permissions can't be tagged (AWS limitation).
- **`source_code_hash` guard combines `var.lambda_zip_path != "" && fileexists(...)`** — defends against both the empty-string default and the missing-file case (CI plan in PRs that don't touch backend code).
- **No CloudWatch log group resource.** Same gotcha as api/generation — auto-created with `Never expire`.
