terraform {
  backend "s3" {
    # All values provided via -backend-config flags or a .tfbackend file:
    #   bucket         = "..."
    #   key            = "..."
    #   region         = "..."
    #   dynamodb_table = "..."
    #   encrypt        = true
  }
}
