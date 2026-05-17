locals {
  function_name = "${var.name_prefix}-daily-cron"
}

# IAM role for daily-cron Lambda
resource "aws_iam_role" "daily_cron_exec" {
  name = "${local.function_name}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = var.tags
}

# CloudWatch Logs policy
resource "aws_iam_role_policy" "daily_cron_logs" {
  name = "${local.function_name}-logs"
  role = aws_iam_role.daily_cron_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/aws/lambda/${local.function_name}:*"
      }
    ]
  })
}

# DynamoDB access policy — Daily-Puzzle access patterns on puzzle-pool only.
# No GSI access required for the daily-cron Lambda's IAM scope.
resource "aws_iam_role_policy" "daily_cron_dynamodb" {
  name = "${local.function_name}-dynamodb"
  role = aws_iam_role.daily_cron_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:TransactWriteItems"
        ]
        Resource = var.puzzle_pool_table_arn
      }
    ]
  })
}

# SQS access — auto-replenish-puzzle-pool slice. The daily-cron Lambda
# publishes to the puzzle-generation queue from the reactive top-up
# hook installed on EnsureCandidate / SyncFinalizeForToday. Scoped to
# the single generation queue ARN, no DLQ access needed.
resource "aws_iam_role_policy" "daily_cron_sqs" {
  name = "${local.function_name}-sqs"
  role = aws_iam_role.daily_cron_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = var.generation_queue_arn
      }
    ]
  })
}

# Daily-cron Lambda function
resource "aws_lambda_function" "daily_cron" {
  function_name = local.function_name
  role          = aws_iam_role.daily_cron_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  filename      = var.lambda_zip_path

  source_code_hash = (var.lambda_zip_path != "" && fileexists(var.lambda_zip_path)) ? filebase64sha256(var.lambda_zip_path) : null

  timeout     = 60
  memory_size = 256

  environment {
    variables = {
      PUZZLE_POOL_TABLE = var.puzzle_pool_table_name
      SQS_QUEUE_URL     = var.generation_queue_url
    }
  }

  tags = var.tags
}

# EventBridge rule: T-6h ensure (18:00 UTC daily)
resource "aws_cloudwatch_event_rule" "daily_cron_t6h_ensure" {
  name                = "${local.function_name}-t6h-ensure"
  description         = "Daily-puzzle T-6h ensure: pre-stage tomorrow's daily-puzzle materialisation"
  schedule_expression = "cron(0 18 * * ? *)"
  event_bus_name      = "default"

  tags = var.tags
}

# EventBridge rule: T=0 finalize (00:00 UTC daily)
resource "aws_cloudwatch_event_rule" "daily_cron_t0_finalize" {
  name                = "${local.function_name}-t0-finalize"
  description         = "Daily-puzzle T=0 finalize: flip today's daily-puzzle to active"
  schedule_expression = "cron(0 0 * * ? *)"
  event_bus_name      = "default"

  tags = var.tags
}

# Targets: Lambda receives a JSON literal carrying detail-type the Go dispatcher reads
resource "aws_cloudwatch_event_target" "daily_cron_t6h_ensure" {
  rule      = aws_cloudwatch_event_rule.daily_cron_t6h_ensure.name
  target_id = "daily-cron-t6h-ensure"
  arn       = aws_lambda_function.daily_cron.arn
  input     = jsonencode({ "detail-type" = "t-6h-ensure" })
}

resource "aws_cloudwatch_event_target" "daily_cron_t0_finalize" {
  rule      = aws_cloudwatch_event_rule.daily_cron_t0_finalize.name
  target_id = "daily-cron-t0-finalize"
  arn       = aws_lambda_function.daily_cron.arn
  input     = jsonencode({ "detail-type" = "t-0-finalize" })
}

# Lambda invoke permissions for EventBridge
resource "aws_lambda_permission" "allow_eventbridge_t6h_ensure" {
  statement_id  = "AllowExecutionFromEventBridgeT6h"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.daily_cron.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.daily_cron_t6h_ensure.arn
}

resource "aws_lambda_permission" "allow_eventbridge_t0_finalize" {
  statement_id  = "AllowExecutionFromEventBridgeT0"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.daily_cron.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.daily_cron_t0_finalize.arn
}
