variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
}

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "reign-game"
}

variable "environment" {
  description = "Deployment environment (e.g., prod, staging)"
  type        = string
  default     = "prod"
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment zip file"
  type        = string
}

variable "clerk_publishable_key" {
  description = "Clerk publishable key (browser-safe). Supplied via TF_VAR_clerk_publishable_key in CI for the initial apply only — rotations happen directly in SSM (lifecycle ignore_changes). Default empty so terraform plan succeeds without the secret in CI for non-deploy contexts."
  type        = string
  sensitive   = true
  default     = ""
}

variable "clerk_secret_key" {
  description = "Clerk server-side secret key. Supplied via TF_VAR_clerk_secret_key in CI for the initial apply only — rotations happen directly in SSM (lifecycle ignore_changes). Default empty so terraform plan succeeds without the secret in CI for non-deploy contexts."
  type        = string
  sensitive   = true
  default     = ""
}
