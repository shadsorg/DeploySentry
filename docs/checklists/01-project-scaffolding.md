# 01 — Project Scaffolding & Dev Environment

## Go Project Setup
- [x] Initialize Go module (`go.mod` with `github.com/your-org/deploysentry`)
- [x] Create `cmd/api/main.go` — API server entrypoint
- [x] Create `cmd/cli/main.go` — CLI entrypoint
- [x] Set up `internal/` package structure:
  - [x] `internal/auth/`
  - [x] `internal/deploy/`
  - [x] `internal/flags/`
  - [x] `internal/releases/`
  - [x] `internal/health/`
  - [x] `internal/rollback/`
  - [x] `internal/notifications/`
  - [x] `internal/platform/database/`
  - [x] `internal/platform/cache/`
  - [x] `internal/platform/messaging/`
  - [x] `internal/platform/config/`
  - [x] `internal/platform/middleware/`
  - [x] `internal/models/`
- [x] Create `sdk/` directory structure (go, node, python, java)
- [x] Create `web/` directory structure (React + TypeScript)
- [x] Create `mobile/` directory structure (Flutter app)
- [x] Create `deploy/kubernetes/` directory structure (base, overlays/dev, overlays/staging, overlays/prod)
- [x] Create `deploy/docker/` directory structure
- [x] Create `migrations/` directory

## Build System (Makefile)
- [x] `make dev-up` — Start Docker Compose dependencies
- [x] `make dev-down` — Stop Docker Compose dependencies
- [x] `make migrate-up` — Run database migrations
- [x] `make migrate-down` — Rollback database migrations
- [x] `make run-api` — Start API server with hot reload
- [x] `make run-web` — Start web UI with hot reload
- [x] `make test` — Run all tests
- [x] `make test-unit` — Run unit tests only
- [x] `make test-int` — Run integration tests only
- [x] `make build` — Build all binaries (linux/amd64, darwin/amd64, darwin/arm64)
- [x] `make docker-build` — Build Docker images
- [x] `make lint` — Run linters (golangci-lint, eslint)

## Docker Compose (Local Dev)
- [x] PostgreSQL 16 service with volume persistence
- [x] Redis 7 service
- [x] NATS JetStream service
- [x] Network configuration for service communication
- [x] Health checks for all services
- [x] Environment variable configuration

## CI/CD Pipeline
- [x] `.github/workflows/ci.yml`:
  - [x] Lint stage: golangci-lint, eslint + prettier (UI), sqlfluff (migrations)
  - [x] Test stage: unit tests (parallel by package), integration tests (testcontainers)
  - [x] Build stage: Go binaries, Docker images, UI static bundle
  - [x] Deploy-dev stage: auto-deploy to dev on main merge, smoke tests
- [x] `.github/workflows/release.yml`:
  - [x] Semantic versioning
  - [x] Build and push Docker images
  - [x] Create GitHub release

## Configuration
- [x] Create `.deploysentry.yml` example project config
- [x] Create `.gitignore` (Go binaries, node_modules, .env, vendor, etc.)
- [x] Create `.env.example` with required environment variables

## Docker Images
- [x] `deploy/docker/Dockerfile.api` — Multi-stage build for Go API
- [x] `deploy/docker/Dockerfile.web` — Multi-stage build for React UI
