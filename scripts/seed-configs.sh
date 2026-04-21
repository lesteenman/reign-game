#!/usr/bin/env bash
# seed-configs.sh — write the Phase-5-shape CONFIG rows to the pool
# table. Used during cutover (R-069 runbook step 4) and safe to re-run
# at any time (put-item is idempotent).
#
# The CONFIG set mirrors .localstack/init-aws.sh:
#   - 7#standard  (threshold=3, enabled=true)
#   - 9#standard  (threshold=3, enabled=true)
#   - 9#double    (threshold=3, enabled=true)
#
# 7x7 Double is NOT seeded — N=7 k=2 is infeasible under 8-neighbor
# adjacency (see backend/internal/generator/bench/n-feasibility.md and
# the KI-007 close-out in ROADMAP.md).
#
# Requires: aws (CLI), jq.
#
# Usage:
#   TABLE_NAME=puzzle-pool ./scripts/seed-configs.sh           # prod
#   TABLE_NAME=puzzle-pool \
#     AWS_ENDPOINT_URL=http://localhost:4566 \
#     ./scripts/seed-configs.sh                                 # LocalStack

set -euo pipefail

TABLE_NAME=${TABLE_NAME:-puzzle-pool}
REGION=${AWS_REGION:-us-east-1}

aws_args=(--region "$REGION")
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
  aws_args+=(--endpoint-url "$AWS_ENDPOINT_URL")
fi

put_config() {
  local sk=$1
  echo "seed CONFIG $sk"
  # Build the item JSON via jq so SK values pass through safely even
  # if a future caller adds a field containing quotes or backslashes.
  local item
  item=$(jq -nc --arg sk "$sk" '{
    PK:        {S: "CONFIG"},
    SK:        {S: $sk},
    threshold: {N: "3"},
    enabled:   {BOOL: true}
  }')
  aws "${aws_args[@]}" dynamodb put-item \
    --table-name "$TABLE_NAME" \
    --item "$item"
}

put_config "7#standard"
put_config "9#standard"
put_config "9#double"

echo "seeded 3 CONFIG rows in $TABLE_NAME"
