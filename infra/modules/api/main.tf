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
}

# API Gateway REST API
resource "aws_api_gateway_rest_api" "api" {
  name        = "${var.project_name}-${var.environment}-api"
  description = "${var.project_name} REST API"
}

# /health resource
resource "aws_api_gateway_resource" "health" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "health"
}

# GET method on /health
resource "aws_api_gateway_method" "health_get" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.health.id
  http_method   = "GET"
  authorization = "NONE"
}

# Lambda integration
resource "aws_api_gateway_integration" "health_lambda" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.health.id
  http_method             = aws_api_gateway_method.health_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
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
    aws_api_gateway_integration.health_lambda,
    aws_api_gateway_integration.puzzles_proxy_lambda,
  ]

  # Force new deployment when integrations change
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_integration.health_lambda.id,
      aws_api_gateway_integration.puzzles_proxy_lambda.id,
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

# /puzzles resource
resource "aws_api_gateway_resource" "puzzles" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "puzzles"
}

# /puzzles/{proxy+} for sub-paths like /puzzles/generate
resource "aws_api_gateway_resource" "puzzles_proxy" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.puzzles.id
  path_part   = "{proxy+}"
}

# GET on /puzzles/{proxy+}
resource "aws_api_gateway_method" "puzzles_proxy_get" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.puzzles_proxy.id
  http_method   = "GET"
  authorization = "NONE"
}

# Lambda integration for /puzzles/{proxy+}
resource "aws_api_gateway_integration" "puzzles_proxy_lambda" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.puzzles_proxy.id
  http_method             = aws_api_gateway_method.puzzles_proxy_get.http_method
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
