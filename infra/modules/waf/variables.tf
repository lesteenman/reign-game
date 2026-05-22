variable "project_name" {
  type        = string
  description = "Project name slug used to name AWS resources."
}

variable "environment" {
  type        = string
  description = "Environment name (e.g. prod, acc). Suffixes the Web ACL name."
}

variable "daily_rate_limit_per_5min" {
  type        = number
  description = "Per-IP request cap on /api/daily/* over a 5-minute rolling window. Requests above the cap are blocked with 403. Minimum 100 per AWS WAFv2."
  default     = 100

  validation {
    condition     = var.daily_rate_limit_per_5min >= 100
    error_message = "AWS WAFv2 rate-based rules require a minimum of 100 requests per 5-minute window."
  }
}
