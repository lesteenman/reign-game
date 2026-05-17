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

variable "tags" {
  description = "Tags applied to every resource created by this module"
  type        = map(string)
  default     = {}
}

variable "generation_queue_arn" {
  description = "ARN of the puzzle-generation SQS queue (used to scope sqs:SendMessage IAM policy for reactive auto-replenish)."
  type        = string
}

variable "generation_queue_url" {
  description = "URL of the puzzle-generation SQS queue (passed as SQS_QUEUE_URL env var so the Lambda can publish replenish messages on approved-pool drains)."
  type        = string
}
