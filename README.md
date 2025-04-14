# AWS LocalStack Project with Go

This project demonstrates a complete serverless architecture using:
- API Gateway for RESTful endpoints
- Lambda functions written in Go
- S3 for object storage
- SQS for message queuing
- Cognito for authentication and authorization
- PostgreSQL for persistent storage
- LocalStack for local AWS service emulation
- Docker Compose for orchestration

## Project Structure

```
aws-localstack-golang/
├── docker-compose.yml             # Docker Compose configuration
├── Makefile                       # Build and deployment automation
├── README.md                      # This file
├── init-scripts/                  # Initialization scripts
│   ├── create-resources.sh        # AWS resources setup
│   ├── init-db.sql                # PostgreSQL setup
│   └── wait-for-it.sh             # Service readiness check
├── api/                           # Go API service
│   ├── Dockerfile                 # API Docker build file
│   ├── go.mod                     # Go module definition
│   ├── go.sum                     # Go module checksums
│   ├── main.go                    # API entry point
│   ├── handlers/                  # API endpoint handlers
│   │   ├── auth.go                # Authentication endpoints
│   │   ├── files.go               # File management endpoints
│   │   └── messages.go            # Message handling endpoints
│   ├── middleware/                # API middleware
│   │   └── cognito.go             # Cognito authentication
│   └── models/                    # Data models
│       ├── file.go                # File model
│       └── message.go             # Message model
└── lambda/                        # Lambda functions
    ├── file-processor/            # File processing Lambda
    │   ├── go.mod                 # Module definition
    │   ├── go.sum                 # Module checksums
    │   └── main.go                # Lambda code
    └── message-processor/         # Message processing Lambda
        ├── go.mod                 # Module definition
        ├── go.sum                 # Module checksums
        └── main.go                # Lambda code
```

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.18 or later
- AWS CLI (for development)

### Setup and Run

1. Clone the repository
   ```
   git clone <repository-url>
   cd aws-localstack-golang
      ```

2. Build and start the services
   ```
   ./init-scripts/create-resources.sh  *run script* 
   make start
 
   cd /api
   go run main.go  // running server on port 8081
   ```
   This will:
   - Build the Go API and Lambda functions
   - Package the Lambda functions as ZIP files
   - Start the Docker containers (LocalStack, PostgreSQL, API)
   - Execute the initialization scripts to set up AWS resources

3. Verify the setup
   ```
   make health-check
   ```

### API Endpoints

All protected endpoints require a Bearer token from Cognito in the Authorization header.

#### Authentication

- `POST /auth/register` - Register a new user
  ```json
  {
    "username": "newuser",
    "password": "password123",
    "email": "user@example.com"
  }
  ```

- `POST /auth/login` - Login and get access token
  ```json
  {
    "username": "newuser",
    "password": "password123"
  }
  ```

#### Files

- `POST /api/files` - Upload a file (multipart/form-data with file field)
- `GET /api/files` - List all files for the authenticated user
- `GET /api/files/:id` - Get a specific file with download URL
- `DELETE /api/files/:id` - Delete a file

#### Messages

- `POST /api/messages` - Send a message
  ```json
  {
    "message": "Hello, world!"
  }
  ```
- `GET /api/messages` - List all messages for the authenticated user
- `GET /api/messages/:id` - Get a specific message

### Testing the API

1. Register a user
   ```bash
   curl -X POST http://localhost:8081/api/auth/register \
     -H "Content-Type: application/json" \
     -d '{"username": "test@example.com", "password": "password123", "email": "test@example.com"}'
   ```

2. Login to get a token
   ```bash
   curl -X POST http://localhost:8081/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username": "test@example.com", "password": "password123"}'
   ```

3. Upload a file
   ```bash
   TOKEN="your-access-token"
   curl -X POST http://localhost:8081/api/files \
     -H "Authorization: Bearer $TOKEN" \
     -F "file=@file.txt"
   ```

4. List files
   ```bash
   curl -X GET http://localhost:8081/api/files \
     -H "Authorization: Bearer $TOKEN" | jq
   ```

5. Send a message
   ```bash
   curl -X POST http://localhost:8081/api/messages \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"message":"Hello from LocalStack!"}' | jq
   ```

6. List messages
   ```bash
   curl -X GET http://localhost:8081/api/messages \
     -H "Authorization: Bearer $TOKEN" | jq
   ```
docker logs go-api
docker logs localstack-api-cognito-local-1


## Development

### Adding New Features

1. Modify or add API handlers in the `api/handlers` directory
2. Update corresponding models in the `api/models` directory
3. Add new Lambda functions in the `lambda` directory
4. Update AWS resources in the `init-scripts/create-resources.sh` file

### Common Tasks

- View logs: `make logs`
- Stop all services: `make stop`
- Clean build artifacts: `make clean`
- Run tests: `make test`

## Troubleshooting

### Common Issues

1. LocalStack not accessible
   - Check if LocalStack is running: `docker ps | grep localstack`
   - Verify port 4566 is accessible: `curl http://localhost:4566`

2. Database connection issues
   - Check PostgreSQL container: `docker ps | grep postgres`
   - Inspect logs: `docker-compose logs postgres`

3. Lambda execution errors
   - Check Lambda logs in LocalStack: `docker-compose logs localstack | grep -i lambda`
   - Verify Lambda functions are properly deployed: `aws --endpoint-url=http://localhost:4566 lambda list-functions`

## License

This project is licensed under the MIT License - see the LICENSE file for details.
