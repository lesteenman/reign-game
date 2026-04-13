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
