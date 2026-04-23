# Agentless Deploy Reporting — Phase 3: `GET /current-state`

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope (Phase 3 only)

One new read-side endpoint that assembles the "per-environment dashboard card" in a single call.

`GET /api/v1/applications/:app_id/environments/:env_id/current-state` returns:

- Current deployment (version, status, timestamps, source, traffic %).
- Health block (state, score, reason, source, last-reported-at, staleness).
- Recent deployments (last N, default 10).

Phase 3 defers the `active_rollout` field (returns `null`); it can be layered in later without schema change. Staleness rules are configurable but default to fresh <60s, stale <5m, missing otherwise.

## Tasks

### T1 — Staleness + response models

`internal/models/current_state.go`:

- `HealthStaleness` enum (`fresh | stale | missing`).
- `CurrentStateResponse` with the sub-structs (`CurrentDeployment`, `HealthBlock`, `RecentDeployment`, `EnvironmentSummary`). JSON tags match the spec example.

### T2 — Package `internal/currentstate`

- `service.go` — `Service.Resolve(ctx, appID, envID) (*models.CurrentStateResponse, error)`.
  - Depends on two narrow interfaces defined in the same file:
    - `DeployLookup` — `GetLatestDeployment(ctx, appID, envID) (*models.Deployment, error)` + `ListDeployments(ctx, appID, opts)` (both already exist on `deploy.DeployService`).
    - `StatusLookup` — `GetStatus(ctx, appID, envID) (*models.AppStatus, error)` (already on `appstatus.Repository`).
  - Additionally, an `EnvironmentLookup` interface (`GetEnvironment(ctx, id) (*models.Environment, error)`) resolved by `entities.EnvironmentRepository` to populate the slug — handler validates the env exists before touching the other two.
  - Staleness calculation: `now - reported_at < 60s → fresh`, `< 5m → stale`, else `missing`. If no status row exists → `missing` + `source="unknown"`.
- `handler.go` — `GET /api/v1/applications/:app_id/environments/:env_id/current-state`. RBAC `PermDeployRead`. Parses `?limit=` (default 10, cap at 50) for recent-deployment list size.

### T3 — Wiring

`cmd/api/main.go`: construct `currentstate.NewService(deployService, appStatusRepo, envRepo)` and mount the handler.

### T4 — Tests

`internal/currentstate/service_test.go`:
- `TestResolve_WithLatestDeployAndStatus` — happy path; assert every field including staleness=fresh.
- `TestResolve_NoStatus` — returns a well-formed response with `health.source="unknown"`, `staleness="missing"`.
- `TestResolve_StaleStatus` — push back `reported_at` by 3m; expect `staleness="stale"`.
- `TestResolve_MissingStatus` — push back by 10m; expect `staleness="missing"`.
- `TestResolve_NoDeployments` — returns `current_deployment=nil`, `recent_deployments=[]`.
- `TestResolve_EnvironmentNotFound` — returns a typed `ErrEnvNotFound`.

Handler tests can be thin (one happy-path + one bad UUID) — the service covers the logic.

### T5 — Docs

Append a "Viewing current state" subsection to `Deploy_Integration_Guide.md` with a `curl` example and the response schema. Flip the Phase 3 row note in `Current_Initiatives.md`.

## Verification

```bash
go build ./...
go test ./internal/currentstate/... ./internal/appstatus/... ./internal/deploy/... ./internal/models/...
```
