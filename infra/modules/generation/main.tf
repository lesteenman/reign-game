locals {
  function_name = "${var.project_name}-${var.environment}-generator"
  queue_name    = "${var.project_name}-${var.environment}-puzzle-generation"
  dlq_name      = "${var.project_name}-${var.environment}-puzzle-generation-dlq"
}

# SQS Dead-Letter Queue
resource "aws_sqs_queue" "generation_dlq" {
  name = local.dlq_name

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

# SQS Main Queue
resource "aws_sqs_queue" "generation" {
  name                       = local.queue_name
  visibility_timeout_seconds = 900

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.generation_dlq.arn
    maxReceiveCount     = 3
  })

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

# IAM role for Generator Lambda
resource "aws_iam_role" "generator_exec" {
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
}

# CloudWatch Logs policy for Generator Lambda
resource "aws_iam_role_policy" "generator_logs" {
  name = "${local.function_name}-logs"
  role = aws_iam_role.generator_exec.id

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

# SQS consume policy for Generator Lambda
resource "aws_iam_role_policy" "generator_sqs" {
  name = "${local.function_name}-sqs"
  role = aws_iam_role.generator_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes"
        ]
        Resource = aws_sqs_queue.generation.arn
      }
    ]
  })
}

# DynamoDB write policy for Generator Lambda
resource "aws_iam_role_policy" "generator_dynamodb" {
  name = "${local.function_name}-dynamodb"
  role = aws_iam_role.generator_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:Query",
          "dynamodb:UpdateItem"
        ]
        Resource = var.puzzle_table_arn
      }
    ]
  })
}

# Generator Lambda function
resource "aws_lambda_function" "generator" {
  function_name = local.function_name
  role          = aws_iam_role.generator_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  filename      = var.lambda_zip_path

  source_code_hash = fileexists(var.lambda_zip_path) ? filebase64sha256(var.lambda_zip_path) : null

  timeout     = 900
  memory_size = 512

  environment {
    variables = {
      GENERATOR_MODE    = "sqs"
      PUZZLE_TABLE_NAME = var.puzzle_table_name
      SQS_QUEUE_URL     = aws_sqs_queue.generation.url
    }
  }

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

# SQS event source mapping for Generator Lambda
resource "aws_lambda_event_source_mapping" "sqs_trigger" {
  event_source_arn = aws_sqs_queue.generation.arn
  function_name    = aws_lambda_function.generator.arn
  batch_size       = 1
}
