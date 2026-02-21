# 01 — Project Scaffolding & Dev Environment

## Go Project Setup
- [ ] Initialize Go module (`go.mod` with `github.com/your-org/deploysentry`)
- [ ] Create `cmd/api/main.go` — API server entrypoint
- [ ] Create `cmd/cli/main.go` — CLI entrypoint
- [ ] Set up `internal/` package structure:
  - [ ] `internal/auth/`
  - [ ] `internal/deploy/`
  - [ ] `internal/flags/`
  - [ ] `internal/releases/`
  - [ ] `internal/health/`
  - [ ] `internal/rollback/`
  - [ ] `internal/notifications/`
  - [ ] `internal/platform/database/`
  - [ ] `internal/platform/cache/`
  - [ ] `internal/platform/messaging/`
  - [ ] `internal/platform/config/`
  - [ ] `internal/platform/middleware/`
  - [ ] `internal/models/`
- [ ] Create `sdk/` directory structure (go, node, python, java)
- [ ] Create `web/` directory structure (React + TypeScript)
- [ ] Create `deploy/kubernetes/` directory structure (base, overlays/dev, overlays/staging, overlays/prod)
- [ ] Create `deploy/docker/` directory structure
- [ ] Create `migrations/` directory

## Build System (Makefile)
- [ ] `make dev-up` — Start Docker Compose dependencies
- [ ] `make dev-down` — Stop Docker Compose dependencies
- [ ] `make migrate-up` — Run database migrations
- [ ] `make migrate-down` — Rollback database migrations
- [ ] `make run-api` — Start API server with hot reload
- [ ] `make run-web` — Start web UI with hot reload
- [ ] `make test` — Run all tests
- [ ] `make test-unit` — Run unit tests only
- [ ] `make test-int` — Run integration tests only
- [ ] `make build` — Build all binaries (linux/amd64, darwin/amd64, darwin/arm64)
- [ ] `make docker-build` — Build Docker images
- [ ] `make lint` — Run linters (golangci-lint, eslint)

## Docker Compose (Local Dev)
- [ ] PostgreSQL 16 service with volume persistence
- [ ] Redis 7 service
- [ ] NATS JetStream service
- [ ] Network configuration for service communication
- [ ] Health checks for all services
- [ ] Environment variable configuration

## CI/CD Pipeline
- [ ] `.github/workflows/ci.yml`:
  - [ ] Lint stage: golangci-lint, eslint + prettier (UI), sqlfluff (migrations)
  - [ ] Test stage: unit tests (parallel by package), integration tests (testcontainers)
  - [ ] Build stage: Go binaries, Docker images, UI static bundle
  - [ ] Deploy-dev stage: auto-deploy to dev on main merge, smoke tests
- [ ] `.github/workflows/release.yml`:
  - [ ] Semantic versioning
  - [ ] Build and push Docker images
  - [ ] Create GitHub release

## Configuration
- [ ] Create `.deploysentry.yml` example project config
- [ ] Create `.gitignore` (Go binaries, node_modules, .env, vendor, etc.)
- [ ] Create `.env.example` with required environment variables

## Docker Images
- [ ] `deploy/docker/Dockerfile.api` — Multi-stage build for Go API
- [ ] `deploy/docker/Dockerfile.web` — Multi-stage build for React UI
