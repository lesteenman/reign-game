terraform {
  required_version = ">= 1.14"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# us-east-1 alias for resources that must live there regardless of the
# project's primary region — AWS WAFv2 Web ACLs scoped to CloudFront
# and ACM certs for CloudFront are the canonical examples.
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}
