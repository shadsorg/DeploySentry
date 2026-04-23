# Agentless Deploy Reporting — Phase 1: `mode=record` + env filter

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope (Phase 1 only)

Ship the smallest slice that lets a caller **record a deploy that the platform (Railway, etc.) already orchestrated**, and wire the `?environment_id=` filter through the list handler so a per-env history view is possible. No `/status`, no webhook adapters, no SDK work — those are later phases.

Specifically:

1. `mode` column on `deployments` (`orchestrate` default, `record` opt-in).
2. `source` column on `deployments` (nullable; audit trail for `record` mode).
3. `POST /api/v1/deployments` accepts `mode` + `source` in the request body. `mode=record` inserts `status=completed`, `traffic_percent=100`, `started_at = completed_at = now()` and emits `deployment.recorded`.
4. `GET /api/v1/deployments` handler parses `?environment_id=` and `?limit=` / `?offset=` (repo already filters by env; only the handler change is needed).
5. New `WebhookEvent` constant `EventDeploymentRecorded` in `internal/models/webhook.go`, appended to `AllWebhookEvents()`.
6. Docs: add "Reporting from a PaaS" intro section to `docs/Deploy_Integration_Guide.md` with a `curl` example showing `mode=record`.

## Tasks

### T1 — Migration 054: `mode` + `source` columns

- `migrations/054_add_deployment_mode_source.up.sql`:
  ```sql
  ALTER TABLE deployments
      ADD COLUMN mode   TEXT NOT NULL DEFAULT 'orchestrate',
      ADD COLUMN source TEXT;
  ALTER TABLE deployments
      ADD CONSTRAINT deployments_mode_check CHECK (mode IN ('orchestrate', 'record'));
  CREATE INDEX deployments_app_env_created_idx
      ON deployments (application_id, environment_id, created_at DESC);
  ```
- `migrations/054_add_deployment_mode_source.down.sql`: drop the index, constraint, and columns.
- Existing rows default to `mode='orchestrate'`, `source=NULL` — no behavioural change for current callers.

### T2 — Model

- Add to `models.Deployment` (`internal/models/deployment.go:68`):
  ```go
  Mode   DeployMode `json:"mode" db:"mode"`
  Source *string    `json:"source,omitempty" db:"source"`
  ```
- Add `DeployMode` type + constants in same file:
  ```go
  type DeployMode string
  const (
      DeployModeOrchestrate DeployMode = "orchestrate"
      DeployModeRecord      DeployMode = "record"
  )
  ```
- Update `Validate()` to accept empty/nil and default to `orchestrate`.

### T3 — Webhook event constant

- In `internal/models/webhook.go`, add `EventDeploymentRecorded WebhookEvent = "deployment.recorded"` alongside the existing `EventDeployment*` block.
- Append to `AllWebhookEvents()`.

### T4 — Postgres repository

- Update `deploymentSelectCols` and `scanDeployment` in `internal/platform/database/postgres/deploy.go:30` to include `mode, source`.
- Update `CreateDeployment` INSERT column list + placeholders + args.
- Update `UpdateDeployment` if it writes `mode` or `source` (check and include if present).
- No new method needed — `ListDeployments` already honors `ListOptions.EnvironmentID`.

### T5 — Service

- In `internal/deploy/service.go`, extend `CreateDeployment`:
  - Default `d.Mode = DeployModeOrchestrate` when empty.
  - When `d.Mode == DeployModeRecord`:
    - Set `d.Status = DeployStatusCompleted`, `d.TrafficPercent = 100`, `d.StartedAt = &now`, `d.CompletedAt = &now`.
    - Skip the phase engine / rollout path entirely (no `publishEvent("deployment.created", …)`; we'll publish `deployment.recorded` from the handler).
    - Still look up `PreviousDeploymentID` for continuity in history.
  - Return the created deployment unchanged otherwise.
- Add a `// Record mode skips the phase engine …` comment on the new branch explaining the semantic difference.

### T6 — Handler

- In `internal/deploy/handler.go:120`, extend `createDeploymentRequest`:
  ```go
  Mode     string  `json:"mode,omitempty"`      // "" | "orchestrate" | "record"
  Source   string  `json:"source,omitempty"`    // free-text audit (e.g. "railway-webhook", "manual")
  ```
- In `createDeployment`:
  - If `req.Mode == "record"`, copy `Mode` + `Source` into `d` before calling the service.
  - When `mode=record`, `strategy` may be empty (relax validation inside the handler — service still validates `artifact`/`version`).
  - After `h.service.CreateDeployment` succeeds, choose webhook event:
    - `record` → publish `EventDeploymentRecorded` with the same data payload shape as `deployment.created` plus `"mode":"record"` and `"source":…`.
    - `orchestrate` (or empty) → keep current `EventDeploymentCreated` publish.
  - Rollout attachment block (`h.rollouts != nil && req.Rollout != nil`) is skipped when `mode=record` — recorded deploys do not attach rollouts.
- Extend `listDeployments`:
  - Parse `?environment_id=` into `opts.EnvironmentID` (validate UUID; `400` on parse error).
  - Parse `?limit=` and `?offset=` (integers; ignore invalid values — service clamps).

### T7 — Tests

Add to `internal/deploy/handler_test.go`:
- `TestCreateDeployment_ModeRecord_Succeeds` — POST with `mode=record`, assert returned deployment has `status=completed`, `traffic_percent=100`, `started_at` and `completed_at` both set, `Mode=record`. Assert the mock webhook service was invoked with `EventDeploymentRecorded`, not `EventDeploymentCreated`.
- `TestCreateDeployment_ModeRecord_StrategyOptional` — POST with `mode=record` and no `strategy`, expect `201`.
- `TestCreateDeployment_ModeRecord_SkipsRollout` — even when a `rollout` block is present in the request, confirm `RolloutAttacher.AttachFromDeployRequest` is never called.
- `TestListDeployments_EnvFilter` — GET `?app_id=…&environment_id=…`, assert `opts.EnvironmentID` was populated on the service call.
- `TestListDeployments_InvalidEnvID` — GET with malformed `environment_id`, expect `400`.

Add to `internal/deploy/service_test.go`:
- `TestCreateDeployment_Record_SetsCompletedNow` — calls service with `Mode=record`, asserts resulting record has terminal state + timestamps.
- `TestCreateDeployment_Orchestrate_Unchanged` — regression; existing orchestrate path still produces `pending` + publishes `deployment.created`.

Repo test (`internal/platform/database/postgres/deploy_test.go` if present, otherwise skip — the model/service tests above cover the behavior):
- `TestCreateAndGetDeployment_WithMode` — round-trip a `record` mode deployment, verify columns persist.

### T8 — Docs

- `docs/Deploy_Integration_Guide.md`:
  - Add a new top-level section "Reporting deploys from a PaaS (Railway, Render, …)" near the top, before the existing agent-centric content.
  - Under it, a single subsection for Phase 1: "Recording platform-driven deploys via the API" with a `curl` example:
    ```bash
    curl -X POST https://api.deploysentry.com/api/v1/deployments \
      -H "Authorization: Bearer $DS_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{
        "application_id": "…",
        "environment_id": "…",
        "artifact": "image:registry/app:1.4.2",
        "version": "1.4.2",
        "commit_sha": "abc123",
        "mode": "record",
        "source": "manual"
      }'
    ```
  - Note that `mode=record` deploys appear in history immediately with `status=completed` and do not trigger the rollout engine.
  - Cross-reference the agentless reporting spec.
- `docs/Current_Initiatives.md`: flip the "Agentless Deploy Reporting" row to `Implementation` when this PR lands.

## Verification

```bash
make migrate-up                             # apply 054
go build ./...
go test ./internal/deploy/... ./internal/models/... ./internal/platform/database/postgres/...
```

Manual sanity check (against a local stack):

```bash
# should return 201 with status=completed, traffic_percent=100
curl -sX POST localhost:8080/api/v1/deployments \
  -H "Authorization: Bearer $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"application_id":"…","environment_id":"…","artifact":"img:1","version":"1.0.0","mode":"record","source":"manual"}' | jq

# should filter by env
curl -s "localhost:8080/api/v1/deployments?app_id=…&environment_id=…" \
  -H "Authorization: Bearer $DS_API_KEY" | jq
```

## Rollback

- Migration down drops the two new columns; `mode`/`source` fields on the Go struct become no-ops (never populated by existing callers).
- Handler/service changes are purely additive — omitting `mode` preserves today's behavior exactly.
- No data migration is required in either direction.

## Out of scope for this phase (already in spec, deferred to later plans)

- `POST /applications/:id/status` + `app_status` tables (Phase 2).
- `GET /current-state` read-side assembly (Phase 3).
- SDK reporter (Phases 4–5).
- Deploy-event ingestion + Railway adapter (Phase 6).
- Remaining provider adapters (Phase 7).
