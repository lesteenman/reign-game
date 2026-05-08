module "database" {
  source = "./modules/database"

  project_name = var.project_name
  environment  = var.environment
}

module "generation" {
  source = "./modules/generation"

  project_name      = var.project_name
  environment       = var.environment
  lambda_zip_path   = var.lambda_zip_path
  puzzle_table_name = module.database.puzzle_table_name
  puzzle_table_arn  = module.database.puzzle_table_arn
}

module "api" {
  source = "./modules/api"

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
  source = "./modules/frontend"

  project_name        = var.project_name
  environment         = var.environment
  api_gateway_domain  = module.api.api_gateway_domain
  api_gateway_stage   = module.api.api_gateway_stage
  domain_aliases      = var.domain_aliases
  acm_certificate_arn = var.acm_certificate_arn
}

module "daily_cron" {
  source = "./modules/daily-cron"

  name_prefix            = "${var.project_name}-${var.environment}"
  puzzle_pool_table_name = module.database.puzzle_table_name
  puzzle_pool_table_arn  = module.database.puzzle_table_arn
  generation_queue_arn   = module.generation.queue_arn
  generation_queue_url   = module.generation.queue_url
  lambda_zip_path        = var.daily_cron_lambda_zip_path
  lambda_zip_hash        = var.daily_cron_lambda_zip_hash

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
