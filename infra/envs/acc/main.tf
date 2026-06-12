module "database" {
  source = "../../modules/database"

  project_name = var.project_name
  environment  = var.environment
}

module "generation" {
  source = "../../modules/generation"

  project_name      = var.project_name
  environment       = var.environment
  lambda_zip_path   = var.lambda_zip_path
  puzzle_table_name = module.database.puzzle_table_name
  puzzle_table_arn  = module.database.puzzle_table_arn
}

module "api" {
  source = "../../modules/api"

  project_name          = var.project_name
  environment           = var.environment
  lambda_zip_path       = var.lambda_zip_path
  puzzle_table_name     = module.database.puzzle_table_name
  puzzle_table_arn      = module.database.puzzle_table_arn
  sqs_queue_url         = module.generation.queue_url
  sqs_queue_arn         = module.generation.queue_arn
  clerk_publishable_key = var.clerk_publishable_key
  clerk_secret_key      = var.clerk_secret_key
}

module "frontend" {
  source = "../../modules/frontend"

  project_name        = var.project_name
  environment         = var.environment
  api_gateway_domain  = module.api.api_gateway_domain
  api_gateway_stage   = module.api.api_gateway_stage
  domain_aliases      = var.domain_aliases
  acm_certificate_arn = var.acm_certificate_arn

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

module "monitoring" {
  source = "../../modules/monitoring"

  project_name = var.project_name
  environment  = var.environment

  api_function_name        = module.api.lambda_function_name
  generator_function_name  = module.generation.generator_function_name
  daily_cron_function_name = module.daily_cron.lambda_function_name

  api_gateway_name  = module.api.api_gateway_name
  api_gateway_stage = module.api.api_gateway_stage

  generation_queue_name = module.generation.queue_name
  generation_dlq_name   = module.generation.dlq_name

  puzzle_table_name          = module.database.puzzle_table_name
  cloudfront_distribution_id = module.frontend.cloudfront_distribution_id

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

module "daily_cron" {
  source = "../../modules/daily-cron"

  name_prefix            = "${var.project_name}-${var.environment}"
  puzzle_pool_table_name = module.database.puzzle_table_name
  puzzle_pool_table_arn  = module.database.puzzle_table_arn
  generation_queue_arn   = module.generation.queue_arn
  generation_queue_url   = module.generation.queue_url
  lambda_zip_path        = var.daily_cron_lambda_zip_path

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
