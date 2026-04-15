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

echo "LocalStack init complete: puzzle-pool table and puzzle-generation queue created."
