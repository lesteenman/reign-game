output "api_gateway_invoke_url" {
  description = "API Gateway invoke URL"
  value       = aws_api_gateway_stage.api.invoke_url
}

output "lambda_function_name" {
  description = "Lambda function name"
  value       = aws_lambda_function.api.function_name
}

output "api_gateway_domain" {
  description = "API Gateway domain name (without protocol or stage)"
  value       = "${aws_api_gateway_rest_api.api.id}.execute-api.${data.aws_region.current.name}.amazonaws.com"
}

output "api_gateway_stage" {
  description = "API Gateway stage name"
  value       = aws_api_gateway_stage.api.stage_name
}

output "clerk_publishable_key_param_name" {
  description = "SSM Parameter Store name for the Clerk publishable key. The CD workflow reads this at frontend build time."
  value       = aws_ssm_parameter.clerk_publishable_key.name
}

output "clerk_secret_key_param_name" {
  description = "SSM Parameter Store name for the Clerk secret key. The Lambda fetches this at startup via auth.LoadClerkSecret."
  value       = aws_ssm_parameter.clerk_secret_key.name
}
