#!/bin/bash
# LocalStack init script. Runs on every container start (via
# /etc/localstack/init/ready.d). Must be idempotent: when PERSISTENCE=1
# is on, the tables + queues already exist on restart and their pool +
# queue messages must be preserved.
set -euo pipefail

# --- DynamoDB tables -------------------------------------------------
# Two tables with identical schema:
#   puzzle-pool      — dev pool, served to the app at :5181 / :5180.
#   puzzle-pool-e2e  — e2e fixtures, served to the test backend at :5182.
# R-06B keeps them separate so an e2e run never touches a dev pool and
# vice versa.
create_table_if_missing() {
  local name=$1
  if awslocal dynamodb describe-table --table-name "$name" >/dev/null 2>&1; then
    echo "$name table already exists; skipping create."
  else
    echo "Creating $name DynamoDB table..."
    awslocal dynamodb create-table \
      --table-name "$name" \
      --attribute-definitions \
        AttributeName=PK,AttributeType=S \
        AttributeName=SK,AttributeType=S \
      --key-schema \
        AttributeName=PK,KeyType=HASH \
        AttributeName=SK,KeyType=RANGE \
      --billing-mode PAY_PER_REQUEST
  fi
}

create_table_if_missing puzzle-pool
create_table_if_missing puzzle-pool-e2e

# --- puzzle-generation SQS queues -----------------------------------
# Only one queue — the e2e backend does not run a generator worker
# (fixtures are pre-seeded), so it does not need its own queue.
create_queue_if_missing() {
  local name=$1
  shift
  if awslocal sqs get-queue-url --queue-name "$name" >/dev/null 2>&1; then
    echo "SQS queue $name already exists; skipping create."
  else
    echo "Creating SQS queue $name..."
    awslocal sqs create-queue --queue-name "$name" "$@"
  fi
}

create_queue_if_missing puzzle-generation-dlq

create_queue_if_missing puzzle-generation \
  --attributes '{
    "VisibilityTimeout": "900",
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:puzzle-generation-dlq\",\"maxReceiveCount\":\"3\"}"
  }'

# --- CONFIG seed items ----------------------------------------------
# put-item is idempotent (unconditional put). We always write the seed
# shape so a code-side schema change lands on the next restart.
# Persisted puzzle rows (PK=<size>#<mode>, SK=<uuid>) are NOT touched;
# only the CONFIG rows are overwritten.
#
# Dev table: 7#standard, 9#standard, 9#double all enabled.
# E2E table: 9#double is intentionally DISABLED so
#   frontend/playwright/e2e/dynamic-modes.spec.ts can assert the
#   landing page button-list filters by `enabled`.
# 7x7 double is infeasible (N=7 k=2 has 0 solutions under 8-neighbor
# adjacency + 2 marks/row — see bench/n-feasibility.md). Not seeded
# in either table.
seed_config() {
  local table=$1 sk=$2 enabled=$3
  awslocal dynamodb put-item --table-name "$table" --item "{
    \"PK\": {\"S\": \"CONFIG\"},
    \"SK\": {\"S\": \"$sk\"},
    \"threshold\": {\"N\": \"3\"},
    \"enabled\": {\"BOOL\": $enabled}
  }"
}

echo "Seeding CONFIG items into puzzle-pool..."
seed_config puzzle-pool "7#standard" true
seed_config puzzle-pool "9#standard" true
seed_config puzzle-pool "9#double" true

echo "Seeding CONFIG items into puzzle-pool-e2e..."
seed_config puzzle-pool-e2e "7#standard" true
seed_config puzzle-pool-e2e "9#standard" true
seed_config puzzle-pool-e2e "9#double" false

echo "LocalStack init complete: tables + queues ensured, CONFIG seeds written."
