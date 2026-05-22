resource "aws_wafv2_web_acl" "cloudfront" {
  provider = aws.us_east_1

  name        = "${var.project_name}-${var.environment}-cloudfront"
  description = "Edge protection for ${var.project_name} (${var.environment}) — per-IP rate limit on /api/daily/*"
  scope       = "CLOUDFRONT"

  default_action {
    allow {}
  }

  rule {
    name     = "DailyPerIPRateLimit"
    priority = 1

    action {
      block {}
    }

    statement {
      rate_based_statement {
        limit                 = var.daily_rate_limit_per_5min
        aggregate_key_type    = "IP"
        evaluation_window_sec = 300

        scope_down_statement {
          byte_match_statement {
            search_string         = "/api/daily/"
            positional_constraint = "STARTS_WITH"

            field_to_match {
              uri_path {}
            }

            # URL_DECODE first so a percent-encoded path like
            # `/%61pi/daily/...` resolves to `/api/daily/...` and still
            # matches. LOWERCASE second so any case variation also
            # matches. Defense-in-depth — CloudFront's own routing
            # already wouldn't reach our origin for an encoded path
            # that doesn't match the cache behavior pattern, but the
            # transformation chain closes the theoretical bypass at
            # the WAF tier.
            text_transformation {
              priority = 0
              type     = "URL_DECODE"
            }

            text_transformation {
              priority = 1
              type     = "LOWERCASE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project_name}-${var.environment}-daily-rate-limit"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project_name}-${var.environment}-cloudfront"
    sampled_requests_enabled   = true
  }

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
