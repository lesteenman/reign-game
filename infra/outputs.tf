output "cloudfront_url" {
  description = "CloudFront distribution domain name for the frontend"
  value       = module.frontend.cloudfront_domain
}

output "api_gateway_url" {
  description = "API Gateway invoke URL"
  value       = module.api.api_gateway_invoke_url
}
