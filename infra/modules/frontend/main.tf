locals {
  bucket_name = "${var.project_name}-${var.environment}-frontend"
}

# S3 bucket for frontend static files (private, no public access)
resource "aws_s3_bucket" "frontend" {
  bucket = local.bucket_name

  # Required to allow Terraform to delete the bucket when the bucket
  # name changes (e.g. an environment rename) — without this, destroy
  # fails on the existing build artifacts. CD re-syncs the frontend
  # to the recreated bucket on the next apply.
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Origin Access Control for CloudFront -> S3
resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${var.project_name}-${var.environment}-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# CloudFront distribution
resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  default_root_object = "index.html"
  comment             = "${var.project_name} ${var.environment} frontend"

  # Only attach aliases when an ACM cert is supplied. CloudFront rejects
  # aliases without a matching cert, so this guard makes the "ARN not set
  # yet" intermediate state apply cleanly (no aliases, default cert).
  aliases = var.acm_certificate_arn != "" ? var.domain_aliases : []

  # Optional WAFv2 Web ACL (CLOUDFRONT scope, must live in us-east-1).
  # Provisioned by the `waf` module at the root. Empty string leaves the
  # distribution unprotected — useful for the "ARN not set yet" bootstrap
  # state.
  web_acl_id = var.web_acl_arn

  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = "s3-frontend"
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  origin {
    domain_name = var.api_gateway_domain
    origin_id   = "api-gateway"
    origin_path = "/${var.api_gateway_stage}"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  # Route /api/* to API Gateway (no caching, forward all methods + query strings).
  # The Authorization header is forwarded so credentials reach the Lambda.
  # Clerk session cookies (__session, __client_uat) are forwarded so the
  # backend's RequireAuth middleware can verify the Clerk session. Only the
  # Clerk-specific cookies are whitelisted to avoid forwarding unrelated
  # cookies that would disrupt CloudFront's cache
  # key for any future cacheable /api/* response. min_ttl/default_ttl/max_ttl
  # are 0, so cache hit ratios on /api/* aren't affected anyway; whitelisting
  # is defense-in-depth.
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "api-gateway"
    viewer_protocol_policy = "redirect-to-https"

    forwarded_values {
      query_string = true
      headers      = ["Authorization"]
      cookies {
        forward           = "whitelist"
        whitelisted_names = ["__session", "__client_uat"]
      }
    }

    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0
  }

  # PWA service worker — never cache at the edge. The browser registers
  # /sw.js once per page load; if CloudFront serves a stale SW, users
  # stay on outdated client code until the TTL expires (default policy
  # is 24h). Managed CachingDisabled forces revalidation on every
  # request. Same logic applies to the Workbox runtime bundles. See GH #116.
  ordered_cache_behavior {
    path_pattern           = "/sw.js"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    cache_policy_id        = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed CachingDisabled
  }

  ordered_cache_behavior {
    path_pattern           = "/workbox-*.js"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    cache_policy_id        = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed CachingDisabled
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    cache_policy_id        = "658327ea-f89d-4fab-a63d-7e88639e58f6" # CachingOptimized managed policy
  }

  # SPA fallback: serve index.html for 404s
  custom_error_response {
    error_code         = 403
    response_code      = 200
    response_page_path = "/index.html"
  }

  custom_error_response {
    error_code         = 404
    response_code      = 200
    response_page_path = "/index.html"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  # When acm_certificate_arn is empty, fall back to the default
  # *.cloudfront.net cert (unblocks first-time apply / non-prod envs).
  # When set, switch to SNI-only ACM with the modern protocol baseline.
  viewer_certificate {
    cloudfront_default_certificate = var.acm_certificate_arn == ""
    acm_certificate_arn            = var.acm_certificate_arn != "" ? var.acm_certificate_arn : null
    ssl_support_method             = var.acm_certificate_arn != "" ? "sni-only" : null
    minimum_protocol_version       = var.acm_certificate_arn != "" ? "TLSv1.2_2021" : null
  }
}

# Bucket policy allowing CloudFront OAC to read from S3
resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowCloudFrontOAC"
        Effect = "Allow"
        Principal = {
          Service = "cloudfront.amazonaws.com"
        }
        Action   = "s3:GetObject"
        Resource = "${aws_s3_bucket.frontend.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.frontend.arn
          }
        }
      }
    ]
  })
}
