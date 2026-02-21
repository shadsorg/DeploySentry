# 04 — Deploy Service

## Core Service (`internal/deploy/service.go`)
- [x] Create deployment orchestration service
- [x] Deployment lifecycle management (create, start, pause, resume, promote, rollback)
- [x] Deployment state machine implementation
- [ ] Interface with Kubernetes API for actual deployments
- [x] Deployment event emission (NATS JetStream)

## Deployment Strategies (`internal/deploy/strategies/`)

### Canary Deployments (`canary.go`)
- [x] Phase-based traffic shifting
  - [x] Phase 1: Route 1% of traffic → monitor 5 min
  - [x] Phase 2: Route 5% of traffic → monitor 5 min
  - [x] Phase 3: Route 25% of traffic → monitor 10 min
  - [x] Phase 4: Route 50% of traffic → monitor 10 min
  - [x] Phase 5: Route 100% — full promotion
- [x] Configurable per-phase settings:
  - [x] Traffic percentage
  - [x] Duration / hold time
  - [x] Health check criteria (error rate, latency p99, custom metrics)
  - [x] Auto-promote vs. manual gate
- [x] Health check integration between phases
- [x] Phase promotion logic (auto and manual)
- [x] Rollback on phase failure

### Blue/Green Deployments (`bluegreen.go`)
- [x] Maintain two identical environments
- [x] Deploy to inactive environment
- [x] Smoke test execution against inactive environment
- [x] Atomic traffic switch via load balancer
- [x] Keep old environment warm for instant rollback
- [x] Environment swap tracking

### Rolling Updates (`rolling.go`)
- [x] Batch-based instance updates
- [x] Configurable batch size
- [x] Health check per batch before proceeding
- [x] Configurable max-unavailable
- [x] Configurable max-surge
- [x] Batch failure handling

## Repository Layer (`internal/deploy/repository.go`)
- [x] Deploy pipeline CRUD (PostgreSQL)
- [x] Deployment CRUD with status tracking
- [x] Deployment phase tracking
- [x] Query deployments by project, environment, status
- [x] Active deployment lookup per project/environment

## HTTP Handler (`internal/deploy/handler.go`)
- [x] `POST /api/v1/deployments` — Create a new deployment
- [x] `GET /api/v1/deployments/:id` — Get deployment status
- [x] `POST /api/v1/deployments/:id/promote` — Advance to next phase
- [x] `POST /api/v1/deployments/:id/rollback` — Trigger rollback
- [x] `POST /api/v1/deployments/:id/pause` — Pause deployment
- [x] `POST /api/v1/deployments/:id/resume` — Resume paused deployment
- [x] `GET /api/v1/deployments` — List deployments (filtered, paginated)
- [x] `GET /api/v1/projects/:id/deployments/active` — Get active deployments for project
- [x] Request validation
- [x] Response serialization
- [x] RBAC enforcement per endpoint

## Domain Models (`internal/models/deployment.go`)
- [x] `DeployPipeline` struct
- [x] `Deployment` struct with status enum
- [x] `DeploymentPhase` struct
- [x] Validation methods
- [x] Status transition methods
