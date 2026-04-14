module "api" {
  source = "./modules/api"

  project_name    = var.project_name
  environment     = var.environment
  lambda_zip_path = var.lambda_zip_path
}

module "frontend" {
  source = "./modules/frontend"

  project_name       = var.project_name
  environment        = var.environment
  api_gateway_domain = module.api.api_gateway_domain
  api_gateway_stage  = module.api.api_gateway_stage
}
