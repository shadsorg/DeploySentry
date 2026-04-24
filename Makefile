.PHONY: dev-up dev-down migrate-up migrate-down run-api run-web test test-unit test-int build docker-build dev-deploy lint clean help dev-setup dev-cli e2e-sdk-up e2e-sdk-down e2e-sdk e2e-sdk-debug run-mobile build-mobile

# Build metadata
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(DATE)"

# Database migration settings
MIGRATE_DSN ?= postgres://deploysentry:deploysentry@localhost:5432/deploysentry?sslmode=disable&search_path=deploy
MIGRATE_DIR ?= migrations

# E2E SDK test stack settings (see docker-compose.e2e.yml)
E2E_COMPOSE ?= docker compose -f docker-compose.e2e.yml
E2E_MIGRATE_DSN ?= postgres://deploysentry:deploysentry@localhost:15432/deploysentry?sslmode=disable&search_path=deploy

# Default target
.DEFAULT_GOAL := help

## dev-setup: Set up complete development environment (new developers)
dev-setup:
	./scripts/dev-setup.sh

## dev-cli: Access development CLI (use 'make dev-cli help' for commands)
dev-cli:
	./scripts/dev.sh $(filter-out $@,$(MAKECMDGOALS))

## help: Show this help message
help:
	@echo "DeploySentry - Deploy release and feature flag management"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Quick start for new developers:"
	@echo "  make dev-setup    # Complete environment setup"
	@echo "  make dev-cli start    # Start services"
	@echo "  make dev-cli api      # Start API (separate terminal)"
	@echo "  make dev-cli web      # Start web UI (separate terminal)"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sort

# Catch-all rule for dev-cli arguments
%:
	@:

## dev-up: Start development infrastructure (PostgreSQL, Redis, NATS)
dev-up:
	docker-compose -f deploy/docker-compose.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 3
	@echo "Development infrastructure is running."

## dev-down: Stop development infrastructure
dev-down:
	docker-compose -f deploy/docker-compose.yml down

## migrate-up: Run database migrations up
migrate-up:
	migrate -database "$(MIGRATE_DSN)" -path $(MIGRATE_DIR) up

## migrate-down: Run database migrations down
migrate-down:
	migrate -database "$(MIGRATE_DSN)" -path $(MIGRATE_DIR) down 1

## run-api: Run the API server locally (dev mode — uses built-in encryption key)
run-api:
	DS_ENVIRONMENT=development go run $(LDFLAGS) ./cmd/api/main.go

## run-web: Run the web frontend dev server
run-web:
	cd web && npm run dev

## run-mobile: Run the mobile PWA dev server (port 3002)
run-mobile:
	cd mobile-pwa && npm install && npm run dev

## build-mobile: Build the mobile PWA for production into mobile-pwa/dist
build-mobile:
	cd mobile-pwa && npm install && npm run build

## test: Run all tests
test:
	go test ./... -race -cover -count=1

## test-unit: Run unit tests only (short mode)
test-unit:
	go test -short ./... -race -count=1

## test-int: Run integration tests only
test-int:
	go test -run Integration ./... -race -count=1

## build: Build Go binaries for all platforms
build: build-linux build-darwin-amd64 build-darwin-arm64
	@echo "Build complete. Binaries in bin/"

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/deploysentry-linux-amd64 ./cmd/api
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/deploysentry-cli-linux-amd64 ./cmd/cli

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/deploysentry-darwin-amd64 ./cmd/api
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/deploysentry-cli-darwin-amd64 ./cmd/cli

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/deploysentry-darwin-arm64 ./cmd/api
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/deploysentry-cli-darwin-arm64 ./cmd/cli

## docker-build: Build Docker images for API and web
docker-build:
	docker build -f deploy/docker/Dockerfile.api -t deploysentry/api:$(VERSION) .
	docker build -f deploy/docker/Dockerfile.web -t deploysentry/web:$(VERSION) ./web

## dev-deploy: Start multi-instance deploy setup (Envoy + agent + blue/green)
dev-deploy:
	docker compose -f deploy/docker-compose.yml -f deploy/docker/docker-compose.deploy.yml up -d --build

## lint: Run linters
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/node_modules/

# ============================================================================
# E2E SDK test stack (hermetic, see docker-compose.e2e.yml)
# ============================================================================

## e2e-sdk-up: Bring up the hermetic e2e SDK stack and run migrations
e2e-sdk-up:
	$(E2E_COMPOSE) up -d --wait
	$(E2E_COMPOSE) exec -T postgres psql -U deploysentry -d deploysentry -c "CREATE SCHEMA IF NOT EXISTS deploy;"
	migrate -database "$(E2E_MIGRATE_DSN)" -path $(MIGRATE_DIR) up

## e2e-sdk-down: Tear down the e2e SDK stack and remove volumes
e2e-sdk-down:
	$(E2E_COMPOSE) down -v

## e2e-sdk: Run Playwright SDK e2e tests against the hermetic stack
e2e-sdk: e2e-sdk-up
	cd web/e2e/sdk-probes/react-harness && npm install && npm run build
	cd web && npx playwright test --project=sdk
	$(MAKE) e2e-sdk-down

## e2e-sdk-debug: Run Playwright SDK e2e tests in headed debug mode
e2e-sdk-debug: e2e-sdk-up
	cd web && npx playwright test --project=sdk --headed --debug
