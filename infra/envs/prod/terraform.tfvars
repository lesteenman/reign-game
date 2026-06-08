# Per-env values for the prod (production) environment.
# Secrets and the cert ARN are NOT here — the cd-prod workflow supplies
# them via TF_VAR_* from the `prod` GitHub Environment (Clerk pk_live/
# sk_live + the us-east-1 ACM cert ARN). This file holds the public,
# fixed config: the env identity (drives reign-game-prod-* names and the
# reign-game/prod state key) and the production domain alias.
environment    = "prod"
domain_aliases = ["reign.steenman.me"]
