WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
MONGO_URI ?= mongodb://localhost:27017

.PHONY: dev build test test-go test-frontend test-integration lint mongo-up mongo-down docker-build docker-test bindings

dev: ## Run the app with hot reload
	$(WAILS) dev

build: ## Build the desktop app for this OS into build/bin
	$(WAILS) build -clean

bindings: ## Regenerate frontend/wailsjs after changing bound Go methods
	$(WAILS) generate module

test: test-go test-frontend ## Unit tests (Go + frontend)

test-go:
	go vet ./...
	go test ./...

test-frontend:
	cd frontend && npm test && npm run typecheck

test-integration: ## Requires MongoDB at MONGO_URI (see `make mongo-up`)
	MONGO_URI=$(MONGO_URI) go test -tags integration -count=1 -v ./internal/integration/...

mongo-up: ## Start MongoDB with sample data on localhost:27017
	docker compose up -d mongo

mongo-down:
	docker compose down

docker-build: ## Build + test in Docker and export the Linux binary to ./dist
	docker build --target artifact --output type=local,dest=dist .

docker-test: ## Run integration tests in Docker against the compose MongoDB
	docker compose --profile test up --build --exit-code-from integration-tests integration-tests
