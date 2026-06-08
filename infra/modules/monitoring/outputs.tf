output "alerts_topic_arn" {
  description = "ARN of the SNS alerts topic. Attach an email/Slack subscriber to this to receive alarm notifications (see docs/runbooks/monitoring.md)."
  value       = aws_sns_topic.alerts.arn
}

output "dashboard_name" {
  description = "Name of the CloudWatch overview dashboard"
  value       = aws_cloudwatch_dashboard.main.dashboard_name
}
