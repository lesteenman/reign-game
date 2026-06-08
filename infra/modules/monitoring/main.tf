locals {
  name_prefix = "${var.project_name}-${var.environment}"
}

# Alert channel. Alarms publish here; no subscription is created in this slice
# (deferred — the topic has no subscriber until one is attached out of band, so
# alarms fire but notify no human until then). See docs/runbooks/monitoring.md.
resource "aws_sns_topic" "alerts" {
  name = "${local.name_prefix}-alerts"

  tags = var.tags
}

# --- Lambda error alarms (absolute Sum, not rate — at low traffic a rate is
# noisy). One per function. treat_missing_data = notBreaching keeps sparse /
# scheduled functions out of perpetual INSUFFICIENT_DATA. ---

resource "aws_cloudwatch_metric_alarm" "lambda_errors_api" {
  alarm_name          = "${local.name_prefix}-lambda-errors-api"
  alarm_description   = "API Lambda function errors >= 5 in 5 minutes."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  dimensions          = { FunctionName = var.api_function_name }
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 5
  period              = 300
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors_generator" {
  alarm_name          = "${local.name_prefix}-lambda-errors-generator"
  alarm_description   = "Generator Lambda function errors >= 5 in 5 minutes."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  dimensions          = { FunctionName = var.generator_function_name }
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 5
  period              = 300
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors_daily_cron" {
  alarm_name          = "${local.name_prefix}-lambda-errors-daily-cron"
  alarm_description   = "Daily-cron Lambda function errors >= 5 in 5 minutes."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  dimensions          = { FunctionName = var.daily_cron_function_name }
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 5
  period              = 300
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  tags = var.tags
}

# API Gateway 5xx. Targets API Gateway (in-region), not CloudFront, so no
# us-east-1 provider alias is needed.
resource "aws_cloudwatch_metric_alarm" "api_gateway_5xx" {
  alarm_name        = "${local.name_prefix}-api-gateway-5xx"
  alarm_description = "API Gateway 5XX responses >= 5 in 5 minutes."
  namespace         = "AWS/ApiGateway"
  metric_name       = "5XXError"
  statistic         = "Sum"
  dimensions = {
    ApiName = var.api_gateway_name
    Stage   = var.api_gateway_stage
  }
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 5
  period              = 300
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  tags = var.tags
}

# Generation DLQ not empty — any dead-lettered message means a generation job
# failed past its retries.
resource "aws_cloudwatch_metric_alarm" "generation_dlq_not_empty" {
  alarm_name          = "${local.name_prefix}-generation-dlq-not-empty"
  alarm_description   = "Generation DLQ has >= 1 visible message in 5 minutes."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Maximum"
  dimensions          = { QueueName = var.generation_dlq_name }
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 1
  period              = 300
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  tags = var.tags
}
