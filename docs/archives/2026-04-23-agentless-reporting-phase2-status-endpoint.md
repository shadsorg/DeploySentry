# Agentless Deploy Reporting — Phase 2: `/status` endpoint + `app_status` tables

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope (Phase 2 only)

Ship the app-direct status-push path:

1. `app_status` + `app_status_history` tables (migration 055).
2. `POST /api/v1/applications/:app_id/status` — env-scoped API key (or session with new `status:write` permission) reports version + health.
3. Auto-create a `mode=record` deployment with `source="app-push"` when the reported version is new to this (app, env). Rides on Phase 1's `mode=record` plumbing.
4. New RBAC `PermStatusWrite` + API-key scope `status:write`.
5. Extend `Deploy_Integration_Guide.md` with the `/status` section.

Still out of scope: `GET /current-state` (Phase 3), SDK reporter (Phases 4–5), provider webhook adapters (Phases 6–7), version-flap guard (deferred open question).

## Tasks

### T1 — Migration 055

Create `migrations/055_create_app_status.up.sql`:

```sql
CREATE TABLE app_status (
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    commit_sha        TEXT,
    health_state      TEXT        NOT NULL,
    health_score      NUMERIC(4,3),
    health_reason     TEXT,
    deploy_slot       TEXT,
    tags              JSONB       NOT NULL DEFAULT '{}'::jsonb,
    source            TEXT        NOT NULL,
    reported_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (application_id, environment_id),
    CONSTRAINT app_status_health_check CHECK (health_state IN ('healthy','degraded','unhealthy','unknown'))
);

CREATE TABLE app_status_history (
    id                BIGSERIAL   PRIMARY KEY,
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    health_state      TEXT        NOT NULL,
    health_score      NUMERIC(4,3),
    reported_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX app_status_history_app_env_reported_idx
    ON app_status_history (application_id, environment_id, reported_at DESC);
```

Down migration drops both tables.

### T2 — Model

`internal/models/app_status.go`:

- `HealthState` string type with constants `HealthStateHealthy | Degraded | Unhealthy | Unknown`.
- `AppStatus` struct with all columns + `Tags map[string]string`.
- `AppStatusSample` trimmed history row.
- `ReportStatusPayload` — request body shape (version, commit_sha, health, health_score, reason, deploy_slot, tags).
- `Validate()` on the payload: version required, health required + in the enum.

### T3 — RBAC permission + API-key scope

- In `internal/auth/rbac.go`, add `PermStatusWrite Permission = "status:write"` to the const block.
- Add it to `RoleOwner`, `RoleAdmin`, `RoleDeveloper`, `RoleOrgOwner`, `RoleOrgAdmin`, `RoleProjectAdmin`, `RoleProjectEditor`, `RoleEnvDeployer` default permissions.
- API-key scope name `status:write` matches — no new code required for scoped keys; `RequireScope` already handles arbitrary scope strings.

### T4 — Repository

`internal/platform/database/postgres/app_status.go`:

- `UpsertAppStatus(ctx, *models.AppStatus) error` — ON CONFLICT (application_id, environment_id) DO UPDATE.
- `AppendAppStatusHistory(ctx, *models.AppStatusSample) error`.
- `GetAppStatus(ctx, appID, envID) (*models.AppStatus, error)`.
- `HasDeploymentForVersion(ctx, appID, envID, version string) (bool, error)` — `SELECT EXISTS(…)` against `deployments`.

Also expose these as interface methods on a new `internal/appstatus/repository.go` `Repository` interface so the service layer doesn't depend on postgres directly.

### T5 — Service

`internal/appstatus/service.go`:

- `Service.Report(ctx, appID, envID, payload, source, createdBy) (*models.AppStatus, error)`:
  1. Validate payload; default `source="app-push"` if empty.
  2. `reported_at = now()`.
  3. Upsert `app_status` row.
  4. Append to `app_status_history`.
  5. If `!HasDeploymentForVersion(appID, envID, version)`, call out to the deploy service to create a `mode=record` deployment with `source="app-push"`. Artifact defaults to `version` when the payload didn't supply one.
  6. Return the upserted row.
- Service depends on the `Repository` interface and an injected `DeployCreator` interface (a narrow contract: `CreateDeployment(ctx, *models.Deployment) error` — satisfied by `deploy.DeployService`).

Publish `deployment.recorded` on auto-create — already handled inside `deploy.DeployService.CreateDeployment` from Phase 1. No new webhook event for `/status` itself (would be noisy; every heartbeat would fire).

### T6 — Handler

`internal/appstatus/handler.go`:

- `POST /api/v1/applications/:app_id/status`.
- Auth: `RequireAuth` + `mw(rbac, PermStatusWrite)`. For API-key auth:
  - Verify `api_key_app_id` matches `:app_id`; else 403.
  - Resolve environment: exactly one entry in `api_key_environment_ids`; else 400 ("status:write key must be scoped to a single environment"). If zero, the key is org-wide and can't report — 400.
- For session auth, require `environment_id` in the payload (fall-through contract). Not heavily used; document it.
- Bind + validate payload, call `service.Report(...)`, return the upserted row.

### T7 — Wiring

- `cmd/api/main.go`: instantiate `appstatus.NewRepository(pool)`, then `appstatus.NewService(repo, deployService)`, then mount `appstatus.NewHandler(service).RegisterRoutes(api, rbac)`.

### T8 — Tests

- `internal/appstatus/service_test.go`:
  - `TestReport_UpsertsStatus` — happy path.
  - `TestReport_AutoCreatesDeployOnNewVersion` — verifies `deploy.CreateDeployment` is called with `Mode=record, Source=app-push` exactly once.
  - `TestReport_NoDeployWhenVersionExists` — if `HasDeploymentForVersion` returns true, no deploy is created.
  - `TestReport_ValidatesHealthState`.
- `internal/appstatus/handler_test.go`:
  - `TestReport_HappyPath`.
  - `TestReport_MissingVersion` → 400.
  - `TestReport_AppIDMismatch` → 403 (API key scoped to different app).
  - `TestReport_MultipleEnvsOnKey` → 400.

### T9 — Docs

Append a "Reporting live app status" subsection to the PaaS section of `docs/Deploy_Integration_Guide.md` with a curl example:

```bash
curl -X POST https://api.deploysentry.com/api/v1/applications/$APP_ID/status \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.4.2","commit_sha":"abc123","health":"healthy","health_score":0.99}'
```

Document the auto-deploy-record behavior and the env-scoping constraint.

Flip `docs/Current_Initiatives.md` row note to "Phase 1–2 implemented; Phase 3+ pending".

## Verification

```bash
make migrate-up
go build ./...
go test ./internal/appstatus/... ./internal/deploy/... ./internal/models/... \
        ./internal/platform/database/postgres/... ./internal/auth/...
```

## Rollback

- Down migration drops both tables; handler returns 500 on lookup → revert the code change.
- All behavior is opt-in (requires new scope + permission). Existing flows untouched.
