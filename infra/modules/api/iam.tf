# Lambda IAM policies that depend on resources defined elsewhere in this
# module. The base role + log/SQS/DynamoDB statements live in main.tf
# alongside the Lambda function. New policies for Phase 6 admin auth
# (R-089) are added here so the SSM and KMS additions are isolated and
# easy to review.

# Read access to the two Clerk-related SSM parameters created by ssm.tf.
# Resource-scoped to those exact ARNs — no wildcard. The publishable key
# is read by the CD workflow from CI; the Lambda only needs the secret
# but receives both ARNs for symmetry and defense-in-depth (if a future
# code path wanted the publishable key for, e.g., session-claims-by-key
# verification).
resource "aws_iam_role_policy" "lambda_ssm_clerk" {
  name = "${local.function_name}-ssm-clerk"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
        ]
        Resource = [
          aws_ssm_parameter.clerk_publishable_key.arn,
          aws_ssm_parameter.clerk_secret_key.arn,
        ]
      },
    ]
  })
}

# Decrypt access for the AWS-managed `aws/ssm` KMS key. SecureString
# parameters fetched with WithDecryption=true require kms:Decrypt on the
# key that encrypted them. The aws/ssm alias points to the AWS-managed
# key in this account/region, so we look up its ARN dynamically rather
# than hardcode it.
data "aws_kms_key" "ssm" {
  key_id = "alias/aws/ssm"
}

resource "aws_iam_role_policy" "lambda_kms_ssm" {
  name = "${local.function_name}-kms-ssm"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Decrypt",
        ]
        Resource = data.aws_kms_key.ssm.arn
        Condition = {
          StringEquals = {
            "kms:ViaService" = "ssm.${data.aws_region.current.name}.amazonaws.com"
          }
        }
      },
    ]
  })
}
