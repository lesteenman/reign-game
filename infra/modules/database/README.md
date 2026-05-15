# module: database

## Responsibility

Provisions the single DynamoDB table the entire backend uses (`<project>-<env>-puzzle-pool`). Pure data-layer module — no IAM, no compute, no events. Other modules that need access to the table receive its name + ARN as inputs (composed at root `main.tf`); this module does NOT define any IAM, by design.

## Inputs

| Variable | Type | Default | Meaning |
|---|---|---|---|
| `project_name` | string | (required) | Project name prefix (`reign-game`). Combined with env into the resource name `<project>-<env>-puzzle-pool`. |
| `environment` | string | (required) | Deployment environment (`acc` today; `prod` planned). |

## Outputs

| Output | Consumer |
|---|---|
| `puzzle_table_name` | Root `main.tf` → `api`, `generation`, `daily_cron` (as `puzzle_table_name` / `puzzle_pool_table_name` input + as the `PUZZLE_TABLE_NAME` Lambda env var). Also surfaced at root `outputs.tf` as `puzzle_table_name`. |
| `puzzle_table_arn` | Root `main.tf` → `api`, `generation`, `daily_cron` (used to scope IAM DynamoDB policies). |

## AWS resources created

| Resource | Purpose |
|---|---|
| `aws_dynamodb_table.puzzle_pool` | Single-table store. `PAY_PER_REQUEST` billing. Hash key `PK` (S), range key `SK` (S). No GSIs. Item rows include `CONFIG#<size>#<mode>` (puzzle-pool config), `<size>#<mode> / <uuid>` (approved puzzles), and `DAILY-CANDIDATE / <single>`, `DAILY-SCHEDULE / <YYYY-MM-DD>` etc. (daily-puzzle scheduler rows). |

## IAM policies created

None — this module does not define IAM. Callers (api, generation, daily-cron modules) attach their own policies scoped to `puzzle_table_arn`.

## Gotchas

- **No explicit `server_side_encryption` block.** AWS defaults to encryption with an AWS-owned key; the audit-compliant form would set `server_side_encryption { enabled = true }`. See `FINDINGS.md`.
- **No GSIs.** Every access pattern is keyed by `PK / SK`. New access patterns require either a GSI here (a Terraform change) or an item-shape change (a backend code change). Discuss before adding GSIs — they double cost.
- **No `point_in_time_recovery` enabled.** Acceptable for the acc env where the puzzle pool can be regenerated; prod should re-evaluate.
- **`force_destroy` is NOT set.** A destroy will fail if items exist — appropriate guard against accidental wipe.
