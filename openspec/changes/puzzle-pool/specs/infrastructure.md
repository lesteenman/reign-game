# Spec: Infrastructure

Covers R-041 (Terraform: DynamoDB, SQS, Generator Lambda) and R-045 (LocalStack setup).

## Requirements

### TF-01: DynamoDB Puzzle Pool Table

- Terraform resource `aws_dynamodb_table.puzzle_pool` in a new `infra/modules/database/` module
- Table name: `${var.project_name}-${var.environment}-puzzle-pool`
- Billing mode: `PAY_PER_REQUEST`
- Hash key: `PK` (String)
- Range key: `SK` (String)
- No GSI
- Tags: project name, environment
- Tests: `terraform validate` passes, `terraform plan` shows expected resources

### TF-02: SQS Queue + Dead-Letter Queue

- Terraform resources in a new `infra/modules/generation/` module (or within the API module)
- Main queue: `${var.project_name}-${var.environment}-puzzle-generation`
  - Visibility timeout: 900 seconds
  - Message retention: 4 days (default)
- Dead-letter queue: `${var.project_name}-${var.environment}-puzzle-generation-dlq`
  - `maxReceiveCount`: 3
- Tests: `terraform validate` passes

### TF-03: Generator Lambda Function

- Terraform resource `aws_lambda_function.generator` using the same zip as the API Lambda
- Timeout: 900 seconds (15 minutes)
- Memory: 512MB
- Environment variable: `GENERATOR_MODE=sqs`, `PUZZLE_TABLE_NAME` from TF-01 output
- SQS event source mapping: batch size 1, from TF-02 main queue
- Tests: `terraform plan` shows the Lambda + event source mapping

### TF-04: IAM Permissions

- API Lambda role gets: `sqs:SendMessage` on the generation queue
- Generator Lambda gets its own role with:
  - `sqs:ReceiveMessage`, `sqs:DeleteMessage`, `sqs:GetQueueAttributes` on the generation queue
  - `dynamodb:PutItem`, `dynamodb:Query`, `dynamodb:UpdateItem` on the puzzle-pool table
  - `logs:CreateLogGroup`, `logs:CreateLogStream`, `logs:PutLogEvents`
- API Lambda role gets: `dynamodb:Query`, `dynamodb:UpdateItem` on the puzzle-pool table (for serving + status updates)
- Principle of least privilege: no wildcard resource ARNs
- Tests: `terraform validate` passes, IAM policies reference correct ARNs

### TF-05: LocalStack Setup

- `docker-compose.yml` includes SQS in the LocalStack services list (DynamoDB already present)
- Init script creates the `puzzle-pool` table and `puzzle-generation` queue on container startup
- Taskfile: `task dev:generator` starts the SQS consumer process with correct environment variables
- Taskfile: `task dev:backend` updated with environment variables for DynamoDB and SQS endpoints
- Tests: `docker compose up localstack` starts successfully, table and queue exist after init

## Acceptance Criteria

All TF-01 through TF-05 pass. `terraform plan` shows all expected resources. LocalStack starts with both DynamoDB table and SQS queue ready.
