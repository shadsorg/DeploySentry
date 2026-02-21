.PHONY: dev-up dev-down migrate-up migrate-down run-api run-web test test-unit test-int build docker-build lint clean help

# Build metadata
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(DATE)"

# Database migration settings
MIGRATE_DSN ?= postgres://deploysentry:deploysentry@localhost:5432/deploysentry?sslmode=disable
MIGRATE_DIR ?= migrations

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "DeploySentry - Deploy release and feature flag management"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sort

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

## run-api: Run the API server locally
run-api:
	go run $(LDFLAGS) ./cmd/api/main.go

## run-web: Run the web frontend dev server
run-web:
	cd web && npm run dev

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

## lint: Run linters
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/node_modules/
