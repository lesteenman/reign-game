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
  default     = ""
}

variable "puzzle_table_arn" {
  description = "DynamoDB puzzle pool table ARN"
  type        = string
  default     = ""
}

variable "sqs_queue_url" {
  description = "SQS puzzle generation queue URL"
  type        = string
  default     = ""
}

variable "sqs_queue_arn" {
  description = "SQS puzzle generation queue ARN"
  type        = string
  default     = ""
}

variable "clerk_publishable_key" {
  description = "Clerk publishable key (browser-safe). Stored in SSM as a String parameter; the CD workflow reads it from SSM at frontend build time. Only the initial value is set via Terraform — subsequent changes happen in the SSM console (lifecycle ignore_changes)."
  type        = string
  sensitive   = true
  default     = ""
}

variable "clerk_secret_key" {
  description = "Clerk server-side secret key. Stored in SSM as a SecureString and fetched by the Lambda at startup via auth.LoadClerkSecret. Only the initial value is set via Terraform — subsequent rotations happen in the SSM console (lifecycle ignore_changes)."
  type        = string
  sensitive   = true
  default     = ""
}
