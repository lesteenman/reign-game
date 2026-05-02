variable "name_prefix" {
  description = "Prefix for resource names (e.g. \"reign-game-prod\")"
  type        = string
}

variable "puzzle_pool_table_name" {
  description = "DynamoDB puzzle-pool table name (used as PUZZLE_POOL_TABLE env var)"
  type        = string
}

variable "puzzle_pool_table_arn" {
  description = "DynamoDB puzzle-pool table ARN (used to scope IAM policy)"
  type        = string
}

variable "lambda_zip_path" {
  description = "Path to the daily-cron Lambda deployment zip. Empty default lets terraform plan succeed in CI without the build artifact (mirrors the api/generation pattern; fileexists() guards source_code_hash)."
  type        = string
  default     = ""
}

variable "lambda_zip_hash" {
  description = "Optional override for source_code_hash. Empty default falls back to filebase64sha256(lambda_zip_path) when the file exists. Reserved for the deploy pipeline."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to every resource created by this module"
  type        = map(string)
  default     = {}
}
