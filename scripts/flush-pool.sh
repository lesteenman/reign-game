#!/usr/bin/env bash
# flush-pool.sh — delete every puzzle row from the pool table, preserving
# CONFIG items. Used during cutover (R-069 runbook step 2) to drop old-
# shape PuzzleRecord rows so the new generator's output does not share a
# table with pre-R-067 data.
#
# Works against LocalStack (set AWS_ENDPOINT_URL=http://localhost:4566)
# and real AWS (leave AWS_ENDPOINT_URL unset, use AWS_PROFILE or role
# credentials).
#
# Usage:
#   TABLE_NAME=puzzle-pool ./scripts/flush-pool.sh          # prod (real AWS)
#   TABLE_NAME=puzzle-pool \
#     AWS_ENDPOINT_URL=http://localhost:4566 \
#     ./scripts/flush-pool.sh                                # LocalStack
#
# Safe to run on a live pool: the DynamoDB delete-item call is
# idempotent. The script refuses to proceed without an explicit
# CONFIRM=YES environment variable so an accidental invocation from a
# shell history doesn't destroy the pool.

set -euo pipefail

TABLE_NAME=${TABLE_NAME:-puzzle-pool}
REGION=${AWS_REGION:-us-east-1}

if [[ "${CONFIRM:-}" != "YES" ]]; then
  echo "refusing to flush: set CONFIRM=YES to proceed" >&2
  echo "(target table: $TABLE_NAME in $REGION)" >&2
  exit 1
fi

aws_args=(--region "$REGION")
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
  aws_args+=(--endpoint-url "$AWS_ENDPOINT_URL")
fi

echo "scanning $TABLE_NAME for non-CONFIG rows..."

# Scan all rows with PK != "CONFIG". Pagination is handled by the aws
# CLI via --no-paginate loop: we drain ExclusiveStartKey until empty.
next_token=""
deleted=0
while :; do
  page_args=()
  if [[ -n "$next_token" ]]; then
    page_args+=(--starting-token "$next_token")
  fi
  page=$(aws "${aws_args[@]}" dynamodb scan \
    --table-name "$TABLE_NAME" \
    --projection-expression "PK, SK" \
    --filter-expression "PK <> :cfg" \
    --expression-attribute-values '{":cfg":{"S":"CONFIG"}}' \
    --output json "${page_args[@]}")

  keys=$(echo "$page" | jq -c '.Items[] | {PK: .PK, SK: .SK}')
  while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    aws "${aws_args[@]}" dynamodb delete-item \
      --table-name "$TABLE_NAME" \
      --key "$key" >/dev/null
    deleted=$((deleted + 1))
  done <<< "$keys"

  next_token=$(echo "$page" | jq -r '.NextToken // empty')
  if [[ -z "$next_token" ]]; then
    break
  fi
done

echo "flushed $deleted rows from $TABLE_NAME (CONFIG items preserved)"
