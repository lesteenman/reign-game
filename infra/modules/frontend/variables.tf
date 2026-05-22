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

variable "domain_aliases" {
  description = "Alternate domain names for the CloudFront distribution. Only attached when acm_certificate_arn is also set — CloudFront rejects aliases without a matching cert."
  type        = list(string)
  default     = []
}

variable "acm_certificate_arn" {
  description = "ARN of the ACM cert (us-east-1) covering domain_aliases. Empty string keeps the default *.cloudfront.net certificate."
  type        = string
  default     = ""
}

variable "web_acl_arn" {
  description = "ARN of an AWS WAFv2 Web ACL (CLOUDFRONT scope, must live in us-east-1) to attach to the CloudFront distribution. Empty string leaves the distribution unprotected."
  type        = string
  default     = ""
}
