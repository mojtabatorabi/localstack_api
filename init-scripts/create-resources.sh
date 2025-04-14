#!/bin/bash
# init-scripts/create-resources.sh

# Wait for LocalStack to be ready
./wait-for-it.sh localstack:4566 -t 30

# AWS CLI with LocalStack endpoint
awslocal() {
  aws --endpoint-url=http://localhost:4566 "$@"
}

echo "Creating S3 bucket..."
awslocal s3 mb s3://file-uploads

echo "Creating SQS queues..."
awslocal sqs create-queue --queue-name file-processing-queue
awslocal sqs create-queue --queue-name message-queue

echo "Creating Cognito User Pool..."
USER_POOL_ID=$(awslocal cognito-idp create-user-pool \
  --pool-name UserPool \
  --auto-verified-attributes email \
  --schema Name=email,Required=true \
  --query 'UserPool.Id' \
  --output text)
echo "Created Cognito User Pool: $USER_POOL_ID"

echo "Creating Cognito App Client..."
CLIENT_ID=$(awslocal cognito-idp create-user-pool-client \
  --user-pool-id $USER_POOL_ID \
  --client-name AppClient \
  --no-generate-secret \
  --explicit-auth-flows ALLOW_USER_PASSWORD_AUTH ALLOW_REFRESH_TOKEN_AUTH \
  --query 'UserPoolClient.ClientId' \
  --output text)
echo "Created Cognito Client: $CLIENT_ID"

# Export values for docker-compose
export USER_POOL_ID=$USER_POOL_ID
export CLIENT_ID=$CLIENT_ID

echo "Creating test user..."
awslocal cognito-idp admin-create-user \
  --user-pool-id $USER_POOL_ID \
  --username test@example.com \
  --temporary-password Test@123 \
  --desired-delivery-mediums EMAIL \
  --message-action SUPPRESS \
  --user-attributes Name=email,Value=test@example.com

echo "Deploying File Processor Lambda..."
awslocal lambda create-function \
  --function-name file-processor \
  --runtime go1.x \
  --handler main \
  --zip-file fileb:///docker-entrypoint-initaws.d/file-processor.zip \
  --role arn:aws:iam::000000000000:role/lambda-role

echo "Deploying Message Processor Lambda..."
awslocal lambda create-function \
  --function-name message-processor \
  --runtime go1.x \
  --handler main \
  --zip-file fileb:///docker-entrypoint-initaws.d/message-processor.zip \
  --role arn:aws:iam::000000000000:role/lambda-role

echo "Creating API Gateway..."
API_ID=$(awslocal apigateway create-rest-api \
  --name 'API Gateway' \
  --query 'id' \
  --output text)

echo "Setting up event triggers..."
awslocal lambda create-event-source-mapping \
  --function-name file-processor \
  --event-source-arn arn:aws:sqs:us-east-1:000000000000:file-processing-queue

awslocal lambda create-event-source-mapping \
  --function-name message-processor \
  --event-source-arn arn:aws:sqs:us-east-1:000000000000:message-queue

echo "AWS resources created successfully!"