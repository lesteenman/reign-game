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

  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
