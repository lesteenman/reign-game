output "cloudfront_url" {
  description = "CloudFront distribution domain name for the frontend"
  value       = module.frontend.cloudfront_domain
}

output "api_gateway_url" {
  description = "API Gateway invoke URL"
  value       = module.api.api_gateway_invoke_url
}

output "frontend_bucket_name" {
  description = "S3 bucket name for frontend assets"
  value       = module.frontend.s3_bucket_name
}

output "frontend_distribution_id" {
  description = "CloudFront distribution ID for cache invalidation"
  value       = module.frontend.cloudfront_distribution_id
}

output "puzzle_table_name" {
  description = "DynamoDB puzzle pool table name"
  value       = module.database.puzzle_table_name
}

output "sqs_queue_url" {
  description = "SQS puzzle generation queue URL"
  value       = module.generation.queue_url
}

output "generator_function_name" {
  description = "Generator Lambda function name"
  value       = module.generation.generator_function_name
}

output "clerk_publishable_key_param_name" {
  description = "SSM Parameter Store name for the Clerk publishable key. CD reads this and passes the fetched value to the frontend build as VITE_CLERK_PUBLISHABLE_KEY."
  value       = module.api.clerk_publishable_key_param_name
}

output "client_api_key_value" {
  description = "Browser-distributed API Gateway key bound to the client usage plan. CD reads this and bakes it into the frontend build as VITE_API_KEY so every /api/* request carries x-api-key."
  value       = module.api.client_api_key_value
  sensitive   = true
}
