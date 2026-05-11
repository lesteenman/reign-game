#!/usr/bin/env bash
# Wrapper that launches the AWS MCP proxy with a per-process AWS env.
#
# The dev container sets AWS_ACCESS_KEY_ID=test / AWS_SECRET_ACCESS_KEY=test
# globally so the backend's `go run ./cmd/api` can sign requests to LocalStack.
# Those would shadow the real SSO profile if the AWS MCP inherited them, so
# we strip them here and inject the real profile (from the project-scoped
# REIGN_AWS_PROFILE — kept out of `AWS_PROFILE` global env to avoid leaking
# into unit tests that don't expect a profile lookup).

set -euo pipefail

unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

if [ -n "${REIGN_AWS_PROFILE:-}" ]; then
  export AWS_PROFILE="$REIGN_AWS_PROFILE"
fi

exec uvx mcp-proxy-for-aws@latest "$@"
