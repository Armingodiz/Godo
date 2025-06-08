.PHONY: help check-redis check-s3 start-tools stop-tools generate-mocks test run

help:
	@echo "Available commands:"
	@echo "  run              - Start the entire project (all services)"
	@echo "  check-redis      - Check Redis stream information"
	@echo "  check-s3         - List files in S3 bucket"
	@echo "  start-tools      - Start CLI tools (redis-cli, aws-cli)"
	@echo "  stop-tools       - Stop CLI tools"
	@echo "  generate-mocks   - Generate mocks using Mockery"
	@echo "  test             - Run all tests"
	@echo "  help             - Show this help message"

run:
	@echo "🚀 Starting the entire project..."
	@docker-compose --profile todo up --build

start-tools:
	@echo "Starting CLI tools..."
	@docker-compose --profile tools up -d redis-cli aws-cli

stop-tools:
	@echo "Stopping CLI tools..."
	@docker-compose stop redis-cli aws-cli
	@docker-compose rm -f redis-cli aws-cli

check-redis: start-tools
	@echo "🔍 Checking Redis stream 'todo-events'..."
	@docker-compose exec redis-cli redis-cli -h redis XINFO STREAM todo-events

check-s3: start-tools
	@echo "🗂️  Checking S3 bucket 'todo-bucket'..."
	@docker-compose exec aws-cli aws --endpoint-url=http://localstack:4566 s3 ls s3://todo-bucket --recursive

generate-mocks:
	@echo "🔧 Generating mocks using Mockery..."
	@mockery
	@echo "✅ Mocks generated successfully!"

test:
	@echo "🧪 Running all tests..."
	@go test ./... -v 