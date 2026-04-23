# Agentless Deploy Reporting — Phase 6: Deploy-event ingestion + generic endpoint + Railway adapter

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope

Ship the shared deploy-event ingestion plumbing + the generic canonical endpoint + the Railway adapter — the first adapter validates the shared contract. Ingested events create `mode=record` deployments via the Phase 1 plumbing.

1. Migration 056: `deploy_integrations` + `deploy_integration_events` tables.
2. Canonical `DeployEvent` type + `DeployEventAdapter` interface.
3. Integration CRUD — `POST/GET/DELETE /api/v1/integrations/deploys` (org-scoped, needs `settings:write` or `deploys:write`).
4. Generic webhook endpoint — `POST /api/v1/integrations/deploys/webhook` with bearer or HMAC auth.
5. Railway adapter — `POST /api/v1/integrations/railway/webhook` (HMAC `X-Railway-Signature` by default).
6. Deduplication via `deploy_integration_events.dedup_key` (24h unique index).
7. Integration events read endpoint for phase-7 MCP `check_deploy_integration` and for this phase's setup verification.
8. Tests + docs.

MCP setup tools are intentionally deferred — Phase 6 focuses on the API primitives.

## Tasks

### T1 — Migration 056

`migrations/056_create_deploy_integrations.up.sql`:

- `deploy_integrations(id, application_id, provider, auth_mode, webhook_secret_enc, provider_config JSONB, env_mapping JSONB, version_extractors JSONB, enabled, created_at, updated_at)`.
- `deploy_integration_events(id, integration_id, event_type, dedup_key, deployment_id, payload_json, received_at)` — `UNIQUE(dedup_key)`, index on `(integration_id, received_at DESC)`.
- CHECK `provider IN ('generic','railway','render','fly','heroku','vercel','netlify','github-actions')`.
- CHECK `auth_mode IN ('hmac','bearer')`.

Down migration drops both tables.

### T2 — Model

`internal/models/deploy_integration.go`:

- `DeployIntegration` struct + `DeployIntegrationEvent` struct.
- Constants: provider strings, auth-mode strings, event types (`deploy.succeeded`, `deploy.failed`, `deploy.started`).
- `DeployEvent` canonical struct (`EventType`, `Environment`, `Version`, `CommitSHA`, `Artifact`, `OccurredAt`, `URL`, `Metadata`).
- `DedupKey(appID, envID, version, eventType)` → SHA-256 hex string.

### T3 — Adapter interface + core ingest

`internal/integrations/deploys/adapter.go`:

```go
type DeployEventAdapter interface {
    Provider() string
    VerifySignature(r *http.Request, body []byte, secret string) error
    ParsePayload(body []byte, cfg *models.DeployIntegration) (models.DeployEvent, error)
}
```

`internal/integrations/deploys/service.go`:

- `Service.Ingest(ctx, integration, event, rawPayload) (*models.Deployment, error)`:
  1. Resolve `environment_id` from `integration.EnvMapping` using the canonical `event.Environment`. Unknown → emit `integration.unmapped_environment` webhook event, store the payload, return `nil, nil` (fail-closed).
  2. Build dedup key. Upsert into `deploy_integration_events` via `ON CONFLICT (dedup_key) DO UPDATE SET payload_json = …` returning the existing `deployment_id` when the event was seen before.
  3. If previously-seen event has a `deployment_id`, return that deployment (idempotent replay).
  4. New event: act on `event.Type`:
     - `deploy.succeeded` → create `mode=record` deployment with `source="<provider>-webhook"` (or `"generic-webhook"`). Write the new deployment ID back to the event row.
     - `deploy.failed` / `deploy.crashed` → emit `deployment.provider_failed` webhook event. No deployment row.
     - `deploy.started` → ignored in v1 (no row created).
- `Service.CreateIntegration(...)` / `GetIntegration` / `ListIntegrationsForApp` / `DeleteIntegration` / `ListRecentEvents(integrationID, limit)` — thin CRUD + read-side.

### T4 — Repository

`internal/platform/database/postgres/deploy_integration.go` — `Repository` implementing Upsert/Get/List/Delete for integrations and Insert/List for events. Handles encryption of `webhook_secret_enc` using the existing `crypto.Encrypt` + the server's encryption key.

### T5 — Adapters

- `internal/integrations/deploys/generic.go` — parses the canonical `DeployEvent` JSON directly; verifies either HMAC (`X-DeploySentry-Signature`, `sha256=<hex>`) or bearer token (`Authorization: Bearer <token>`) based on `integration.AuthMode`.
- `internal/integrations/deploys/railway.go` — HMAC `X-Railway-Signature` verification; maps Railway's webhook JSON shape (`type: DEPLOY`, `status: SUCCESS|FAILED|CRASHED`, `meta.deploymentId`, `commit.sha`, `environment.name`) into the canonical `DeployEvent`. Tolerates schema drift using ordered `version_extractors` paths.
- Registry map `providers[name] = adapter` in `adapter.go`; dispatch by URL param in the handler.

### T6 — HTTP handlers

`internal/integrations/deploys/handler.go`:

- `POST /api/v1/integrations/deploys/webhook` — generic path. Looks up integration by `X-DeploySentry-Integration-Id` header; dispatches to the `generic` adapter.
- `POST /api/v1/integrations/:provider/webhook` — provider path. Looks up integration by the provider-specific identifier in the payload (Railway: `service.id` from `provider_config.service_id`); dispatches to the matching adapter.
- `POST /api/v1/integrations/deploys` — CRUD (create).
- `GET /api/v1/integrations/deploys?application_id=…` — list.
- `GET /api/v1/integrations/deploys/:id/events?limit=…` — recent events read-side (for verify/check tooling).
- `DELETE /api/v1/integrations/deploys/:id` — delete.

Auth: webhook endpoints take no bearer at the Gin level (signature verified inside the adapter). CRUD endpoints require `auth.PermDeployCreate` (same level as creating a deploy manually).

### T7 — Wiring + config

`cmd/api/main.go` — construct repository, service, handler; pass the encryption key; register routes. Register `generic` + `railway` adapters.

### T8 — Tests

- `internal/integrations/deploys/service_test.go` — happy path (creates record deployment), dedup returns existing ID, unmapped env is no-op, failed event creates no deployment.
- `internal/integrations/deploys/railway_test.go` — signature rejects bad HMAC, parses a captured Railway payload fixture.
- `internal/integrations/deploys/generic_test.go` — bearer + HMAC both accepted per integration `auth_mode`.
- `internal/integrations/deploys/handler_test.go` — generic endpoint happy path; Railway endpoint happy path; missing integration → 404; bad signature → 401.

### T9 — Docs

- `Deploy_Integration_Guide.md` — new "Wiring a PaaS deploy webhook" section covering (a) Railway signed HMAC flow, (b) generic bearer flow for CI/GitHub Actions.
- `Current_Initiatives.md` — flip row note through Phase 6.

## Verification

```bash
go build ./...
go test  ./...
make migrate-up    # apply 056
```

## Out of scope

- Additional provider adapters (Render/Fly/Heroku/Vercel/Netlify/GitHub Actions) — Phase 7.
- MCP setup tools — deferred; the events read-side endpoint keeps the door open.
