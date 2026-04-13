module "frontend" {
  source = "./modules/frontend"

  project_name = var.project_name
  environment  = var.environment
}

module "api" {
  source = "./modules/api"

  project_name    = var.project_name
  environment     = var.environment
  lambda_zip_path = var.lambda_zip_path
}
