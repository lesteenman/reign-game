terraform {
  backend "s3" {
    # Placeholders. Real values are supplied via -backend-config= flags
    # in the cd-prod workflow (.github/workflows/cd-prod.yml). The prod
    # state key is reign-game/prod (vs reign-game/acc), isolating prod
    # state from acc by construction. Required because `terraform validate`
    # checks for the presence of these arguments at the source level,
    # independent of init/state. Placeholders are silently overridden by
    # the init-time flags.
    bucket       = "_overridden_via_backend_config_"
    key          = "_overridden_via_backend_config_"
    region       = "eu-west-1"
    use_lockfile = true
  }
}
