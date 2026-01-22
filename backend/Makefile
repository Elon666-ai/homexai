.PHONY: help dev build test clean migrate-up migrate-down seed docker-up docker-down

# Variables
BINARY_NAME=homexai-api.exe
MAIN_PATH=./cmd/api/main.go
MIGRATE_PATH=./cmd/migrate/main.go

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Run development server
	go run $(MAIN_PATH)

build: ## Build the application
	go mod tidy
	go mod vendor
	go build -ldflags="-s -w -X 'main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')'" -o $(BINARY_NAME) $(MAIN_PATH)

test: ## Run tests
	go test -v -cover ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf vendor/

install-deps: ## Install dependencies
	go mod download
	go mod tidy

migrate-up: ## Run database migrations (up)
	@echo "Migrating master database..."
	go run $(MIGRATE_PATH) -type master

migrate-property: ## Run property database migration (usage: make migrate-property PROPERTY_ID=1001)
	@echo "Migrating property database for $(PROPERTY_ID)..."
	go run $(MIGRATE_PATH) -type property -property $(PROPERTY_ID)

migrate-all: ## Run all migrations
	@echo "Running all migrations..."
	go run $(MIGRATE_PATH) -type all

migrate-down: ## Rollback database migrations (down)
	@echo "Note: Auto-rollback not implemented. Use manual SQL scripts."

migrate-create: ## Create a new migration file (usage: make migrate-create name=create_users_table)
	@if [ -z "$(name)" ]; then \
		echo "Error: name is required. Usage: make migrate-create name=create_users_table"; \
		exit 1; \
	fi
	@echo "Creating migration: $(name)"
	@mkdir -p migrations/master
	@touch migrations/master/$$(date +%Y%m%d%H%M%S)_$(name).sql

seed: ## Seed database with test data
	go run ./cmd/seed/main.go

docker-up: ## Start docker containers
	docker-compose up -d

docker-down: ## Stop docker containers
	docker-compose down

docker-logs: ## View docker logs
	docker-compose logs -f

lint: ## Run linter
	golangci-lint run

format: ## Format code
	go fmt ./...
	goimports -w .

check: ## Run all checks (format, lint, test)
	@echo "Running format..."
	@$(MAKE) format
	@echo "Running linter..."
	@$(MAKE) lint
	@echo "Running tests..."
	@$(MAKE) test

run-prod: ## Run in production mode
	$(BINARY_NAME)

.DEFAULT_GOAL := help
