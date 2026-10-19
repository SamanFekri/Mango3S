WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
MONGO_URI ?= mongodb://localhost:27017
PG_DSN ?= postgres://mango_reader:reader@localhost:5433/shop?sslmode=disable
MYSQL_DSN ?= mango_reader:reader@tcp(localhost:3307)/shop

# Version embedded in the binary (internal/api.Version) and the web admin
# platform this build checks in with for updates/notifications
# (internal/api.PlatformBaseURL) — see web/docs/API_CONTRACT.md.
VERSION ?= $(shell node -pe "require('./wails.json').info.productVersion" 2>/dev/null || echo 0.1.0-dev)
PLATFORM_BASE_URL ?= https://mango3s.com
LDFLAGS := -X mango3s/internal/api.Version=$(VERSION) -X mango3s/internal/api.PlatformBaseURL=$(PLATFORM_BASE_URL)

.PHONY: notices dev build test test-go test-frontend test-integration test-integration-sql lint mongo-up mongo-down sql-up sql-down docker-build docker-test bindings mcp

dev: ## Run the app with hot reload
	$(WAILS) dev -ldflags "$(LDFLAGS)"

build: ## Build the desktop app for this OS into build/bin
	$(WAILS) build -clean -ldflags "$(LDFLAGS)"

notices: ## Regenerate the third-party license list shown in About › License
	scripts/third-party-notices.sh

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

test-integration-sql: ## Integration tests incl. PostgreSQL/MySQL migrations (see `make sql-up`)
	MONGO_URI=$(MONGO_URI) PG_DSN='$(PG_DSN)' MYSQL_DSN='$(MYSQL_DSN)' go test -tags integration -count=1 -v ./internal/integration/...

mongo-up: ## Start MongoDB with sample data on localhost:27017
	docker compose up -d mongo

mongo-down:
	docker compose down

sql-up: ## Start PostgreSQL (:5433) and MySQL (:3307) with the seeded shop schema
	docker compose --profile sql up -d --wait postgres mysql

sql-down:
	docker compose --profile sql down

mcp: ## Run the MCP server from source (stdio)
	go run . mcp

docker-build: ## Build + test in Docker and export the Linux binary to ./dist
	docker build --target artifact --output type=local,dest=dist .

docker-test: ## Run integration tests in Docker against the compose MongoDB
	docker compose --profile test up --build --exit-code-from integration-tests integration-tests
