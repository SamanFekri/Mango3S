WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
MONGO_URI ?= mongodb://localhost:27017
PG_DSN ?= postgres://mango_reader:reader@localhost:5433/shop?sslmode=disable
MYSQL_DSN ?= mango_reader:reader@tcp(localhost:3307)/shop

# Version embedded in the binary (internal/api.Version) and the web admin
# platform this build checks in with for updates/notifications
# (internal/api.PlatformBaseURL) — see web/docs/API_CONTRACT.md.
VERSION ?= $(shell node -pe "require('./wails.json').info.productVersion" 2>/dev/null || echo 0.9.3-dev)
PLATFORM_BASE_URL ?= https://mango3s.com
LDFLAGS := -X mango3s/internal/api.Version=$(VERSION) -X mango3s/internal/api.PlatformBaseURL=$(PLATFORM_BASE_URL)

# Point `make dev` / `make platform-check` at another platform without
# rebuilding, e.g. `make dev PLATFORM_URL=http://localhost:4330` for the local
# web API (`pnpm run dev:api` in ../web). Sets MANGO3S_PLATFORM_URL.
PLATFORM_URL ?=
PLATFORM_ENV := $(if $(PLATFORM_URL),MANGO3S_PLATFORM_URL=$(PLATFORM_URL))

.PHONY: notices dev platform-check platform-api build test test-go test-frontend test-integration test-integration-sql lint mongo-up mongo-down sql-up sql-down rs-up sharded-up search-up docker-build docker-test bindings mcp dmg-background

dev: ## Run the app with hot reload in test mode: request log + API console (Settings › Diagnostics)
	$(PLATFORM_ENV) MANGO3S_TEST_MODE=1 $(WAILS) dev -ldflags "$(LDFLAGS)"

platform-api: ## Send one web API request, e.g. `make platform-api ARGS="call notifications.list"` (ARGS=list for all)
	$(PLATFORM_ENV) go run -ldflags "$(LDFLAGS)" . platform-api $(ARGS)

platform-check: ## Test the web platform APIs: all, or one with CHECK=notifications (ARGS="--list", "--register -v")
	$(PLATFORM_ENV) go run -ldflags "$(LDFLAGS)" . platform-check $(if $(CHECK),--only $(CHECK)) $(ARGS)

build: ## Build the desktop app for this OS into build/bin
	$(WAILS) build -clean -ldflags "$(LDFLAGS)"

dmg-background: ## Regenerate the macOS .dmg window background (needs rsvg-convert: brew install librsvg)
	rsvg-convert -w 660 -h 400 build/darwin/dmg-background.svg -o build/darwin/.bg-1x.png
	rsvg-convert -w 1320 -h 800 build/darwin/dmg-background.svg -o build/darwin/.bg-2x.png
	tiffutil -cathidpicheck build/darwin/.bg-1x.png build/darwin/.bg-2x.png -out build/darwin/dmg-background.tiff
	rm build/darwin/.bg-1x.png build/darwin/.bg-2x.png

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

rs-up: ## Start a single-node replica set on localhost:27020 (change streams, replication)
	docker compose --profile rs up -d --wait mongo-rs

sharded-up: ## Start a sharded cluster (config server, 2 shards, mongos on localhost:27021)
	docker compose --profile sharded up -d --wait

search-up: ## Start a local Atlas deployment with Atlas Search on localhost:27022 (search indexes)
	docker compose --profile search up -d --wait mongo-atlas

sql-down:
	docker compose --profile sql down

mcp: ## Run the MCP server from source (stdio)
	go run . mcp

docker-build: ## Build + test in Docker and export the Linux binary to ./dist
	docker build --target artifact --output type=local,dest=dist .

docker-test: ## Run integration tests in Docker against the compose MongoDB
	docker compose --profile test up --build --exit-code-from integration-tests integration-tests
