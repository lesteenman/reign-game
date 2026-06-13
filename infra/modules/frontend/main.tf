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

  tags = var.tags
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Explicit at-rest encryption with SSE-S3 (AES256). The bucket holds
# public static frontend assets (no secrets), so the AWS-owned SSE-S3 key
# is the right fit — SSE-KMS would add key cost + per-request KMS calls +
# a kms:Decrypt grant on the OAC path for no compliance benefit.
# Functionally identical to the AWS default; declared for audit visibility
# and drift prevention.
resource "aws_s3_bucket_server_side_encryption_configuration" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Origin Access Control for CloudFront -> S3
resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${var.project_name}-${var.environment}-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# Security response headers attached to every cache behavior. CSP is
# enforced (not report-only) — an empty acc environment lets us catch
# breakages immediately rather than promote from report-only weeks
# later. HSTS deliberately skips the `preload` directive: the preload
# list is a near-permanent commitment and the acc domain may move.
# X-Frame-Options DENY is kept alongside CSP `frame-ancestors 'none'`
# for older clients that don't honor CSP.
#
# The CSP allowlist covers:
#   - script: Clerk FAPI (`*.clerk.com`, `*.clerk.accounts.dev`) +
#     Cloudflare Turnstile (`challenges.cloudflare.com`) which Clerk
#     uses for bot defence on sign-up
#   - connect: same Clerk hosts plus `clerk-telemetry.com` (+ subdomains)
#     which the Clerk JS SDK pings asynchronously
#   - frame: same Clerk hosts + Turnstile challenge frame
#   - style 'unsafe-inline': Clerk's mounted sign-in / user-button
#     widgets inject inline styles at runtime; Tamagui's compiler
#     statically extracts ours, but the Clerk widget DOM lives inside
#     the same document. Tightening to nonces requires opting both
#     into a nonce pipeline (deferred)
#   - font: Google Fonts (`fonts.gstatic.com`) for Nunito Sans + Space
#     Mono, both imported from index.css; `data:` for inline glyphs
#   - style: Google Fonts CSS host (`fonts.googleapis.com`) for the
#     `@import` in index.css
#   - img: Clerk avatar / image CDNs plus `data:` for inline icons
#   - worker: PWA service worker (`/sw.js`); `blob:` covers Workbox-
#     generated workers + Clerk worker bootstrap
#   - manifest: PWA Web App Manifest
#
# Future tightening targets when traffic ramps:
#   - Replace `'unsafe-inline'` in style-src with nonces or hashes
#   - Add a `report-uri` / `report-to` endpoint to surface violations
#   - Add `preload` to HSTS once the prod domain is stable
resource "aws_cloudfront_response_headers_policy" "security" {
  name = "${var.project_name}-${var.environment}-security"

  security_headers_config {
    strict_transport_security {
      access_control_max_age_sec = 63072000
      include_subdomains         = true
      preload                    = false
      override                   = true
    }

    content_type_options {
      override = true
    }

    frame_options {
      frame_option = "DENY"
      override     = true
    }

    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = true
    }

    content_security_policy {
      content_security_policy = join("; ", [
        "default-src 'self'",
        "script-src 'self' https://*.clerk.com https://*.clerk.accounts.dev https://challenges.cloudflare.com",
        "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
        "font-src 'self' data: https://fonts.gstatic.com",
        "img-src 'self' data: https://img.clerk.com https://images.clerk.dev",
        "connect-src 'self' https://*.clerk.com https://*.clerk.accounts.dev https://clerk-telemetry.com https://*.clerk-telemetry.com",
        "frame-src 'self' https://*.clerk.com https://challenges.cloudflare.com",
        "worker-src 'self' blob:",
        "manifest-src 'self'",
        "object-src 'none'",
        "base-uri 'self'",
        "form-action 'self'",
        "frame-ancestors 'none'",
        "upgrade-insecure-requests",
      ])
      override = true
    }
  }

  # Permissions-Policy has no first-class block in the AWS provider —
  # ship via custom_headers_config. Disables sensors and payment APIs
  # the puzzle game has no reason to access; reduces the impact of a
  # successful XSS by removing the relevant attack surface.
  custom_headers_config {
    items {
      header   = "Permissions-Policy"
      value    = "camera=(), microphone=(), geolocation=(), payment=(), usb=(), accelerometer=(), gyroscope=(), magnetometer=(), interest-cohort=(), browsing-topics=()"
      override = true
    }
  }
}

# Origin request policy for the /api/* behavior: forwards exactly what the
# backend needs and nothing that would break the API Gateway custom origin.
# Headers: Authorization (Clerk bearer) + x-api-key (API Gateway usage-plan
# key the frontend sends) + X-Device-Id (anonymous identity — the daily and
# other anonymous endpoints 401 without it). Cookies: the two Clerk session
# cookies. Query strings: all. Host is deliberately NOT forwarded — a
# forwarded Host would not match the API Gateway domain and would 403 at the
# origin. Note: the AWS provider has no `tags` argument on origin request
# policies.
resource "aws_cloudfront_origin_request_policy" "api" {
  name = "${var.project_name}-${var.environment}-api-origin"

  headers_config {
    header_behavior = "whitelist"
    headers {
      items = ["Authorization", "x-api-key", "X-Device-Id"]
    }
  }

  cookies_config {
    cookie_behavior = "whitelist"
    cookies {
      items = ["__session", "__client_uat"]
    }
  }

  query_strings_config {
    query_string_behavior = "all"
  }
}

# SPA router: rewrites navigation routes to /index.html at the edge for the S3
# behavior, leaving /api/* and static assets with real status codes. Associated
# with the default cache behavior below.
resource "aws_cloudfront_function" "spa_router" {
  name    = "${var.project_name}-${var.environment}-spa-router"
  runtime = "cloudfront-js-2.0"
  comment = "Rewrite SPA navigation routes to /index.html; leave /api/* and assets untouched."
  publish = true
  code    = file("${path.module}/spa-router.js")
}

# CloudFront distribution
resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  default_root_object = "index.html"
  comment             = "${var.project_name} ${var.environment} frontend"

  tags = var.tags

  # Only attach aliases when an ACM cert is supplied. CloudFront rejects
  # aliases without a matching cert, so this guard makes the "ARN not set
  # yet" intermediate state apply cleanly (no aliases, default cert).
  aliases = var.acm_certificate_arn != "" ? var.domain_aliases : []

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

  # Route /api/* to API Gateway. Caching is disabled (managed
  # CachingDisabled policy), and forwarding is controlled by the custom
  # origin request policy below: Authorization + x-api-key headers, the
  # Clerk session cookies (__session, __client_uat), and all query strings
  # reach the Lambda. The whitelists are deliberately narrow — a managed
  # all-headers/all-cookies policy would forward Host (breaking the API
  # Gateway custom origin) and unrelated cookies.
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "api-gateway"
    viewer_protocol_policy = "redirect-to-https"

    cache_policy_id            = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed CachingDisabled
    origin_request_policy_id   = aws_cloudfront_origin_request_policy.api.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
  }

  # PWA service worker — never cache at the edge. The browser registers
  # /sw.js once per page load; if CloudFront serves a stale SW, users
  # stay on outdated client code until the TTL expires (default policy
  # is 24h). Managed CachingDisabled forces revalidation on every
  # request. Same logic applies to the Workbox runtime bundles. See GH #116.
  ordered_cache_behavior {
    path_pattern               = "/sw.js"
    allowed_methods            = ["GET", "HEAD", "OPTIONS"]
    cached_methods             = ["GET", "HEAD"]
    target_origin_id           = "s3-frontend"
    viewer_protocol_policy     = "redirect-to-https"
    cache_policy_id            = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed CachingDisabled
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
  }

  ordered_cache_behavior {
    path_pattern               = "/workbox-*.js"
    allowed_methods            = ["GET", "HEAD", "OPTIONS"]
    cached_methods             = ["GET", "HEAD"]
    target_origin_id           = "s3-frontend"
    viewer_protocol_policy     = "redirect-to-https"
    cache_policy_id            = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed CachingDisabled
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
  }

  default_cache_behavior {
    allowed_methods            = ["GET", "HEAD", "OPTIONS"]
    cached_methods             = ["GET", "HEAD"]
    target_origin_id           = "s3-frontend"
    viewer_protocol_policy     = "redirect-to-https"
    compress                   = true                                   # gzip/brotli the app bundle (CachingOptimized keys on Accept-Encoding)
    cache_policy_id            = "658327ea-f89d-4fab-a63d-7e88639e58f6" # CachingOptimized managed policy
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id

    # SPA routing: rewrite navigation routes to /index.html at the edge.
    # This replaces distribution-wide custom_error_response 403/404 ->
    # /index.html, which also rewrote /api/* error responses to 200 and masked
    # real API status codes. Scoped to this S3 behavior only, so /api/* keeps
    # its real status (e.g. keyless -> 403) and missing assets return 404.
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_router.arn
    }
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
