# module: generation

## Responsibility

Provisions the asynchronous puzzle-generation pipeline: an SQS queue (with DLQ), a Go generator Lambda (`cmd/api` running in `GENERATOR_MODE=sqs`), an event source mapping that triggers the Lambda from the queue, and least-privilege IAM. The API Lambda publishes generation jobs to this queue; the generator consumes them and writes approved puzzles to the puzzle-pool DynamoDB table.

## Inputs

| Variable | Type | Default | Meaning |
|---|---|---|---|
| `project_name` | string | (required) | Project name prefix. |
| `environment` | string | (required) | Deployment environment. |
| `lambda_zip_path` | string | (required) | Path on disk to the generator Lambda zip (built by `task build:lambda` — same `bootstrap` binary as the API Lambda; behavior switches on `GENERATOR_MODE=sqs` env). |
| `puzzle_table_name` | string | (required) | DynamoDB puzzle-pool table name; used as `PUZZLE_TABLE_NAME` env var. |
| `puzzle_table_arn` | string | (required) | DynamoDB puzzle-pool table ARN; scopes the generator's IAM policy. |

## Outputs

| Output | Consumer |
|---|---|
| `queue_url` | Root `main.tf` → `api` module (`sqs_queue_url`, the API's `SQS_QUEUE_URL` env var) + `daily_cron` module (`generation_queue_url`, reactive replenish publish target). Also surfaced at root `outputs.tf` as `sqs_queue_url`. |
| `queue_arn` | Root `main.tf` → `api` module (`sqs_queue_arn`, to scope API's `sqs:SendMessage` policy) + `daily_cron` module (`generation_queue_arn`, same purpose). |
| `generator_function_name` | Root `outputs.tf` → `generator_function_name`. (Not currently consumed by any workflow.) |
| `dlq_url` | (not consumed — candidate cleanup or surface to root for operational visibility; see `FINDINGS.md`) |

## AWS resources created

| Resource | Purpose |
|---|---|
| `aws_sqs_queue.generation_dlq` | Dead-letter queue. |
| `aws_sqs_queue.generation` | Main queue. `visibility_timeout_seconds = 900` (matches Lambda max timeout). Redrive policy points at `generation_dlq` with `maxReceiveCount = 3`. |
| `aws_iam_role.generator_exec` | Execution role for the generator Lambda. |
| `aws_iam_role_policy.generator_logs` | CloudWatch Logs CreateLogGroup/Stream + PutLogEvents, scoped to the generator's log group. |
| `aws_iam_role_policy.generator_sqs` | `sqs:ReceiveMessage`, `sqs:DeleteMessage`, `sqs:GetQueueAttributes` — the SQS poller permissions only (no SendMessage). Scoped to the main queue ARN. |
| `aws_iam_role_policy.generator_dynamodb` | `dynamodb:PutItem`, `Query`, `UpdateItem` on the puzzle-pool table. (No `GetItem`, no `DeleteItem` — least privilege.) |
| `aws_lambda_function.generator` | The Go Lambda. `provided.al2023` runtime, `bootstrap` handler, 15-min (900 s) timeout, 512 MB memory. Env vars: `GENERATOR_MODE=sqs`, `PUZZLE_TABLE_NAME`, `SQS_QUEUE_URL`. |
| `aws_lambda_event_source_mapping.sqs_trigger` | Wires the queue to the Lambda. `batch_size = 1` (each puzzle gets its own invocation). |

## Gotchas

- **`batch_size = 1`** — each SQS message triggers a fresh Lambda invocation. Correct for heavy puzzle generation; would be a cost smell for cheap work. See `FINDINGS.md`.
- **15-minute timeout.** The Lambda max — accommodates worst-case generator runs (large N + property tests). If usage drops, drop the timeout.
- **Generator's DynamoDB policy is intentionally narrower than the API Lambda's** (write-only-ish: `PutItem`, `Query`, `UpdateItem`). New access patterns require explicit policy updates.
- **No CloudWatch log group resource.** Same gotcha as the API module — Lambda auto-creates with `Never expire` retention.
- **DLQ has no alarm or notification.** A poison message that exhausts `maxReceiveCount = 3` lands silently in the DLQ. Adding a CloudWatch alarm + SNS topic is future cleanup.
- **`SQS_QUEUE_URL` env var on the generator is the same queue it's reading from** — used by the worker for retry / nack flows. Don't be tempted to set it to a different queue.
