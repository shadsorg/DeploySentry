# 06 — Release Tracker Service

## Core Service (`internal/releases/service.go`)
- [ ] Release creation from commit/tag
- [ ] Release lifecycle management:
  - [ ] `building` → `built` → `deploying` → `deployed` → `healthy` / `degraded` / `rolled_back`
- [ ] Environment promotion workflow:
  - [ ] Commit → Build → Artifact → Deploy:Dev → Deploy:Staging → Deploy:Prod
- [ ] Promotion gates (auto-promote vs. manual gate per environment)
- [ ] Release health status aggregation
- [ ] Release event emission (NATS JetStream)

## Release Lifecycle
- [ ] Map commits/PRs to releases
- [ ] Track promotion through environments (dev → staging → prod)
- [ ] Auto-promote from dev after health check passes
- [ ] Manual gate for staging → prod promotion
- [ ] Canary strategy for production deployments
- [ ] Changelog generation from commits/PRs

## Repository Layer (`internal/releases/repository.go`)
- [ ] Release CRUD (PostgreSQL)
- [ ] Release-environment status tracking
- [ ] Query releases by project, version, status
- [ ] Latest release lookup per project/environment
- [ ] Release history with environment timeline

## HTTP Handler (`internal/releases/handler.go`)
- [ ] `POST /api/v1/releases` — Create a new release
- [ ] `GET /api/v1/releases` — List releases (filtered, paginated)
- [ ] `GET /api/v1/releases/:id` — Get release details
- [ ] `GET /api/v1/releases/:id/status` — Get release status across environments
- [ ] `POST /api/v1/releases/:id/promote` — Promote release to next environment
- [ ] Request validation
- [ ] RBAC enforcement per endpoint

## Domain Models (`internal/models/release.go`)
- [ ] `Release` struct with status enum
- [ ] `ReleaseEnvironment` struct
- [ ] Status transition validation
- [ ] Version parsing and comparison
