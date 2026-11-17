#!/bin/sh
# Creates the DynamoDB tables in DynamoDB Local. Idempotent.
set -eu

endpoint="${DYNAMODB_ENDPOINT:-http://dynamodb:8000}"

if aws dynamodb describe-table --endpoint-url "$endpoint" --table-name sync_cursors >/dev/null 2>&1; then
  echo "sync_cursors already exists"
else
  aws dynamodb create-table \
    --endpoint-url "$endpoint" \
    --table-name sync_cursors \
    --attribute-definitions AttributeName=pk,AttributeType=S \
    --key-schema AttributeName=pk,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null
  echo "created sync_cursors"
fi
