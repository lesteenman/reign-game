data "aws_region" "current" {}

locals {
  function_name = "${var.project_name}-${var.environment}-api"
}

# IAM role for Lambda execution
resource "aws_iam_role" "lambda_exec" {
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

# Least-privilege policy: CloudWatch Logs write only
resource "aws_iam_role_policy" "lambda_logs" {
  name = "${local.function_name}-logs"
  role = aws_iam_role.lambda_exec.id

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

# SQS publish policy for API Lambda
resource "aws_iam_role_policy" "lambda_sqs" {
  name = "${local.function_name}-sqs"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage"
        ]
        Resource = var.sqs_queue_arn
      }
    ]
  })
}

# DynamoDB access policy for API Lambda — actions match the call sites in
# internal/repository/{puzzle,daily}.go. Keep this list in sync with that
# code; an untracked addition there will fail at runtime, not at build.
resource "aws_iam_role_policy" "lambda_dynamodb" {
  name = "${local.function_name}-dynamodb"
  role = aws_iam_role.lambda_exec.id

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
        # Index ARN included so the api Lambda can Query the ready-index GSI
        # (CountReady/NextReady). The generator only writes the readyPoolKey
        # attribute via PutPuzzle and never reads the GSI.
        Resource = [var.puzzle_table_arn, "${var.puzzle_table_arn}/index/*"]
      }
    ]
  })
}

# Lambda function
resource "aws_lambda_function" "api" {
  function_name = local.function_name
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  filename      = var.lambda_zip_path

  source_code_hash = fileexists(var.lambda_zip_path) ? filebase64sha256(var.lambda_zip_path) : null

  timeout     = 29
  memory_size = 512

  environment {
    variables = {
      PUZZLE_TABLE_NAME = var.puzzle_table_name
      SQS_QUEUE_URL     = var.sqs_queue_url
      # The Lambda reads the Clerk secret via SSM at startup
      # (auth.LoadClerkSecret). Only the parameter NAME lives here —
      # never the secret itself — because Lambda env vars are readable
      # via lambda:GetFunctionConfiguration.
      CLERK_SECRET_PARAM_NAME = aws_ssm_parameter.clerk_secret_key.name
    }
  }
}

# API Gateway REST API
resource "aws_api_gateway_rest_api" "api" {
  name        = "${var.project_name}-${var.environment}-api"
  description = "${var.project_name} REST API"
}

# Permission for API Gateway to invoke Lambda
resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

# API Gateway deployment
resource "aws_api_gateway_deployment" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id

  depends_on = [
    aws_api_gateway_integration.api_proxy_lambda,
  ]

  # Force new deployment when any part of the API surface changes. The
  # integration's `id` is a composite of (rest_api_id, resource_id,
  # http_method) — it does NOT include the integration `uri`. Hashing
  # the URI as well catches Lambda renames (e.g. PR #102's prod→acc
  # rename, where the live integration was updated to the new function
  # name but the deployment was never rebuilt, leaving the stage
  # invoking the deleted reign-game-prod-api Lambda and serving 500s
  # for two days). Resource and method ids are included so additions
  # to the API surface also force a redeploy.
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.api_root.id,
      aws_api_gateway_resource.api_proxy.id,
      aws_api_gateway_method.api_proxy_any.id,
      aws_api_gateway_integration.api_proxy_lambda.id,
      aws_api_gateway_integration.api_proxy_lambda.uri,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

# API Gateway stage
resource "aws_api_gateway_stage" "api" {
  deployment_id = aws_api_gateway_deployment.api.id
  rest_api_id   = aws_api_gateway_rest_api.api.id
  stage_name    = var.environment
}

# /api resource — all backend routes live under this prefix
# (puzzles, admin, health). SPA routes like /admin stay on CloudFront's S3
# origin; only /api/* is forwarded to Lambda.
resource "aws_api_gateway_resource" "api_root" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "api"
}

# /api/{proxy+} catches every backend path
resource "aws_api_gateway_resource" "api_proxy" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.api_root.id
  path_part   = "{proxy+}"
}

# ANY on /api/{proxy+} — covers every HTTP method the backend serves.
# api_key_required = true gates every /api/* request behind the
# usage-plan-bound API key below. The key is browser-distributed (baked
# into the frontend bundle at build time), so it acts as a namespace
# token, not a secret. Its value is to bind callers to the usage plan
# so the per-key throttle applies; without it, requests fall to API
# Gateway's account-wide default throttle, which is much higher.
resource "aws_api_gateway_method" "api_proxy_any" {
  rest_api_id      = aws_api_gateway_rest_api.api.id
  resource_id      = aws_api_gateway_resource.api_proxy.id
  http_method      = "ANY"
  authorization    = "NONE"
  api_key_required = true
}

# Lambda integration for /api/{proxy+}
resource "aws_api_gateway_integration" "api_proxy_lambda" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.api_proxy.id
  http_method             = aws_api_gateway_method.api_proxy_any.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

# API Gateway throttling
resource "aws_api_gateway_method_settings" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  stage_name  = aws_api_gateway_stage.api.stage_name
  method_path = "*/*"

  settings {
    throttling_burst_limit = 50
    throttling_rate_limit  = 100
  }
}

# Browser-distributed API key bound to a usage plan with per-key
# throttle. AWS auto-generates the value when `value` is omitted.
# Frontend reads it as `VITE_API_KEY` at build time and sends it as
# `x-api-key` on every request. Anyone can scrape it from the bundle
# — that's intentional; the key is a namespace token, not a secret.
# Its purpose is to bind callers to the usage plan so the per-key
# throttle below caps total app traffic.
resource "aws_api_gateway_api_key" "client" {
  name        = "${var.project_name}-${var.environment}-client"
  description = "Browser-distributed key for the Reign frontend; bound to the client usage plan."
  enabled     = true
}

# Usage plan with per-key throttle. 50 rps / 100 burst is well above
# expected legitimate traffic (a single user solving the daily
# generates ~3-5 requests across page load + submit) and bounds the
# steady-state damage of a scraped-key attack. No daily quota: a
# quota that gets burned by an attacker would lock out legitimate
# users for hours, which is a worse failure mode than letting the
# rate-limit absorb the spike.
resource "aws_api_gateway_usage_plan" "client" {
  name        = "${var.project_name}-${var.environment}-client"
  description = "Per-key throttle for the browser-distributed API key."

  api_stages {
    api_id = aws_api_gateway_rest_api.api.id
    stage  = aws_api_gateway_stage.api.stage_name
  }

  throttle_settings {
    rate_limit  = 50
    burst_limit = 100
  }
}

resource "aws_api_gateway_usage_plan_key" "client" {
  key_id        = aws_api_gateway_api_key.client.id
  key_type      = "API_KEY"
  usage_plan_id = aws_api_gateway_usage_plan.client.id
}
