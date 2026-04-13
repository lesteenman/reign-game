terraform {
  backend "s3" {
    # All values provided via -backend-config flags or a .tfbackend file:
    #   bucket       = "..."
    #   key          = "..."
    #   region       = "..."
    #   use_lockfile = true
  }
}
