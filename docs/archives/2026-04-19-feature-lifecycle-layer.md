# Feature Lifecycle Layer — Implementation Plan

**Status**: Implementation
**Spec**: [2026-04-19-feature-lifecycle-layer-design.md](../specs/2026-04-19-feature-lifecycle-layer-design.md)
**Date**: 2026-04-19

## Tasks

### T1 — Migration 052: lifecycle columns
- `migrations/052_add_flag_lifecycle.up.sql` adds to `feature_flags`:
  - `smoke_test_status text NULL` with CHECK (`pending|pass|fail`)
  - `user_test_status text NULL` with same CHECK
  - `scheduled_removal_at timestamptz NULL`
  - `iteration_count integer NOT NULL DEFAULT 0`
  - `iteration_exhausted boolean NOT NULL DEFAULT false`
  - `last_smoke_test_notes text NULL`
  - `last_user_test_notes text NULL`
  - `scheduled_removal_fired_at timestamptz NULL` (internal — prevents duplicate due events)
- Partial index on `scheduled_removal_at` where not null and not yet fired.
- Matching `.down.sql` drops the columns + index.

### T2 — Model extension
- Add lifecycle fields to `models.FeatureFlag` (pointer types for nullable text columns).
- Add new event constants to `models/webhook.go`:
  `EventFlagSmokeTestPassed`, `EventFlagSmokeTestFailed`, `EventFlagUserTestPassed`, `EventFlagUserTestFailed`, `EventFlagScheduledForRemovalSet`, `EventFlagScheduledForRemovalCancelled`, `EventFlagScheduledForRemovalDue`, `EventFlagIterationExhausted`.
- Append to `AllWebhookEvents()`.

### T3 — Repository
- Update `scanFeatureFlag` column list + scan to include the new columns.
- Update `CreateFlag` / `UpdateFlag` column lists so round-tripping preserves lifecycle state (they write `0/false/null` by default, which is safe for existing callers).
- Add `GetFlagByProjectKey(projectID, key)` — lifecycle endpoints resolve `:key` against API key project scope without an env.
- Add `UpdateFlagLifecycle(ctx, id, patch)` — targeted update of only the lifecycle columns.
- Add `DisableFlagEverywhere(ctx, id)` — sets `feature_flags.enabled=false` AND `UPDATE flag_environment_state SET enabled=false WHERE flag_id=$1` in one tx.
- Add `ListFlagsDueForRemoval(ctx, now)` + `MarkFlagRemovalFired(ctx, id, ts)` — used by the scheduler.

### T4 — Service
- Add `FlagService.RecordSmokeTestResult`, `RecordUserTestResult`, `ScheduleRemoval`, `CancelScheduledRemoval`, `MarkIterationExhausted`.
- Service owns the state transitions: on `fail`, call `DisableFlagEverywhere` + increment iteration_count + store notes in a single lifecycle update.
- Cache invalidation on every lifecycle change.

### T5 — Handler (ApiKey-authenticated)
- New route group mounted on the existing `/api/v1/flags` group (keeps the same auth chain).
- Handlers resolve `:key` → flag via project_id from context, call the service method, then publish the matching webhook event. Webhook payload schema matches the spec.
- Validate body inputs (`status in {pass,fail}`, `days > 0`, notes required on user-test fail).

### T6 — Scheduler
- `internal/flags/lifecycle_scheduler.go` — simple ticker-driven loop (default 1m, configurable for tests). On each tick: list flags with `scheduled_removal_at <= now() AND scheduled_removal_fired_at IS NULL`, emit `flag.scheduled_for_removal.due`, then mark fired.
- Started in `cmd/api/main.go` inside the existing background-goroutine block.

### T7 — Route wiring
- `handler.RegisterRoutes` in `internal/flags/handler.go` adds the five new endpoints.
- `cmd/api/main.go` starts the scheduler.

### T8 — Tests
- `internal/flags/handler_test.go`: extend `mockFlagService` with lifecycle methods (no-op defaults), add table-driven tests for each endpoint — happy paths + validation errors + status transitions.
- `internal/flags/lifecycle_scheduler_test.go`: verify the scheduler marks fired and emits exactly once.

### T9 — Web dashboard
- Add `FlagLifecycle` fields to `web/src/types.ts` `Flag` interface.
- `web/src/api.ts`: add `lifecycleApi` methods (`recordSmokeTest`, `recordUserTest`, `scheduleRemoval`, `cancelScheduledRemoval`, `markExhausted`).
- `web/src/pages/FlagDetailPage.tsx`: add a **Lifecycle** tab — status pills, iteration count, scheduled-removal countdown, last failure notes. Timeline tab reads filtered audit/webhook entries and renders chronologically.

### T10 — Docs
- `docs/Feature_Lifecycle.md` — public-facing guide covering endpoints, events, payload shape, smoke-test targeting convention (`{ is_smoke_test: true }`), header convention (`X-DS-Test-Context` / `X-DS-Test-Signature`).
- Cross-reference from `docs/Rollout_Strategies.md`.
- Update `docs/Current_Initiatives.md` with the new entry.

## Verification
- `go test ./internal/flags/... ./internal/platform/database/postgres/...`
- `go build ./...`
- `cd web && npm run build` (tsc + vite)

## Rollback
- All new behaviour is gated behind new fields. Rolling back the migration (`down.sql`) removes the columns; service methods become no-ops because callers can't invoke them without the migration.
