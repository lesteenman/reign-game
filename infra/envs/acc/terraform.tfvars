# Per-env values for the acc (acceptance) environment.
# Secrets and domain/cert wiring are NOT here — CD/CI supply them via
# TF_VAR_* env and -var flags from the `acc` GitHub Environment (and the
# CI_-prefixed repo copies). This file holds only the env identity, which
# drives every resource name (reign-game-acc-*) and the state key
# reign-game/acc. environment="acc" matches the variable default, so this
# file is explicit documentation, not a behavior change.
environment = "acc"
