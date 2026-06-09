output "queue_url" {
  description = "SQS puzzle generation queue URL"
  value       = aws_sqs_queue.generation.url
}

output "queue_arn" {
  description = "SQS puzzle generation queue ARN"
  value       = aws_sqs_queue.generation.arn
}

output "generator_function_name" {
  description = "Generator Lambda function name"
  value       = aws_lambda_function.generator.function_name
}

output "queue_name" {
  description = "Generation main queue name (CloudWatch AWS/SQS QueueName dimension)"
  value       = aws_sqs_queue.generation.name
}

output "dlq_name" {
  description = "Dead-letter queue name (CloudWatch AWS/SQS QueueName dimension)"
  value       = aws_sqs_queue.generation_dlq.name
}

output "dlq_arn" {
  description = "Dead-letter queue ARN"
  value       = aws_sqs_queue.generation_dlq.arn
}
