#!/bin/bash
set -euo pipefail

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

echo "Creating puzzle-generation SQS queue..."
awslocal sqs create-queue \
  --queue-name puzzle-generation-dlq

awslocal sqs create-queue \
  --queue-name puzzle-generation \
  --attributes '{
    "VisibilityTimeout": "900",
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:puzzle-generation-dlq\",\"maxReceiveCount\":\"3\"}"
  }'

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

awslocal dynamodb put-item \
  --table-name puzzle-pool \
  --item '{
    "PK": {"S": "CONFIG"},
    "SK": {"S": "9#double"},
    "threshold": {"N": "3"},
    "enabled": {"BOOL": true}
  }'

echo "LocalStack init complete: puzzle-pool table, puzzle-generation queue, and CONFIG items created."
