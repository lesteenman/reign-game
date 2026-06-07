locals {
  table_name = "${var.project_name}-${var.environment}-puzzle-pool"
}

resource "aws_dynamodb_table" "puzzle_pool" {
  name         = local.table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  # Sparse ready-index key: written only while a puzzle is "ready" (carries
  # the PK value). Backs the ready-index GSI so CountReady/NextReady read
  # only ready rows instead of scanning a partition and filtering.
  attribute {
    name = "readyPoolKey"
    type = "S"
  }

  global_secondary_index {
    name            = "ready-index"
    hash_key        = "readyPoolKey"
    projection_type = "ALL"
  }

  # Explicit at-rest encryption with the AWS-owned key. Functionally
  # identical to the AWS default, but declared so audit/compliance tooling
  # sees it and so the setting can't silently drift.
  server_side_encryption {
    enabled = true
  }

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
