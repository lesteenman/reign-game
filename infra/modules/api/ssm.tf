# Clerk authentication secrets.
#
# The Lambda reads the secret key via auth.LoadClerkSecret, which prefers
# CLERK_SECRET_KEY from env (dev path) and falls back to fetching the
# parameter named in CLERK_SECRET_PARAM_NAME from SSM (prod path). The
# secret value never lives in a Lambda env var because env vars are
# readable via lambda:GetFunctionConfiguration.
#
# Both parameters use lifecycle { ignore_changes = [value] } so the human
# can rotate keys directly in the SSM console without Terraform reverting
# the change on the next apply. The initial value is supplied via the
# clerk_publishable_key / clerk_secret_key root variables (TF_VAR_* in CI).

# SSM parameter names follow the path convention `/reign/<env>/<key>`.
# The `reign` namespace is the short product name (var.project_name is
# "reign-game" — the longer slug used for AWS resource names — but SSM
# paths conventionally use the shorter human-friendly product name).
locals {
  ssm_namespace = "reign"
}

resource "aws_ssm_parameter" "clerk_publishable_key" {
  name        = "/${local.ssm_namespace}/${var.environment}/clerk-publishable-key"
  description = "Clerk publishable key (browser-safe). Read by CD workflow at frontend build time."
  type        = "String"
  value       = var.clerk_publishable_key

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "clerk_secret_key" {
  name        = "/${local.ssm_namespace}/${var.environment}/clerk-secret-key"
  description = "Clerk server-side secret key. Read by the API Lambda via SSM GetParameter at startup."
  type        = "SecureString"
  value       = var.clerk_secret_key

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }

  lifecycle {
    ignore_changes = [value]
  }
}
