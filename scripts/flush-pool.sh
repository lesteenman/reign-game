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
# Requires: aws (CLI), jq.
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

# Scan paginates server-side via LastEvaluatedKey / ExclusiveStartKey —
# raw DynamoDB API semantics. The AWS CLI's --starting-token flag is a
# CLI-side concept that pairs with --max-items, NOT a drop-in for
# ExclusiveStartKey, so we drive pagination ourselves against the raw
# keys returned by each scan response.
#
# Verified end-to-end against LocalStack with --limit 10 forcing four
# page boundaries: 30 rows deleted, LastEvaluatedKey pass-through
# correctly threads ExclusiveStartKey on each follow-up scan.
start_key=""
deleted=0
while :; do
  scan_args=(
    --table-name "$TABLE_NAME"
    --projection-expression "PK, SK"
    --filter-expression "PK <> :cfg"
    --expression-attribute-values '{":cfg":{"S":"CONFIG"}}'
    --output json
    --no-paginate
  )
  if [[ -n "$start_key" ]]; then
    scan_args+=(--exclusive-start-key "$start_key")
  fi
  page=$(aws "${aws_args[@]}" dynamodb scan "${scan_args[@]}")

  while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    aws "${aws_args[@]}" dynamodb delete-item \
      --table-name "$TABLE_NAME" \
      --key "$key" >/dev/null
    deleted=$((deleted + 1))
  done < <(echo "$page" | jq -c '.Items[]? | {PK: .PK, SK: .SK}')

  start_key=$(echo "$page" | jq -c '.LastEvaluatedKey // empty')
  if [[ -z "$start_key" ]]; then
    break
  fi
done

echo "flushed $deleted rows from $TABLE_NAME (CONFIG items preserved)"
