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

# DynamoDB access policy for API Lambda
resource "aws_iam_role_policy" "lambda_dynamodb" {
  name = "${local.function_name}-dynamodb"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:Query",
          "dynamodb:UpdateItem",
          "dynamodb:PutItem"
        ]
        Resource = var.puzzle_table_arn
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

  # Force new deployment when integrations change
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_integration.api_proxy_lambda.id,
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

# ANY on /api/{proxy+} — covers every HTTP method the backend serves
resource "aws_api_gateway_method" "api_proxy_any" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.api_proxy.id
  http_method   = "ANY"
  authorization = "NONE"
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
