# 05 — Feature Flag Service

## Core Service (`internal/flags/service.go`)
- [ ] Feature flag CRUD operations
- [ ] Flag toggle (enable/disable)
- [ ] Flag archival
- [ ] Targeting rule management (add, update, delete, reorder)
- [ ] Flag change event emission (NATS JetStream)
- [ ] Bulk flag operations

## Evaluation Engine (`internal/flags/evaluator.go`)
- [ ] Flag evaluation for given context (user ID, attributes)
- [ ] Evaluation flow:
  - [ ] Check local cache first (Redis-backed, sub-millisecond)
  - [ ] Cache miss → fetch from PostgreSQL, populate cache
  - [ ] Evaluate targeting rules in priority order
  - [ ] Return resolved value + metadata (reason, rule matched)
- [ ] Deterministic hashing for percentage rollouts
- [ ] Default value fallback when flag disabled or no rules match
- [ ] Evaluation telemetry logging (opt-in, sampled)
- [ ] Batch evaluation (multiple flags in single request)

## Targeting Rules Engine (`internal/flags/targeting.go`)
- [ ] **Percentage rollout**: Hash user ID for deterministic bucketing
- [ ] **User targeting**: Explicit include/exclude lists
- [ ] **Attribute matching**: Rules based on user/context attributes (country, plan, version)
  - [ ] Operators: equals, not_equals, contains, starts_with, ends_with, in, not_in, gt, lt, gte, lte
- [ ] **Segment targeting**: Reusable audience segments
- [ ] **Schedule**: Time-based activation/deactivation
- [ ] Rule priority ordering
- [ ] Rule combination logic (AND/OR conditions)

## Cache Layer
- [ ] Redis-based flag cache
- [ ] Cache invalidation on flag update (pub/sub)
- [ ] Configurable TTL per flag
- [ ] Cache warm-up on service start
- [ ] Cache hit/miss metrics

## Repository Layer (`internal/flags/repository.go`)
- [ ] Feature flag CRUD (PostgreSQL)
- [ ] Targeting rule CRUD
- [ ] Flag evaluation log writing (batched, sampled)
- [ ] Query flags by project, tags, status
- [ ] Stale flag detection (flags not evaluated recently)

## HTTP Handler (`internal/flags/handler.go`)
- [ ] `POST /api/v1/flags` — Create a new flag
- [ ] `GET /api/v1/flags` — List flags (with filtering/search)
- [ ] `GET /api/v1/flags/:key` — Get flag details
- [ ] `PUT /api/v1/flags/:key` — Update flag
- [ ] `DELETE /api/v1/flags/:key` — Archive flag
- [ ] `POST /api/v1/flags/:key/toggle` — Toggle flag on/off
- [ ] `POST /api/v1/flags/evaluate` — Evaluate flag(s) for context
- [ ] `POST /api/v1/flags/:key/rules` — Add targeting rule
- [ ] `PUT /api/v1/flags/:key/rules/:ruleId` — Update targeting rule
- [ ] `DELETE /api/v1/flags/:key/rules/:ruleId` — Delete targeting rule
- [ ] SSE endpoint for streaming flag updates to SDKs
- [ ] Request validation
- [ ] RBAC enforcement per endpoint

## Domain Models (`internal/models/flag.go`)
- [ ] `FeatureFlag` struct with type enum (boolean, string, number, json)
- [ ] `TargetingRule` struct with rule type enum
- [ ] `FlagEvaluationResult` struct (value, reason, rule matched)
- [ ] `EvaluationContext` struct (user ID, attributes)
- [ ] Validation methods
