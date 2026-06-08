# One-time adoption of the CloudWatch log groups the Lambdas auto-created
# (with "Never expire" retention) before #162 declared them explicitly.
# Without these, the first `terraform apply` after #162 fails with
# "log group already exists" because the group name already exists in AWS
# but not in Terraform state. With them, apply imports the existing group
# and then applies the 30-day retention in the same run.
#
# Idempotent: once a group is in state the import is a no-op, so the blocks
# are safe to leave (they can be removed after the first successful apply).
#
# These assume the groups already exist — true for acc, whose Lambdas have
# run. The prod env (../prod) has no such import blocks: its Lambdas have
# never been invoked, so there is no group to adopt on the first apply.
import {
  to = module.api.aws_cloudwatch_log_group.api
  id = "/aws/lambda/${var.project_name}-${var.environment}-api"
}

import {
  to = module.generation.aws_cloudwatch_log_group.generator
  id = "/aws/lambda/${var.project_name}-${var.environment}-generator"
}

import {
  to = module.daily_cron.aws_cloudwatch_log_group.daily_cron
  id = "/aws/lambda/${var.project_name}-${var.environment}-daily-cron"
}
