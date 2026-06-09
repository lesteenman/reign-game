variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment (e.g. acc, prod)"
  type        = string
}

variable "tags" {
  description = "Tags applied to every resource created by this module"
  type        = map(string)
  default     = {}
}

variable "api_function_name" {
  description = "API Lambda function name (AWS/Lambda FunctionName dimension)"
  type        = string
}

variable "generator_function_name" {
  description = "Generator Lambda function name (AWS/Lambda FunctionName dimension)"
  type        = string
}

variable "daily_cron_function_name" {
  description = "Daily-cron Lambda function name (AWS/Lambda FunctionName dimension)"
  type        = string
}

variable "api_gateway_name" {
  description = "REST API name (AWS/ApiGateway ApiName dimension)"
  type        = string
}

variable "api_gateway_stage" {
  description = "API Gateway stage name (AWS/ApiGateway Stage dimension)"
  type        = string
}

variable "generation_dlq_name" {
  description = "Generation dead-letter queue name (AWS/SQS QueueName dimension)"
  type        = string
}

variable "generation_queue_name" {
  description = "Generation main queue name (AWS/SQS QueueName dimension)"
  type        = string
}

variable "puzzle_table_name" {
  description = "DynamoDB puzzle-pool table name (AWS/DynamoDB TableName dimension)"
  type        = string
}

variable "cloudfront_distribution_id" {
  description = "CloudFront distribution ID (AWS/CloudFront DistributionId dimension)"
  type        = string
}
