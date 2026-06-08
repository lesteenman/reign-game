terraform {
  backend "s3" {
    # Placeholders. Real values are supplied via -backend-config= flags
    # in CI/CD (.github/workflows/ci.yml, cd.yml). Required because
    # `terraform validate` checks for the presence of these arguments
    # at the source level, independent of init/state. Placeholders are
    # silently overridden by the init-time flags.
    # Tracking upstream: https://github.com/hashicorp/terraform/issues/38466
    # — once Terraform restores partial-config validation, the placeholders
    # below can revert to an empty `backend "s3" {}` block.
    bucket       = "_overridden_via_backend_config_"
    key          = "_overridden_via_backend_config_"
    region       = "eu-west-1"
    use_lockfile = true
  }
}
