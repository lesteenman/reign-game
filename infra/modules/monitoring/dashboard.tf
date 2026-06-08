data "aws_region" "current" {}

# First-line diagnostic dashboard. Default (free) metrics only — no detailed
# monitoring subscription. CloudFront metrics live only in us-east-1, so that
# widget pins region = us-east-1; every other widget uses this stack's region.
resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${local.name_prefix}-overview"

  dashboard_body = jsonencode({
    widgets = [
      # ---------------- Lambda ----------------
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Lambda — Invocations"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/Lambda", "Invocations", "FunctionName", var.api_function_name],
            ["AWS/Lambda", "Invocations", "FunctionName", var.generator_function_name],
            ["AWS/Lambda", "Invocations", "FunctionName", var.daily_cron_function_name],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Lambda — Errors"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/Lambda", "Errors", "FunctionName", var.api_function_name],
            ["AWS/Lambda", "Errors", "FunctionName", var.generator_function_name],
            ["AWS/Lambda", "Errors", "FunctionName", var.daily_cron_function_name],
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Lambda — Duration (p50 + p99)"
          region = data.aws_region.current.name
          view   = "timeSeries"
          period = 300
          metrics = [
            ["AWS/Lambda", "Duration", "FunctionName", var.api_function_name, { stat = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.api_function_name, { stat = "p99" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.generator_function_name, { stat = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.generator_function_name, { stat = "p99" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.daily_cron_function_name, { stat = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.daily_cron_function_name, { stat = "p99" }],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Lambda — Throttles + ConcurrentExecutions"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/Lambda", "Throttles", "FunctionName", var.api_function_name],
            ["AWS/Lambda", "Throttles", "FunctionName", var.generator_function_name],
            ["AWS/Lambda", "Throttles", "FunctionName", var.daily_cron_function_name],
            ["AWS/Lambda", "ConcurrentExecutions", "FunctionName", var.api_function_name, { stat = "Maximum" }],
            ["AWS/Lambda", "ConcurrentExecutions", "FunctionName", var.generator_function_name, { stat = "Maximum" }],
            ["AWS/Lambda", "ConcurrentExecutions", "FunctionName", var.daily_cron_function_name, { stat = "Maximum" }],
          ]
        }
      },

      # ---------------- API Gateway ----------------
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 12
        height = 6
        properties = {
          title  = "API Gateway — Count + 4XX + 5XX"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/ApiGateway", "Count", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage],
            ["AWS/ApiGateway", "4XXError", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage],
            ["AWS/ApiGateway", "5XXError", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 12
        width  = 12
        height = 6
        properties = {
          title  = "API Gateway — Latency (p50/p99) + IntegrationLatency"
          region = data.aws_region.current.name
          view   = "timeSeries"
          period = 300
          metrics = [
            ["AWS/ApiGateway", "Latency", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage, { stat = "p50" }],
            ["AWS/ApiGateway", "Latency", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage, { stat = "p99" }],
            ["AWS/ApiGateway", "IntegrationLatency", "ApiName", var.api_gateway_name, "Stage", var.api_gateway_stage, { stat = "p99" }],
          ]
        }
      },

      # ---------------- DynamoDB (puzzle_pool, on-demand) ----------------
      {
        type   = "metric"
        x      = 0
        y      = 18
        width  = 12
        height = 6
        properties = {
          title  = "DynamoDB — Consumed R/W Capacity"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/DynamoDB", "ConsumedReadCapacityUnits", "TableName", var.puzzle_table_name],
            ["AWS/DynamoDB", "ConsumedWriteCapacityUnits", "TableName", var.puzzle_table_name],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 18
        width  = 12
        height = 6
        properties = {
          title  = "DynamoDB — Throttles + Errors"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/DynamoDB", "ReadThrottleEvents", "TableName", var.puzzle_table_name],
            ["AWS/DynamoDB", "WriteThrottleEvents", "TableName", var.puzzle_table_name],
            ["AWS/DynamoDB", "ThrottledRequests", "TableName", var.puzzle_table_name],
            ["AWS/DynamoDB", "SystemErrors", "TableName", var.puzzle_table_name],
            ["AWS/DynamoDB", "UserErrors", "TableName", var.puzzle_table_name],
          ]
        }
      },

      # ---------------- SQS ----------------
      {
        type   = "metric"
        x      = 0
        y      = 24
        width  = 12
        height = 6
        properties = {
          title  = "SQS — Generation depth + age"
          region = data.aws_region.current.name
          view   = "timeSeries"
          period = 300
          metrics = [
            ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", var.generation_queue_name, { stat = "Maximum" }],
            ["AWS/SQS", "ApproximateAgeOfOldestMessage", "QueueName", var.generation_queue_name, { stat = "Maximum" }],
            ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", var.generation_dlq_name, { stat = "Maximum" }],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 24
        width  = 12
        height = 6
        properties = {
          title  = "SQS — Generation sent/received/deleted"
          region = data.aws_region.current.name
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/SQS", "NumberOfMessagesSent", "QueueName", var.generation_queue_name],
            ["AWS/SQS", "NumberOfMessagesReceived", "QueueName", var.generation_queue_name],
            ["AWS/SQS", "NumberOfMessagesDeleted", "QueueName", var.generation_queue_name],
          ]
        }
      },

      # ---------------- CloudFront (us-east-1 only) ----------------
      {
        type   = "metric"
        x      = 0
        y      = 30
        width  = 12
        height = 6
        properties = {
          title  = "CloudFront — Requests + BytesDownloaded"
          region = "us-east-1"
          view   = "timeSeries"
          stat   = "Sum"
          period = 300
          metrics = [
            ["AWS/CloudFront", "Requests", "DistributionId", var.cloudfront_distribution_id, "Region", "Global"],
            ["AWS/CloudFront", "BytesDownloaded", "DistributionId", var.cloudfront_distribution_id, "Region", "Global"],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 30
        width  = 12
        height = 6
        properties = {
          title  = "CloudFront — Error rates"
          region = "us-east-1"
          view   = "timeSeries"
          stat   = "Average"
          period = 300
          metrics = [
            ["AWS/CloudFront", "4xxErrorRate", "DistributionId", var.cloudfront_distribution_id, "Region", "Global"],
            ["AWS/CloudFront", "5xxErrorRate", "DistributionId", var.cloudfront_distribution_id, "Region", "Global"],
          ]
        }
      },
    ]
  })
}
