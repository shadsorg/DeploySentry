# 04 — Deploy Service

## Core Service (`internal/deploy/service.go`)
- [ ] Create deployment orchestration service
- [ ] Deployment lifecycle management (create, start, pause, resume, promote, rollback)
- [ ] Deployment state machine implementation
- [ ] Interface with Kubernetes API for actual deployments
- [ ] Deployment event emission (NATS JetStream)

## Deployment Strategies (`internal/deploy/strategies/`)

### Canary Deployments (`canary.go`)
- [ ] Phase-based traffic shifting
  - [ ] Phase 1: Route 1% of traffic → monitor 5 min
  - [ ] Phase 2: Route 5% of traffic → monitor 5 min
  - [ ] Phase 3: Route 25% of traffic → monitor 10 min
  - [ ] Phase 4: Route 50% of traffic → monitor 10 min
  - [ ] Phase 5: Route 100% — full promotion
- [ ] Configurable per-phase settings:
  - [ ] Traffic percentage
  - [ ] Duration / hold time
  - [ ] Health check criteria (error rate, latency p99, custom metrics)
  - [ ] Auto-promote vs. manual gate
- [ ] Health check integration between phases
- [ ] Phase promotion logic (auto and manual)
- [ ] Rollback on phase failure

### Blue/Green Deployments (`bluegreen.go`)
- [ ] Maintain two identical environments
- [ ] Deploy to inactive environment
- [ ] Smoke test execution against inactive environment
- [ ] Atomic traffic switch via load balancer
- [ ] Keep old environment warm for instant rollback
- [ ] Environment swap tracking

### Rolling Updates (`rolling.go`)
- [ ] Batch-based instance updates
- [ ] Configurable batch size
- [ ] Health check per batch before proceeding
- [ ] Configurable max-unavailable
- [ ] Configurable max-surge
- [ ] Batch failure handling

## Repository Layer (`internal/deploy/repository.go`)
- [ ] Deploy pipeline CRUD (PostgreSQL)
- [ ] Deployment CRUD with status tracking
- [ ] Deployment phase tracking
- [ ] Query deployments by project, environment, status
- [ ] Active deployment lookup per project/environment

## HTTP Handler (`internal/deploy/handler.go`)
- [ ] `POST /api/v1/deployments` — Create a new deployment
- [ ] `GET /api/v1/deployments/:id` — Get deployment status
- [ ] `POST /api/v1/deployments/:id/promote` — Advance to next phase
- [ ] `POST /api/v1/deployments/:id/rollback` — Trigger rollback
- [ ] `POST /api/v1/deployments/:id/pause` — Pause deployment
- [ ] `POST /api/v1/deployments/:id/resume` — Resume paused deployment
- [ ] `GET /api/v1/deployments` — List deployments (filtered, paginated)
- [ ] `GET /api/v1/projects/:id/deployments/active` — Get active deployments for project
- [ ] Request validation
- [ ] Response serialization
- [ ] RBAC enforcement per endpoint

## Domain Models (`internal/models/deployment.go`)
- [ ] `DeployPipeline` struct
- [ ] `Deployment` struct with status enum
- [ ] `DeploymentPhase` struct
- [ ] Validation methods
- [ ] Status transition methods
