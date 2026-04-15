variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment zip file"
  type        = string
}

variable "puzzle_table_name" {
  description = "DynamoDB puzzle pool table name"
  type        = string
}

variable "puzzle_table_arn" {
  description = "DynamoDB puzzle pool table ARN"
  type        = string
}
