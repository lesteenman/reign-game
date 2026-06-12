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
  description = "Deployment environment. 'prod' — this stack serves the production domain reign.steenman.me. Resources are reign-game-prod-* in the same AWS account as acc, isolated by name + the reign-game/prod state prefix."
  type        = string
  default     = "prod"
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment zip file"
  type        = string
}

variable "daily_cron_lambda_zip_path" {
  description = "Path to the daily-cron Lambda deployment zip (cmd/daily-cron). Empty default keeps terraform plan/validate green without the build artifact; the deploy pipeline overrides via TF_VAR_daily_cron_lambda_zip_path."
  type        = string
  default     = ""
}

variable "clerk_publishable_key" {
  description = "Clerk publishable key (browser-safe), from the prod Clerk tenant (pk_live_*). Supplied via TF_VAR_clerk_publishable_key from the prod GitHub Environment for the initial apply only — rotations happen directly in SSM (lifecycle ignore_changes). Default empty keeps validate green before go-live."
  type        = string
  sensitive   = true
  default     = ""
}

variable "clerk_secret_key" {
  description = "Clerk server-side secret key, from the prod Clerk tenant (sk_live_*). Supplied via TF_VAR_clerk_secret_key from the prod GitHub Environment for the initial apply only — rotations happen directly in SSM (lifecycle ignore_changes). Default empty keeps validate green before go-live."
  type        = string
  sensitive   = true
  default     = ""
}

variable "domain_aliases" {
  description = "CloudFront alternate domain names (CNAME aliases) for the prod frontend distribution. Set to [\"reign.steenman.me\"] in terraform.tfvars. When non-empty, acm_certificate_arn MUST also be set and the cert MUST cover every entry."
  type        = list(string)
  default     = []
}

variable "acm_certificate_arn" {
  description = "ARN of the ACM certificate (in us-east-1) covering domain_aliases. Supplied via TF_VAR_acm_certificate_arn from the prod GitHub Environment. The cert is provisioned in a separate Terraform project (accounts/reign-game) because it crosses AWS-account boundaries; this repo just references the ARN. Empty default keeps validate green before go-live."
  type        = string
  default     = ""
}
