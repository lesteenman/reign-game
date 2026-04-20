#!/bin/bash
# LocalStack init script. Runs on every container start (via
# /etc/localstack/init/ready.d). Must be idempotent: when PERSISTENCE=1
# is on, the table + queues already exist on restart and their pool +
# queue messages must be preserved.
set -euo pipefail

# --- puzzle-pool DynamoDB table -------------------------------------
if awslocal dynamodb describe-table --table-name puzzle-pool >/dev/null 2>&1; then
  echo "puzzle-pool table already exists; skipping create."
else
  echo "Creating puzzle-pool DynamoDB table..."
  awslocal dynamodb create-table \
    --table-name puzzle-pool \
    --attribute-definitions \
      AttributeName=PK,AttributeType=S \
      AttributeName=SK,AttributeType=S \
    --key-schema \
      AttributeName=PK,KeyType=HASH \
      AttributeName=SK,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST
fi

# --- puzzle-generation SQS queues -----------------------------------
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
echo "Seeding CONFIG items (Phase 5 shape: threshold/enabled)..."
awslocal dynamodb put-item \
  --table-name puzzle-pool \
  --item '{
    "PK": {"S": "CONFIG"},
    "SK": {"S": "7#standard"},
    "threshold": {"N": "3"},
    "enabled": {"BOOL": true}
  }'

awslocal dynamodb put-item \
  --table-name puzzle-pool \
  --item '{
    "PK": {"S": "CONFIG"},
    "SK": {"S": "9#standard"},
    "threshold": {"N": "3"},
    "enabled": {"BOOL": true}
  }'

# 7x7 double is infeasible (N=7 k=2 has 0 solutions under 8-neighbor
# adjacency + 2 marks/row — see bench/n-feasibility.md). Not seeded.

awslocal dynamodb put-item \
  --table-name puzzle-pool \
  --item '{
    "PK": {"S": "CONFIG"},
    "SK": {"S": "9#double"},
    "threshold": {"N": "3"},
    "enabled": {"BOOL": true}
  }'

echo "LocalStack init complete: table + queues ensured, CONFIG seeds written."
