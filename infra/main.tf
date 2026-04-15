module "database" {
  source = "./modules/database"

  project_name = var.project_name
  environment  = var.environment
}

module "generation" {
  source = "./modules/generation"

  project_name     = var.project_name
  environment      = var.environment
  lambda_zip_path  = var.lambda_zip_path
  puzzle_table_name = module.database.puzzle_table_name
  puzzle_table_arn  = module.database.puzzle_table_arn
}

module "api" {
  source = "./modules/api"

  project_name     = var.project_name
  environment      = var.environment
  lambda_zip_path  = var.lambda_zip_path
  puzzle_table_name = module.database.puzzle_table_name
  puzzle_table_arn  = module.database.puzzle_table_arn
  sqs_queue_url    = module.generation.queue_url
  sqs_queue_arn    = module.generation.queue_arn
}

module "frontend" {
  source = "./modules/frontend"

  project_name       = var.project_name
  environment        = var.environment
  api_gateway_domain = module.api.api_gateway_domain
  api_gateway_stage  = module.api.api_gateway_stage
}
