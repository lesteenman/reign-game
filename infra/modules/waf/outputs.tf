output "web_acl_arn" {
  description = "ARN of the WAFv2 Web ACL. Pass to the frontend module so the CloudFront distribution attaches it."
  value       = aws_wafv2_web_acl.cloudfront.arn
}
