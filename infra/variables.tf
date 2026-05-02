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

variable "daily_cron_lambda_zip_path" {
  description = "Path to the daily-cron Lambda deployment zip (cmd/daily-cron). Empty default keeps terraform plan green in CI without the build artifact; the deploy pipeline overrides via TF_VAR_daily_cron_lambda_zip_path."
  type        = string
  default     = ""
}

variable "daily_cron_lambda_zip_hash" {
  description = "Optional override for the daily-cron source_code_hash. Empty default falls back to filebase64sha256(daily_cron_lambda_zip_path) when the file exists."
  type        = string
  default     = ""
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
