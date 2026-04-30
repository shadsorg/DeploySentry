# Org Status & Deploy History — Phase 1: Backend endpoints

**Status**: Implementation
**Spec**: [2026-04-23-org-status-and-deploy-history-design.md](../specs/2026-04-23-org-status-and-deploy-history-design.md)
**Date**: 2026-04-23

## Scope

Backend-only. Ship the primitives the UI phases consume.

1. Migration 057: `applications.monitoring_links JSONB NOT NULL DEFAULT '[]'`.
2. Model + validation for `MonitoringLink` (max 10, label ≤60, icon allow-list, http(s) URL).
3. `PUT /api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/monitoring-links` — replace-only CRUD.
4. `GET /api/v1/orgs/:orgSlug/status` — project → app → per-env current-deploy + health fan-in.
5. `GET /api/v1/orgs/:orgSlug/deployments?…` — org-scoped chronological list, cursor-paginated.
6. Scan/select/insert/update for `monitoring_links` on the applications table.

Out of scope for this phase: ETag short-circuit on `/status`, UI pages, bulk-edit.

## Tasks

### T1 — Migration 057

- `migrations/057_add_application_monitoring_links.up.sql`:
  ```sql
  ALTER TABLE applications
      ADD COLUMN monitoring_links JSONB NOT NULL DEFAULT '[]'::jsonb;
  ```
- Matching `.down.sql` drops the column.

### T2 — Model

- Add `MonitoringLink` struct (`label`, `url`, `icon`) to `internal/models/application.go`.
- `Application` gains `MonitoringLinks []MonitoringLink`.
- `ValidateMonitoringLinks([]MonitoringLink) error` — enforces count/length/URL scheme/icon allow-list.

### T3 — Postgres repo

- Update `deploymentSelectCols`-equivalent for applications to include `monitoring_links`.
- Update `CreateApp`, `UpdateApp`, `GetAppByID`, `GetAppBySlug`, `ListAppsByProject` to read/write the field.
- New `UpdateAppMonitoringLinks(ctx, appID, links)` — focused write.
- New `ListLatestDeploymentsByOrg(ctx, orgID, visibility)` — helper for Phase 1 `/status`. Uses a single query joining `projects × applications × environments` LEFT JOIN LATERAL (latest deploy per `(app, env)`) + LEFT JOIN `app_status`.
- New `ListDeploymentsByOrg(ctx, orgID, filters, cursor, limit)` — deploy history, org-scoped, cursor on `(created_at DESC, id DESC)`.

### T4 — Service layer

- Add to entity service: `UpdateAppMonitoringLinks(ctx, appID, links, actor)` with validation.
- New package `internal/orgstate` with:
  - `StatusService.Resolve(ctx, orgID, visibility) (*OrgStatusResponse, error)`.
  - `DeploymentsService.List(ctx, orgID, filters, cursor, limit) (*DeploymentsPage, error)`.
  - Response types live alongside in `internal/models/org_status.go` + `internal/models/org_deployments.go`.

### T5 — HTTP handler

- Extend `internal/entities/handler.go` with the monitoring-links PUT under the existing apps route group. RBAC: `auth.PermProjectManage` (same as app edit).
- New `internal/orgstate/handler.go`:
  - `GET /api/v1/orgs/:orgSlug/status` — RBAC `auth.PermDeployRead`.
  - `GET /api/v1/orgs/:orgSlug/deployments` — RBAC `auth.PermDeployRead`.
  - Visibility: caller's `user_id` + org role resolved via existing `ResolveOrgRole` middleware.

### T6 — Wiring

- `cmd/api/main.go`: instantiate `orgstate.NewStatusService(...)` and `orgstate.NewDeploymentsService(...)`, mount routes.

### T7 — Tests

- `internal/orgstate/status_service_test.go` — fake repo returning canned rows; asserts response structure, faded-chip flag for never-deployed envs, staleness computation reuse from `currentstate`.
- `internal/orgstate/deployments_service_test.go` — cursor encode/decode, filter application, limit clamping.
- `internal/entities/handler_test.go` — monitoring-links PUT happy-path + validation errors.

### T8 — Docs

- Extend `docs/Deploy_Integration_Guide.md` with a short "Org-wide views" section linking to the Current_Initiatives row.
- No README updates until Phase 2 ships the UI.

## Verification

```bash
make migrate-up
go build ./...
go test ./...
```

## Rollback

Migration down drops the column; model field becomes a no-op; new endpoints return 500 on a fresh binary against an un-migrated DB. Revert the migration and redeploy.
