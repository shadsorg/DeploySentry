# Agentless Deploy Reporting — Phase 7: Remaining provider adapters + MCP setup tools

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope

Build on Phase 6's shared ingestion to add the remaining first-party provider adapters + three MCP setup tools.

Provider adapters (each is a single-file implementation of `DeployEventAdapter`):

- **Render** — `Render-Webhook-Signature: t=<ts>,v1=<hex>` over `ts + "." + body`
- **Fly.io** — `X-Fly-Signature: sha256=<hex>` (same shape as our generic HMAC; Fly's native webhook story is lightweight)
- **Heroku** — `Heroku-Webhook-Hmac-SHA256: <base64>` over raw body
- **Vercel** — `x-vercel-signature: <hex>` (no `sha256=` prefix) over raw body
- **Netlify** — `x-webhook-signature: sha256=<hex>` (shared-secret HMAC variant)
- **GitHub Actions** — `X-Hub-Signature-256: sha256=<hex>` with `workflow_run` event shape

MCP tools:

- **`ds_reporting_setup_deploy_integration`** — thin wrapper around `POST /api/v1/integrations/deploys` for any provider (including generic). Returns the webhook URL, signing secret, and provider-specific setup instructions.
- **`ds_reporting_check_deploy_integration`** — polls `GET /integrations/deploys/:id/events` and reports whether any event landed within the last N minutes.
- **`ds_reporting_verify`** — composite check: deployment recorded + status sample fresh + recent integration events.

## Tasks

### T1 — Render adapter

`internal/integrations/deploys/render.go`:
- Parses `Render-Webhook-Signature: t=<ts>,v1=<hex>`; verifies `HMAC_SHA256(ts + "." + body, secret)` against `v1`.
- Recognizes `deploy_ended/started/failed` event types.
- Status → event-type mapping: `live/succeeded → deploy.succeeded`, `update_failed/build_failed/failed → deploy.failed`, `created/build_in_progress → deploy.started`.
- Version falls back to `data.deploy.id` / `data.deploy.commit.id` when the extractor chain returns nothing.
- `service.id` in `data.service.id` is the match key for the integration lookup.

### T2 — Fly adapter

`internal/integrations/deploys/fly.go`:
- HMAC via `X-Fly-Signature: sha256=<hex>` (same format as generic).
- Payload permissive; extracts `app.name` as environment key by default and `release.version` / `image.ref` as version.

### T3 — Heroku adapter

`internal/integrations/deploys/heroku.go`:
- Signature: `Heroku-Webhook-Hmac-SHA256` header carries base64 HMAC of raw body.
- Filters for `resource=release` and `action=create`.
- Status `succeeded → deploy.succeeded`, `failed → deploy.failed`; others ignored.
- Environment = `data.app.name` by default (users map Heroku app names to DS envs).
- Version falls back to `data.slug.commit` or `fmt.Sprintf("v%d", data.version)`.

### T4 — Vercel adapter

`internal/integrations/deploys/vercel.go`:
- Signature: `x-vercel-signature` (hex, no prefix).
- `type=deployment.succeeded/deployment.failed/deployment.created` → canonical event types.
- Environment = `payload.target` (`production` / `preview`). Version = `payload.deployment.id` (falls back to commit sha via extractor).

### T5 — Netlify adapter

`internal/integrations/deploys/netlify.go`:
- Signature: `x-webhook-signature: sha256=<hex>` (HMAC of raw body).
- `state=ready/building/error` → canonical types.
- Environment = `context` (typically `production`, `deploy-preview`, `branch-deploy`).
- Version fallback: `commit_ref` → `deploy.id`.

### T6 — GitHub Actions adapter

`internal/integrations/deploys/github_actions.go`:
- Signature: `X-Hub-Signature-256: sha256=<hex>` (HMAC of raw body).
- Only process `workflow_run` events with `action=completed`.
- `conclusion=success → deploy.succeeded`, `failure/cancelled/timed_out → deploy.failed`.
- Environment = `workflow_run.head_branch` or a user-configured mapping key.
- Version = `workflow_run.head_sha`.
- Match key = `workflow_run.workflow_id` (or `workflow_run.repository.full_name` via `provider_config.repository`).

### T7 — Handler integration-lookup switch

Extend `lookupProviderIntegration` in `handler.go` with per-provider match predicates for every adapter. Keep the default permissive-matcher fallback for adapters whose payloads don't expose a stable identifier.

### T8 — Tests

`internal/integrations/deploys/providers_test.go`: one table-driven test per adapter covering:
- Signature verification (happy path + tampered signature).
- Payload parse to canonical event (success + failure event mapping).
- Version extractor override honored.

### T9 — MCP tools

`internal/mcp/tools_reporting.go`:
- `ds_reporting_setup_deploy_integration` — POSTs `/api/v1/integrations/deploys`, returns a map with `webhook_url`, `integration_id`, `signing_secret`, and provider-specific `instructions` (per-provider prose + env-var names).
- `ds_reporting_check_deploy_integration` — GETs `/integrations/deploys/:id/events?limit=…`; returns `pass/fail` + most-recent event timestamp.
- `ds_reporting_verify` — composite that hits `current-state` + `events` and returns a per-check pass/fail table.

Register all three in `server.go`.

### T10 — Docs

- `docs/Deploy_Integration_Guide.md`: expand the PaaS webhook section with Render/Fly/Heroku/Vercel/Netlify/GitHub Actions sub-notes — each lists signing header, default version path, and env-mapping hint.
- `docs/Current_Initiatives.md`: flip the row to Complete when all phases are in; otherwise update the note.

## Verification

```bash
go build ./...
go test  ./...
```

## Out of scope

- Cloudflare Pages / AWS App Runner / GCP Cloud Run / Azure Container Apps / DO App Platform adapters — incremental; each is a single-file PR matching the established pattern.
- Payload fixtures captured from real provider webhooks (defer until a real customer sends one — the adapters are permissive enough to tolerate minor drift).
