# 05 — Feature Flag Service

## Core Service (`internal/flags/service.go`)
- [x] Feature flag CRUD operations
- [x] Flag toggle (enable/disable)
- [x] Flag archival
- [x] Targeting rule management (add, update, delete, reorder)
- [x] Flag change event emission (NATS JetStream)
- [x] Bulk flag operations

## Evaluation Engine (`internal/flags/evaluator.go`)
- [x] Flag evaluation for given context (user ID, attributes)
- [x] Evaluation flow:
  - [x] Check local cache first (Redis-backed, sub-millisecond)
  - [x] Cache miss → fetch from PostgreSQL, populate cache
  - [x] Evaluate targeting rules in priority order
  - [x] Return resolved value + metadata (reason, rule matched)
- [x] Deterministic hashing for percentage rollouts
- [x] Default value fallback when flag disabled or no rules match
- [x] Evaluation telemetry logging (opt-in, sampled)
- [x] Batch evaluation (multiple flags in single request)

## Targeting Rules Engine (`internal/flags/targeting.go`)
- [x] **Percentage rollout**: Hash user ID for deterministic bucketing
- [x] **User targeting**: Explicit include/exclude lists
- [x] **Attribute matching**: Rules based on user/context attributes (country, plan, version)
  - [x] Operators: equals, not_equals, contains, starts_with, ends_with, in, not_in, gt, lt, gte, lte
- [x] **Segment targeting**: Reusable audience segments
- [x] **Schedule**: Time-based activation/deactivation
- [x] Rule priority ordering
- [x] Rule combination logic (AND/OR conditions)

## Cache Layer
- [x] Redis-based flag cache
- [x] Cache invalidation on flag update (pub/sub)
- [x] Configurable TTL per flag
- [x] Cache warm-up on service start
- [x] Cache hit/miss metrics

## Repository Layer (`internal/flags/repository.go`)
- [x] Feature flag CRUD (PostgreSQL)
- [x] Targeting rule CRUD
- [x] Flag evaluation log writing (batched, sampled)
- [x] Query flags by project, tags, status
- [x] Stale flag detection (flags not evaluated recently)

## HTTP Handler (`internal/flags/handler.go`)
- [x] `POST /api/v1/flags` — Create a new flag
- [x] `GET /api/v1/flags` — List flags (with filtering/search)
- [x] `GET /api/v1/flags/:key` — Get flag details
- [x] `PUT /api/v1/flags/:key` — Update flag
- [x] `DELETE /api/v1/flags/:key` — Archive flag
- [x] `POST /api/v1/flags/:key/toggle` — Toggle flag on/off
- [x] `POST /api/v1/flags/evaluate` — Evaluate flag(s) for context
- [x] `POST /api/v1/flags/:key/rules` — Add targeting rule
- [x] `PUT /api/v1/flags/:key/rules/:ruleId` — Update targeting rule
- [x] `DELETE /api/v1/flags/:key/rules/:ruleId` — Delete targeting rule
- [x] SSE endpoint for streaming flag updates to SDKs
- [x] Request validation
- [x] RBAC enforcement per endpoint

## Domain Models (`internal/models/flag.go`)
- [x] `FeatureFlag` struct with type enum (boolean, string, number, json)
- [x] `TargetingRule` struct with rule type enum
- [x] `FlagEvaluationResult` struct (value, reason, rule matched)
- [x] `EvaluationContext` struct (user ID, attributes)
- [x] Validation methods
