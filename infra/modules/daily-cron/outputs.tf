output "lambda_function_name" {
  description = "Daily-cron Lambda function name"
  value       = aws_lambda_function.daily_cron.function_name
}

output "lambda_function_arn" {
  description = "Daily-cron Lambda function ARN"
  value       = aws_lambda_function.daily_cron.arn
}

output "t6h_rule_name" {
  description = "EventBridge rule name for the T-6h ensure schedule"
  value       = aws_cloudwatch_event_rule.daily_cron_t6h_ensure.name
}

output "t0_rule_name" {
  description = "EventBridge rule name for the T=0 finalize schedule"
  value       = aws_cloudwatch_event_rule.daily_cron_t0_finalize.name
}
