.PHONY: build start stop clean test init-localstack

# Build all Go code
build:
	@echo "Building Go applications..."
	cd api && go build -o ../bin/api
	cd lambda/file-processor && GOOS=linux GOARCH=amd64 go build -o ../../bin/file-processor
	cd lambda/message-processor && GOOS=linux GOARCH=amd64 go build -o ../../bin/message-processor

# ZIP lambda functions for deployment
package-lambdas: build
	@echo "Packaging Lambda functions..."
	mkdir -p bin
	cd bin && zip -j file-processor.zip file-processor
	cd bin && zip -j message-processor.zip message-processor
	cp bin/file-processor.zip init-scripts/
	cp bin/message-processor.zip init-scripts/

# Start all services with Docker Compose
start: package-lambdas
	@echo "Starting services with Docker Compose..."
	chmod +x init-scripts/wait-for-it.sh
	chmod +x init-scripts/create-resources.sh
	docker compose up -d

# Stop all services
stop:
	@echo "Stopping services..."
	docker compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f init-scripts/*.zip

# Initialize LocalStack resources manually if needed
init-localstack:
	@echo "Initializing LocalStack resources..."
	docker compose exec localstack bash /docker-entrypoint-initaws.d/create-resources.sh

# Run tests
test:
	@echo "Running tests..."
	cd api && go test ./...
	cd lambda/file-processor && go test ./...
	cd lambda/message-processor && go test ./...

# Check everything is working
health-check:
	@echo "Checking API health..."
	curl -s http://localhost:8081/health | grep -q "ok" && echo "API is healthy" || echo "API is not responding"

# View logs
logs:
	@echo "Viewing logs..."
	docker compose logs -f

# Create a new user in Cognito
create-user:
	@echo "Creating test user..."
	aws --endpoint-url=http://localhost:9229 cognito-idp admin-create-user \
		--user-pool-id local_user_pool \
		--username testuser2 \
		--temporary-password Test@123 \
		--user-attributes Name=email,Value=test2@example.com