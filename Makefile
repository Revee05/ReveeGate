# Makefile for ReveeGate
# Production-ready build and deployment commands

.PHONY: all build run test clean dev docker-build docker-up docker-down migrate lint fmt help

# Variables
APP_NAME := reveegate
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO := go
GOFLAGS := -ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Default target
all: lint test build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/server

# Build for Linux (for Docker/deployment)
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/server

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	./bin/$(APP_NAME)

# Run in development mode with hot reload
dev:
	@echo "Starting development server..."
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# Lint the code
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

# Format the code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

# Generate code (SQLC, etc.)
generate:
	@echo "Generating code..."
	@which sqlc > /dev/null || go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	sqlc generate -f db/sqlc/sqlc.yaml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f app

docker-clean:
	@echo "Cleaning Docker resources..."
	docker-compose down -v --remove-orphans

# Database migrations
migrate-up:
	@echo "Running migrations..."
	docker-compose --profile migrate run --rm migrate

migrate-down:
	@echo "Rolling back last migration..."
	@which migrate > /dev/null || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate -path db/migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@echo "Creating new migration..."
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir db/migrations -seq $$name

# Database commands
db-shell:
	@echo "Opening database shell..."
	docker-compose exec postgres psql -U reveegate -d reveegate

db-reset:
	@echo "Resetting database..."
	docker-compose exec postgres psql -U reveegate -d reveegate -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	$(MAKE) migrate-up

# Redis commands
redis-shell:
	@echo "Opening Redis shell..."
	docker-compose exec redis redis-cli

# Security audit
security:
	@echo "Running security audit..."
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# Install development dependencies
deps:
	@echo "Installing development dependencies..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Download Go module dependencies
mod:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Vendor dependencies (for offline builds)
vendor:
	@echo "Vendoring dependencies..."
	$(GO) mod vendor

# Help
help:
	@echo "ReveeGate Makefile Commands:"
	@echo ""
	@echo "  Build & Run:"
	@echo "    make build          - Build the application"
	@echo "    make build-linux    - Build for Linux (Docker/deployment)"
	@echo "    make run            - Build and run the application"
	@echo "    make dev            - Run with hot reload (requires air)"
	@echo ""
	@echo "  Testing:"
	@echo "    make test           - Run all tests"
	@echo "    make test-coverage  - Run tests with coverage report"
	@echo "    make bench          - Run benchmarks"
	@echo ""
	@echo "  Code Quality:"
	@echo "    make lint           - Run linter"
	@echo "    make fmt            - Format code"
	@echo "    make security       - Run security audit"
	@echo ""
	@echo "  Docker:"
	@echo "    make docker-build   - Build Docker image"
	@echo "    make docker-up      - Start containers"
	@echo "    make docker-down    - Stop containers"
	@echo "    make docker-logs    - View container logs"
	@echo "    make docker-clean   - Remove containers and volumes"
	@echo ""
	@echo "  Database:"
	@echo "    make migrate-up     - Run migrations"
	@echo "    make migrate-down   - Rollback last migration"
	@echo "    make migrate-create - Create new migration"
	@echo "    make db-shell       - Open PostgreSQL shell"
	@echo "    make db-reset       - Reset database"
	@echo "    make redis-shell    - Open Redis shell"
	@echo ""
	@echo "  Other:"
	@echo "    make generate       - Generate code (SQLC)"
	@echo "    make deps           - Install dev dependencies"
	@echo "    make mod            - Download module dependencies"
	@echo "    make clean          - Clean build artifacts"
	@echo "    make help           - Show this help"
