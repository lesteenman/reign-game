variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "api_gateway_domain" {
  description = "API Gateway domain name for the backend origin"
  type        = string
}

variable "api_gateway_stage" {
  description = "API Gateway stage name (used as origin path)"
  type        = string
}
