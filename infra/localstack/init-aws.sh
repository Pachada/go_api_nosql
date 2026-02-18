#!/usr/bin/env bash
# LocalStack init script — runs once the container is ready.
# Creates all DynamoDB tables and the S3 bucket used by the API.

set -e

ENDPOINT="http://localhost:4566"
REGION="${AWS_DEFAULT_REGION:-us-east-1}"

echo ">>> Creating DynamoDB tables..."

awslocal dynamodb create-table \
  --table-name users \
  --attribute-definitions \
    AttributeName=user_id,AttributeType=S \
    AttributeName=username,AttributeType=S \
    AttributeName=email,AttributeType=S \
  --key-schema AttributeName=user_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    '[{"IndexName":"username-index","KeySchema":[{"AttributeName":"username","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}},
      {"IndexName":"email-index","KeySchema":[{"AttributeName":"email","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}}]'

awslocal dynamodb create-table \
  --table-name sessions \
  --attribute-definitions \
    AttributeName=session_id,AttributeType=S \
    AttributeName=user_id,AttributeType=S \
  --key-schema AttributeName=session_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    '[{"IndexName":"user_id-index","KeySchema":[{"AttributeName":"user_id","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}}]'

awslocal dynamodb create-table \
  --table-name roles \
  --attribute-definitions AttributeName=role_id,AttributeType=S \
  --key-schema AttributeName=role_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
  --table-name statuses \
  --attribute-definitions AttributeName=status_id,AttributeType=S \
  --key-schema AttributeName=status_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
  --table-name devices \
  --attribute-definitions \
    AttributeName=device_id,AttributeType=S \
    AttributeName=user_id,AttributeType=S \
    AttributeName=device_uuid,AttributeType=S \
  --key-schema AttributeName=device_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    '[{"IndexName":"user_id-index","KeySchema":[{"AttributeName":"user_id","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}},
      {"IndexName":"device_uuid-index","KeySchema":[{"AttributeName":"device_uuid","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}}]'

awslocal dynamodb create-table \
  --table-name notifications \
  --attribute-definitions \
    AttributeName=notification_id,AttributeType=S \
    AttributeName=user_id,AttributeType=S \
    AttributeName=created_at,AttributeType=S \
  --key-schema AttributeName=notification_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    '[{"IndexName":"user_id-created_at-index","KeySchema":[{"AttributeName":"user_id","KeyType":"HASH"},{"AttributeName":"created_at","KeyType":"RANGE"}],"Projection":{"ProjectionType":"ALL"}}]'

awslocal dynamodb create-table \
  --table-name files \
  --attribute-definitions \
    AttributeName=file_id,AttributeType=S \
    AttributeName=uploaded_by_user_id,AttributeType=S \
  --key-schema AttributeName=file_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    '[{"IndexName":"uploaded_by_user_id-index","KeySchema":[{"AttributeName":"uploaded_by_user_id","KeyType":"HASH"}],"Projection":{"ProjectionType":"ALL"}}]'

awslocal dynamodb create-table \
  --table-name user_verifications \
  --attribute-definitions \
    AttributeName=user_id,AttributeType=S \
    AttributeName=type,AttributeType=S \
  --key-schema \
    AttributeName=user_id,KeyType=HASH \
    AttributeName=type,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST

# Enable TTL on user_verifications for automatic OTP/token expiry
awslocal dynamodb update-time-to-live \
  --table-name user_verifications \
  --time-to-live-specification "Enabled=true,AttributeName=expires_at"

awslocal dynamodb create-table \
  --table-name app_versions \
  --attribute-definitions AttributeName=version_id,AttributeType=S \
  --key-schema AttributeName=version_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

echo ">>> Creating S3 bucket..."
awslocal s3 mb s3://${S3_BUCKET_NAME:-go-api-files}

echo ">>> LocalStack init complete."
