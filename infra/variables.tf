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
  description = "Deployment environment. Currently 'acc' — this stack serves the acceptance domain reign.acc.steenman.me. Future real-prod env (reign.steenman.me) will set this to 'prod' via per-env tfvars or a separate workspace."
  type        = string
  default     = "acc"
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

variable "domain_aliases" {
  description = "CloudFront alternate domain names (CNAME aliases) for the frontend distribution. Empty list keeps the distribution at *.cloudfront.net only. When non-empty, acm_certificate_arn MUST also be set and the cert MUST cover every entry. Set via TF_VAR_domain_aliases='[\"reign.acc.steenman.me\"]' in CI."
  type        = list(string)
  default     = []
}

variable "acm_certificate_arn" {
  description = "ARN of the ACM certificate (in us-east-1) covering domain_aliases. Empty string keeps the default *.cloudfront.net cert. The cert is provisioned in a separate Terraform project (accounts/reign-game) because it crosses AWS-account boundaries; this repo just references the ARN."
  type        = string
  default     = ""
}
