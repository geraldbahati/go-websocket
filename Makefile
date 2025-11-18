.PHONY: help build run test clean docker-up docker-down docker-logs dev-up dev-down fmt vet

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary
	@echo "Building..."
	@go build -o bin/websocket-server cmd/server/main.go
	@echo "Build complete: bin/websocket-server"

run: ## Run the server locally
	@echo "Starting server..."
	@go run cmd/server/main.go

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

lint: fmt vet ## Run all linters

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker-compose build

docker-up: ## Start all services with Docker Compose
	@echo "Starting services..."
	@docker-compose up -d
	@echo "Services started. Use 'make docker-logs' to view logs"

docker-down: ## Stop all services
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs: ## View logs from Docker services
	@docker-compose logs -f websocket-server

docker-restart: ## Restart Docker services
	@echo "Restarting services..."
	@docker-compose restart
	@echo "Services restarted"

docker-rebuild: ## Rebuild and restart services
	@echo "Rebuilding and restarting..."
	@docker-compose up -d --build
	@echo "Services rebuilt and restarted"

dev-up: ## Start only Redis for local development
	@echo "Starting Redis for development..."
	@docker-compose -f docker-compose.dev.yml up -d
	@echo "Redis started on localhost:6379"
	@echo "Redis Commander available at http://localhost:8081"

dev-down: ## Stop development services
	@echo "Stopping development services..."
	@docker-compose -f docker-compose.dev.yml down
	@echo "Development services stopped"

redis-cli: ## Connect to Redis CLI
	@docker exec -it go-websocket-redis redis-cli

setup: ## Setup project (copy .env.example to .env)
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ".env file created. Please update KINDE_ISSUER_URL"; \
	else \
		echo ".env file already exists"; \
	fi

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies downloaded"

.DEFAULT_GOAL := help
