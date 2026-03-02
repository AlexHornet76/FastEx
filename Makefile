.PHONY: help build up down logs test migrate clean

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build all Docker images
	docker-compose build

up: ## Start all services
	docker-compose up -d
	@echo "Services starting... Check health:"
	@echo "  curl http://localhost:8080/health"

down: ## Stop all services
	docker-compose down

logs: ## Tail logs from all services
	docker-compose logs -f

logs-gateway: ## Tail gateway logs only
	docker-compose logs -f gateway

logs-postgres: ## Tail PostgreSQL logs only
	docker-compose logs -f postgres

test: ## Run all tests
	cd gateway && go test ./... -v

test-integration: ## Run integration tests (requires running services)
	cd gateway && go test ./tests -v -tags=integration

migrate: ## Run database migrations manually
	docker-compose exec postgres psql -U exchangeuser -d exchangedb -f /migrations/001_init_schema.up.sql

psql: ## Open PostgreSQL shell
	docker-compose exec postgres psql -U exchangeuser -d exchangedb

clean: ## Stop services and remove volumes
	docker-compose down -v
	rm -rf postgres_data

restart: down up ## Restart all services

status: ## Show service status
	docker-compose ps

keygen: ## Generate test Ed25519 key pair
	go run scripts/keygen.go

test-client: ## Run test client for manual auth flow
	go run scripts/test-client.go