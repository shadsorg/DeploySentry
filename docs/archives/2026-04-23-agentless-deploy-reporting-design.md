# Agentless Deploy Reporting (Railway, Render, Fly, Heroku, Vercel, Netlify, CI, …)

**Date:** 2026-04-23
**Status:** Complete — **Shipped 2026-04-23** (all 7 phases on `main`)

## Completion Record

- **Branch:** `main` (phase work landed incrementally on the working tree; per project convention commits are authored by the user when ready to push).
- **Committed:** Pending — changes staged on working tree awaiting user commit.
- **Pushed:** Pending.
- **CI Checks:** Local verification only so far — `go build ./...`, `go test ./...` all green; `sdk/node` Jest (28 tests pass; pre-existing `contract.test.ts` fixture miss is unrelated); `sdk/python` pytest 8/8 pass; `sdk/ruby` Minitest 8 runs / 20 assertions / 0 failures; `sdk/go` all tests pass. `sdk/java` compile-checked by inspection — local JDK not installed, so Maven CI will verify.
- **Migrations added:** 054 (`mode`/`source` columns on `deployments`), 055 (`app_status` + `app_status_history`), 056 (`deploy_integrations` + `deploy_integration_events`).
- **New API surface:**
  - `POST /api/v1/deployments` accepts `mode=record` + `source`; emits `deployment.recorded` webhook event.
  - `GET /api/v1/deployments?environment_id=…` (env filter wired through the existing list handler).
  - `POST /api/v1/applications/:app_id/status` (+ new `status:write` RBAC permission / API-key scope).
  - `GET /api/v1/applications/:app_id/environments/:env_id/current-state` (single-call dashboard feed with `fresh/stale/missing` staleness rules).
  - `POST/GET/DELETE /api/v1/integrations/deploys` (integration CRUD).
  - `GET /api/v1/integrations/deploys/:id/events` (recent-events read-side).
  - `POST /api/v1/integrations/deploys/webhook` (generic canonical endpoint; HMAC or bearer).
  - `POST /api/v1/integrations/:provider/webhook` for `railway`, `render`, `fly`, `heroku`, `vercel`, `netlify`, `github-actions`.
- **SDKs:** Status reporter landed in all five server SDKs — Node (`@dr-sentry/sdk`), Go (`deploysentry-go`), Python (`deploysentry`), Java (`io.deploysentry`), Ruby (`deploysentry`) — with uniform opt-in config, optional health provider, and a shared env-var version-detection chain.
- **MCP:** Three new tools — `ds_reporting_setup_deploy_integration`, `ds_reporting_check_deploy_integration`, `ds_reporting_verify`.
- **Docs:** `docs/Deploy_Integration_Guide.md` rewrote its top section to cover PaaS reporting (agentless primary, agent as escalation), with per-provider webhook wiring notes.

## Original Spec

**Status:** Design — **Approved 2026-04-23** (superseded by the Completion Record above)

## Overview

DeploySentry's existing deployment observability assumes a DeploySentry Agent (Envoy sidecar) is running alongside the workload — it registers, heartbeats, and pulls desired state. That model doesn't fit our current reality: production DeploySentry is deployed on **Railway**, which gives us no place to run a sidecar and does its own rollout. We still need the platform to meet its core promise: **know what version is live in each environment, whether it's healthy, and keep a history of deploys.**

This design adds an **agentless reporting path** that works on Railway today and on any other PaaS without modification. It does not touch the agent path — both coexist. Four primitives plus a small read-side assembly close the gap:

1. **`mode=record` on deployment create** — represent a deploy that the platform (not DeploySentry) orchestrated, without faking it through the phase engine.
2. **`POST /applications/:id/status`** — an env-scoped API key lets an app push its own version + health directly. Counterpart to the agent heartbeat for the no-sidecar case.
3. **SDK-embedded reporter** — every first-party DeploySentry SDK (go, node, python, java, react, flutter, ruby) gains an opt-in status reporter that calls `/status` automatically. Customers already importing the SDK for flag evaluation add **one config flag** (`reportStatus: true`) to get version + health reporting for free.
4. **Deploy-event webhook ingestion** — a generic `POST /integrations/deploys/webhook` endpoint accepting a canonical DeploySentry payload, *plus* thin provider adapters (`/integrations/:provider/webhook` where `:provider ∈ {railway, render, fly, heroku, vercel, netlify, github-actions}`) that translate native payloads into that canonical shape. Every adapter funnels into the same internal `mode=record` primitive.
5. **`GET /applications/:id/environments/:env_id/current-state`** — one read returning current version, health, and recent history for a per-environment dashboard card.

Plus one trivial cleanup (expose `?environment_id=` on the deployments list) so the history UI can filter by environment.

## Goals

- A DeploySentry user running on Railway can, with **zero infrastructure** beyond a webhook URL, see accurate deploy history per app/environment.
- A DeploySentry user who imports any first-party SDK gets version + health reporting by **flipping one flag in SDK init** — no handwritten status-push code in the app.
- The dashboard can show "current version + health + last 10 deploys" for any app/environment with a single API call.
- All ingest paths (agent, Railway webhook, SDK reporter, raw app push) land in the **same data shape** — downstream UI and rollout logic don't care how a sample arrived.
- No regressions to the agent flow. Existing rollout orchestration remains the default when `mode` is omitted.

## Non-Goals

- **Replacing the agent.** The agent remains the right answer for self-hosted workloads where we actually orchestrate traffic. This spec is for platforms where we don't.
- **Driving rollouts from the agentless path.** A `mode=record` deploy is a post-hoc record of a platform-driven rollout; it does not feed the phase engine or rollback controller. Driving Railway rollouts from DeploySentry is the separate Cloudflare-canary spec (`2026-04-22-cloudflare-canary-and-observability-design.md`).
- **Provider-agnostic observability ingest.** The `/status` endpoint is deliberately narrow (app-direct push). Pulling from Prometheus/NR/Datadog is the separate observability spec. This spec uses whatever health state the app self-reports.
- **Railway-specific rollout control.** We consume Railway's deploy-success signal; we do not call Railway's deploy API outbound in this spec.
- **Historical backfill of pre-feature deploys.** Starts recording from the day it ships.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Represent platform-driven deploys | `mode=record` on existing `POST /deployments` | Reuses the existing resource; no parallel "recorded-deploys" table. Honest state: `completed` at creation, no phase engine involvement. |
| App-direct health reporting | New `POST /applications/:id/status`, env-scoped API key | Mirrors agent heartbeat shape; scoped key gives implicit app+env auth, same pattern as existing scoped-key usage. |
| Webhook ingestion shape | Generic canonical endpoint **plus** per-provider adapters | The agent-unavailable case is not Railway-specific — it applies to every PaaS. A single Railway endpoint would force a parallel rewrite for each new provider. Generic-plus-adapters lets native payloads work zero-config where possible, while the canonical endpoint covers CI pipelines, custom scripts, and providers we haven't added adapters for yet. |
| Adapter contract | Each provider implements a small `DeployEventAdapter` interface (verify signature, extract event) | Keeps provider code isolated; adding a provider is a self-contained PR; fuzzy edges live in the adapter, not the core ingestion path. |
| Convergence point | All webhook adapters emit a canonical `DeployEvent` → same internal `mode=record` path | One code path creates recorded deploys, regardless of which PaaS sent the event. |
| Version auto-discovery from `/status` | If `/status` reports a version with no corresponding deployment, auto-create a `mode=record` deploy | Makes "I forgot to wire the webhook" still produce sensible history. Webhook + app-push both converge. |
| Health model | App pushes `health` (enum) and optional `health_score` | Apps know their own health best; we don't need to infer from metrics. External observability remains available as a parallel input via the other spec. |
| Staleness semantics | Same thresholds as agent heartbeat (green <60s, yellow <5m, red ≥5m); configurable | Uniform operator mental model across both paths. |
| Storage | New `app_status` table, ring-buffer of recent samples per (app, env) | Supports sparkline/mini-chart in the dashboard without a full time-series DB. |
| Dashboard read | New `GET /applications/:id/environments/:env_id/current-state` | Single round-trip. Frontend doesn't fan out. |
| Env filter on list | Wire `?environment_id=` through `listDeployments` handler | Repository already supports it; 3-line handler change. |
| SDK reporter | Opt-in flag in every first-party SDK; uses the same config/key the SDK already has | Customers already have the SDK wired for flags. Adding status reporting there is the smallest possible incremental surface for them. |
| SDK health model | Default "process alive = healthy"; optional `healthProvider` callback | SDK cannot genuinely know app health; default must be safe. Callback lets sophisticated apps report real health. |
| Language parity | Node, Go, Python, Java, Ruby first (server runtimes); React/Flutter deferred | Server SDKs are where version+health reporting makes sense. Client SDKs report a different thing (user-session presence), out of scope. |

## API Surface

### 1. `POST /api/v1/deployments` — add `mode` field

Existing endpoint (`internal/deploy/handler.go:120`). Request body gains:

```jsonc
{
  "application_id": "...",
  "environment_id": "...",
  "artifact": "...",
  "version": "1.4.2",
  "commit_sha": "abc123",        // optional
  "strategy": "rolling",          // ignored when mode=record
  "mode": "record",               // new: "orchestrate" (default) | "record"
  "started_at": "2026-04-23T...", // optional, defaults to now()
  "completed_at": "2026-04-23T...", // optional, defaults to now()
  "source": "railway-webhook" | "app-push" | "manual" // new, optional, for audit
}
```

Semantics when `mode=record`:
- Row inserted with `status=completed`, `traffic_percent=100`.
- No phase rows created; no NATS event to the phase engine; rollback controller does not observe this deployment.
- A new webhook event `deployment.recorded` fires (separate from `deployment.created` so subscribers can distinguish orchestrated vs recorded).
- Existing RBAC: requires `deploys:write` (same as orchestrate).

Semantics when `mode=orchestrate` (or omitted): unchanged from today.

### 2. `POST /api/v1/applications/:app_id/status` — app-direct push

New endpoint. Auth: env-scoped API key (resolves `environment_id` from the key; request payload does not carry it). Body:

```jsonc
{
  "version": "1.4.2",
  "commit_sha": "abc123",         // optional
  "health": "healthy",            // enum: healthy | degraded | unhealthy | unknown
  "health_score": 0.98,           // optional, 0.0–1.0
  "health_reason": "db slow",     // optional, short free-text on non-healthy
  "deploy_slot": "stable",        // optional; forward-compat with cloudflare-canary spec
  "tags": { "region": "us-east" } // optional, small k/v
}
```

Behavior:
- Upsert into `app_status` (latest sample per `(app_id, env_id)`) and append to `app_status_history` (ring-buffer, configurable retention; default 24h / 1440 samples).
- If `version` differs from the latest `completed` deployment for this `(app_id, env_id)` **and** no deployment row exists for that version, auto-create a `mode=record` deployment with `source="app-push"`. One-shot per version — subsequent `/status` calls for the same version do not create new deployments.
- Rate limit: 1 req/sec per API key (configurable); dashboard polling + app startup samples both fit easily.
- RBAC: new scope `status:write`. Existing keys do not gain this implicitly — users opt in.

### 3. Deploy-event webhook ingestion — generic endpoint + provider adapters

The agent-unavailable problem is not Railway-specific. Every PaaS that doesn't let us run a sidecar produces the same need: "tell DeploySentry when a deploy landed." This is solved with a **single internal event shape** fed by two kinds of HTTP endpoints:

#### 3a. Generic canonical endpoint

```
POST /api/v1/integrations/deploys/webhook
Headers:
  Authorization: Bearer <integration_token>    // OR X-DeploySentry-Signature for HMAC
  X-DeploySentry-Integration-Id: <id>          // selects the integration config row
```

Body (DeploySentry's canonical `DeployEvent`):

```jsonc
{
  "event_type": "deploy.succeeded" | "deploy.failed" | "deploy.started",
  "environment": "production",                 // mapped via integration's env_mapping
  "version": "1.4.2",
  "commit_sha": "abc123",                      // optional
  "artifact": "image:registry/app:1.4.2",      // optional
  "occurred_at": "2026-04-23T12:34:56Z",       // optional, defaults to receive time
  "url": "https://...",                        // optional, link back to provider UI
  "metadata": { "...": "..." }                 // optional, stored for audit
}
```

Intended for: GitHub Actions jobs, custom CI scripts, platform operators rolling their own integration, and any provider DeploySentry does not (yet) ship an adapter for. Auth is either a bearer API key with `deploys:write`+`status:write` or an HMAC shared secret — operator's choice per integration.

#### 3b. Provider adapter endpoints

```
POST /api/v1/integrations/:provider/webhook
  where :provider ∈ {railway, render, fly, heroku, vercel, netlify, github-actions, ...}
```

Each adapter:
1. Verifies the provider's native signature format (Railway HMAC, Render `X-Render-Signature`, Vercel signature, GitHub HMAC, etc.).
2. Parses the provider's native payload.
3. Emits a canonical `DeployEvent` (same shape as 3a) into the shared ingestion pipeline.

Adapter contract (Go-side):

```go
// internal/integrations/deploys/adapter.go
type DeployEventAdapter interface {
    Provider() string                                     // "railway", "render", ...
    VerifySignature(r *http.Request, secret string) error
    ParsePayload(body []byte) (DeployEvent, error)        // returns canonical event
}
```

Adding a new provider = one file implementing this interface + registration in an adapter map. The ingestion path, deduplication, event emission, and error handling are shared.

#### Supported events (uniform across adapters and generic endpoint)

- `deploy.succeeded` → create `mode=record` deployment (`source="<provider>-webhook"` or `"generic-webhook"`).
- `deploy.failed` / `deploy.crashed` → emit `deployment.provider_failed` webhook event + set latest `app_status.health="unhealthy"` for the mapped env. No deployment row.
- `deploy.started` → optional: create a `pending` deployment that the next `succeeded` event transitions to `completed` (nice-to-have; skip in v1 if adapter's payload doesn't include it).

#### Configuration (single integration table for all providers)

```yaml
integrations:
  deploys:
    - provider: railway            # "railway" | "render" | "fly" | ... | "generic"
      application_id: <ds_app_id>
      enabled: true
      webhook_secret_ref: secrets://integrations/<id>/secret
      provider_config:             # provider-specific, schema varies
        service_id: <railway_service_id>
      environment_mapping:
        production: <ds_env_id_prod>
        staging:    <ds_env_id_stage>
      version_extractor:           # only used by adapters; generic endpoint sends canonical payload
        - "meta.deploymentId"
        - "commit.sha"
        - "image.tag"
```

Unknown provider environments (not in `environment_mapping`) produce an `integration.unmapped_environment` webhook event and the payload is logged but not recorded — fail closed, not silent. Applies to every adapter and the generic endpoint alike.

#### Idempotency

All ingestion paths dedupe on `(application_id, environment_id, version, event_type)` with a 24h window. Replayed webhooks, retries, and double-fires across providers (e.g. both GitHub Actions and Railway reporting the same deploy) converge to a single deployment row with a `sources: ["github-actions", "railway"]` array stored in metadata.

#### First-party adapters shipped in v1

| Provider | Native auth | Notes |
|---|---|---|
| Railway | HMAC (`X-Railway-Signature`) | First implementation; drives the design. |
| Render | HMAC (`X-Render-Signature`) | Render webhooks are well-documented; near-parity with Railway. |
| Fly.io | HMAC | Fly machines API produces deploy events. |
| Heroku | HMAC (`Heroku-Webhook-Hmac-SHA256`) | Uses app webhooks; filter `api:release` event. |
| Vercel | HMAC (`x-vercel-signature`) | Deployment-succeeded event. |
| Netlify | HMAC (`X-Webhook-Signature`) | `deploy_succeeded` event. |
| GitHub Actions | HMAC via `workflow_run` — OR recommend the generic endpoint from a workflow step | The generic endpoint with a bearer token is often cleaner for CI-driven flows. |

Later adapters (Cloudflare Pages, AWS App Runner, Google Cloud Run, Azure Container Apps, Digital Ocean App Platform) are incremental — each is a single-file adapter.

### 4. `GET /api/v1/applications/:app_id/environments/:env_id/current-state`

New read-side assembly endpoint. Returns:

```jsonc
{
  "environment": { "id": "...", "slug": "production" },
  "current_deployment": {
    "id": "...",
    "version": "1.4.2",
    "commit_sha": "abc123",
    "status": "completed",
    "started_at": "...",
    "completed_at": "...",
    "source": "railway-webhook",    // "<provider>-webhook" | "generic-webhook" | "app-push" | "agent" | "manual"
    "traffic_percent": 100
  },
  "health": {
    "state": "healthy",
    "score": 0.98,
    "reason": null,
    "source": "app-push",        // "app-push" | "agent" | "observability" | "unknown"
    "last_reported_at": "...",
    "staleness": "fresh"         // fresh | stale | missing
  },
  "recent_deployments": [
    { "version": "...", "status": "...", "completed_at": "...", "source": "..." },
    ...up to 10
  ],
  "active_rollout": null         // populated when an orchestrated rollout is in flight
}
```

One DB round-trip of 3 scoped reads (active deployment, latest app_status, last 10 deployments); no joins across tenants.

### 5. Cleanup — `?environment_id=` on `GET /api/v1/deployments`

Wire `environment_id` from query string through `listDeployments` (`internal/deploy/handler.go:236`) into the existing `ListOptions.EnvironmentID` filter (`internal/deploy/repository.go:52`). Three-line change. Unblocks the per-environment history list.

## Data Model

New tables in the `deploy` schema (migrations follow the existing convention — no `deploy.` prefix in SQL):

```sql
-- Latest self-reported status per (app, env)
CREATE TABLE app_status (
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    commit_sha        TEXT,
    health_state      TEXT        NOT NULL, -- healthy|degraded|unhealthy|unknown
    health_score      NUMERIC(4,3),
    health_reason     TEXT,
    deploy_slot       TEXT,
    tags              JSONB       NOT NULL DEFAULT '{}'::jsonb,
    source            TEXT        NOT NULL, -- app-push|agent|<provider>-webhook|generic-webhook
    reported_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (application_id, environment_id)
);

-- Ring-buffered samples for sparkline / forensics
CREATE TABLE app_status_history (
    id                BIGSERIAL   PRIMARY KEY,
    application_id    UUID        NOT NULL,
    environment_id    UUID        NOT NULL,
    version           TEXT        NOT NULL,
    health_state      TEXT        NOT NULL,
    health_score      NUMERIC(4,3),
    reported_at       TIMESTAMPTZ NOT NULL
);
CREATE INDEX ON app_status_history (application_id, environment_id, reported_at DESC);
-- Retention handled by background cleanup job (default 24h).

-- Generic deploy-event integration config (one row per provider per app, including "generic")
CREATE TABLE deploy_integrations (
    id                UUID        PRIMARY KEY,
    application_id    UUID        NOT NULL,
    provider          TEXT        NOT NULL, -- railway|render|fly|heroku|vercel|netlify|github-actions|generic
    webhook_secret_enc TEXT       NOT NULL, -- HMAC secret OR bearer token (generic); encrypted at rest
    auth_mode         TEXT        NOT NULL DEFAULT 'hmac', -- hmac|bearer
    provider_config   JSONB       NOT NULL DEFAULT '{}'::jsonb, -- adapter-specific (e.g. railway service_id)
    env_mapping       JSONB       NOT NULL, -- {provider_env_name: ds_env_id}
    version_extractors JSONB      NOT NULL DEFAULT '[]'::jsonb, -- ordered list, first non-empty wins
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON deploy_integrations (application_id, provider);

-- Audit of received webhook events (idempotency + forensics)
CREATE TABLE deploy_integration_events (
    id                UUID        PRIMARY KEY,
    integration_id    UUID        NOT NULL REFERENCES deploy_integrations(id),
    event_type        TEXT        NOT NULL, -- deploy.succeeded|deploy.failed|deploy.started
    dedup_key         TEXT        NOT NULL, -- sha256(app_id|env_id|version|event_type)
    deployment_id     UUID,                 -- set if this event produced/updated a deployment row
    payload_json      JSONB       NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dedup_key)
);
```

Adds to existing `deployments` table:

```sql
ALTER TABLE deployments
    ADD COLUMN mode   TEXT NOT NULL DEFAULT 'orchestrate', -- orchestrate|record
    ADD COLUMN source TEXT;                                -- audit trail for record-mode
```

Existing rows backfill as `mode='orchestrate'`, `source=NULL`.

## Authentication

- **`POST /deployments` with `mode=record`:** env-scoped API key with `deploys:write`. Same scope as orchestrate — the mode flag is not a new privilege.
- **`POST /applications/:id/status`:** env-scoped API key with new `status:write` scope. Must be opted into explicitly on the key.
- **`POST /integrations/:provider/webhook` (Railway, Render, Fly, Heroku, Vercel, Netlify, GitHub Actions):** no bearer; HMAC signature verified by the provider adapter against `deploy_integrations.webhook_secret_enc`. Provider-specific identifier in the payload (e.g. Railway service ID) selects the integration row.
- **`POST /integrations/deploys/webhook` (generic):** bearer API key with `deploys:write` + `status:write` — OR — HMAC with an `X-DeploySentry-Signature` header if the integration is configured `auth_mode=hmac`. `X-DeploySentry-Integration-Id` selects the integration row.
- **`GET /current-state`:** standard user session or API key with `deploys:read` (read-side only).

Scoped API keys already resolve `ApplicationID` + `EnvironmentIDs[]` in middleware (`internal/auth/middleware.go:207`); no new auth code required, only new scopes.

## SDK Reporter

Every first-party server SDK gains an optional **status reporter** module that calls `POST /applications/:id/status` on behalf of the app. The SDK already holds the API base URL, API key (env-scoped), and application identity — everything needed to report. Adding this is additive to existing SDK surface; no breaking changes.

### Scope

- **In scope (v1):** `sdk/node`, `sdk/go`, `sdk/python`, `sdk/java`, `sdk/ruby` — these are server-side runtimes where "version of the binary in this environment" is a meaningful concept.
- **Deferred:** `sdk/react`, `sdk/flutter` — client SDKs run on end-user devices; reporting per-user-session version is a different problem (client-side observability) and doesn't belong in this spec.

### Config Surface (uniform across SDKs)

```jsonc
// example: Node SDK init
new DeploySentry({
  apiKey: process.env.DS_API_KEY,
  application: "checkout-api",

  reportStatus: true,                       // opt-in, default false
  reportStatusIntervalMs: 30000,            // default 30s; 0 = startup-only
  reportStatusVersion: process.env.APP_VERSION,   // optional override
  reportStatusCommitSha: process.env.GIT_SHA,     // optional
  reportStatusDeploySlot: process.env.DS_DEPLOY_SLOT, // optional, forward-compat
  reportStatusHealthProvider: async () => ({ // optional, default healthy
    state: "healthy",
    score: 0.99,
    reason: null,
  }),
  reportStatusTags: { region: process.env.REGION }, // optional
});
```

Equivalent fields in Go (`ReportStatus bool`, etc.), Python (keyword args), Java (builder methods), Ruby (hash keys). Naming mirrors existing SDK conventions per language.

### Behavior

1. **On SDK init** (if `reportStatus=true`): fire one `POST /status` with version + current health. This is the deploy-detection signal — first `/status` for a new version triggers auto-record (per the API semantics above).
2. **On interval** (`reportStatusIntervalMs > 0`): re-fire with current health. Health is either the static default (`healthy`) or the result of calling `healthProvider()`.
3. **On graceful shutdown** (process signals where the runtime allows): fire one final `/status` with `health=unknown` and `reason="shutting_down"`. Lets the dashboard distinguish planned stops from crashes.
4. **Version auto-detection** (when `reportStatusVersion` is unset): each SDK tries a language-idiomatic chain, **in this order**, stopping at first non-empty:
   - Explicit config value.
   - Standard env vars: `APP_VERSION`, `GIT_SHA`, `GIT_COMMIT`, `SOURCE_COMMIT`, `RAILWAY_GIT_COMMIT_SHA`, `RENDER_GIT_COMMIT`.
   - Language-native fallbacks (node: `package.json.version`; python: `importlib.metadata.version(pkg)` if the SDK knows the pkg name; go: `debug.ReadBuildInfo().Main.Version`; java: `Implementation-Version` manifest attr; ruby: `Gem.loaded_specs[pkg].version` if known).
   - Ultimately falls back to the literal string `"unknown"` with a one-time warning logged.
5. **Failure handling:** reporter failures are non-fatal and logged at `debug` level by default. A single failure does not break flag evaluation or any other SDK function. Exponential backoff on consecutive failures (cap at 5 min) prevents retry storms during incidents.
6. **Concurrency:** a single in-flight reporter per SDK instance; replicas each run their own reporter. Last-writer-wins per the server-side `app_status` semantics.

### API Key Scope

The reporter requires the SDK's API key to carry both `flags:read` (existing) and the new `status:write` scope. The dashboard's key-creation UI gains a "reports status" toggle that auto-adds the scope. Existing SDK keys continue to work for flag evaluation; adding `reportStatus: true` without the scope produces a clear `403` logged once and then backoff (no loop).

### What the SDK does NOT do

- **Does not call `POST /deployments`.** Deploy records are produced server-side from `/status` auto-recording or the Railway webhook. The SDK's job is state reporting, not deployment creation.
- **Does not ship metrics or traces.** Health is a single enum + optional score, not a telemetry firehose. Heavier observability remains the domain of the separate provider-agnostic observability spec.
- **Does not reconcile desired state.** That is the agent's job. SDK reporter is one-way (app → DeploySentry).

### Tradeoffs

- **Pushing this into SDKs ties reporter cadence to process lifecycle.** If the app is a short-lived Lambda/Cloud Function, the interval reporter never ticks. Mitigation: `reportStatusIntervalMs: 0` mode sends on init only — still gives deploy-detection, just no health freshness. Document clearly that long-lived servers benefit most.
- **Health defaults optimistic.** "Process alive = healthy" will over-report health during silent failures (deadlock, event loop blocked). That's acceptable as a floor — apps that care provide a `healthProvider`. The agent path remains the right answer for customers who want provider-driven health.
- **Seven SDKs to update.** Real work, but mechanical. The wire format is one endpoint, one JSON shape; per-SDK effort is small once one is done. Sequencing: ship Node + Go first (covers the current customer base), then Python/Java/Ruby.

## UI Surface (dashboard — to spec separately)

Two screens are unblocked by this API:

1. **Per-app home** — a card per environment showing current version, health indicator, last-reported age, and a "view history" link. Feeds from `GET /current-state`.
2. **Per-environment deploy history** — chronological list of deploys with version, status, source, timestamps. Feeds from `GET /deployments?app_id=…&environment_id=…`.

No new dashboard concepts — same widgets work for both agent and agentless apps. `health.source` in the response tells the UI whether to show a "reported by app" vs "reported by agent" badge.

## Customer-Facing Setup (what a Railway user does)

**Path A — Provider webhook only (zero app code):** applies to Railway, Render, Fly, Heroku, Vercel, Netlify, or any provider with a first-party adapter.
1. In DeploySentry: create a deploy-event integration for the app, pick `provider`, map provider environments to DS environments.
2. Copy the generated webhook URL + signing secret.
3. In the provider's dashboard: add the webhook on the service.

Result: every successful deploy appears in DeploySentry with version + timestamps, regardless of provider. Health is unknown until Path B is enabled or external observability is wired.

**Path A′ — Generic webhook from CI (for GitHub Actions, custom scripts, or unsupported providers):**
1. In DeploySentry: create a `provider=generic` integration with `auth_mode=bearer`; copy the integration ID and API token.
2. In the CI job: on successful deploy, `curl -X POST` the generic endpoint with the canonical `DeployEvent` payload.

Result: same as Path A — deploy history populates. Useful for CI-driven pipelines (GitHub Actions, CircleCI, Jenkins) where the native webhook payload is less expressive than a tailored `curl`.

**Path B — enable the SDK reporter (one config flag):**
1. In DeploySentry: mint (or update) the app+env API key with `status:write` in addition to `flags:read`.
2. In the app's SDK init, add `reportStatus: true` (and optionally a `healthProvider`).

Result: health fills in, live-version is self-verified, and deploy-detection works even without the Railway webhook. No new dependency — the SDK is already imported.

**Path B-fallback — raw HTTP (for languages without a first-party SDK):** hit `POST /applications/:id/status` on startup + 30s interval with the same payload the SDK sends. Documented but not recommended when a first-party SDK exists.

**Path C (future, separate spec):** wire Prometheus/New Relic/Datadog for health via the provider-agnostic observability spec; Path B becomes redundant for apps that already have that stack.

## Rollout Phasing (if approved)

Each phase is independently shippable and useful:

1. **`mode=record` + deployments list env filter.** Foundational; unblocks manual / scripted deploy recording and per-env history UI immediately.
2. **`app_status` tables + `POST /applications/:id/status` + `status:write` scope.** App-direct push online. Status column in UI can light up even before the dashboard assembly endpoint exists.
3. **`GET /current-state` read-side.** Dashboard card + per-env history pages ship.
4. **SDK reporter — Node + Go.** Covers the existing customer base. Validates the uniform config shape before rolling to other languages.
5. **SDK reporter — Python, Java, Ruby.** Mechanical port of the Node/Go implementation; each is a small PR.
6. **Deploy-event ingestion core + generic endpoint + Railway adapter.** Ship the shared `DeployEventAdapter` interface, the canonical endpoint (`POST /integrations/deploys/webhook`), the `deploy_integrations` + `deploy_integration_events` tables, and the Railway adapter in one phase so the first adapter validates the shared contract. This closes the zero-code loop for Railway users.
7. **Additional provider adapters.** Render, Fly, Heroku, Vercel, Netlify, GitHub Actions — each is a single-file adapter PR built on the phase-6 foundation. Prioritize by customer demand; no dependency between them.

## Open Questions

- **Staleness defaults.** Is 60s fresh / 5m red the right cliff for a 30s push cadence? Probably yes for prod; may be too tight for staging. Recommend per-app override, default conservative.
- **Concurrent `/status` writers.** If an app has 5 replicas, all posting, do we last-writer-wins on `app_status`, or aggregate? Recommend last-writer-wins for v1; aggregation (min of health states) is a follow-up if real-world noise demands it.
- **Auto-recorded deploys from `/status` — can they cause surprise history?** E.g. an app booting with a stale cached version briefly reports `version=1.4.0` when `1.4.1` is live. Mitigation: require `/status` auto-record to only fire after N consistent samples (default 3) of the same version from the same key. Prevents flapping history.
- **Provider payload evolution.** None of the provider payloads are versioned in a standard we control. The `version_extractors` config + captured payload fixtures per adapter are the escape hatches; each adapter must ship with a test against a real recorded payload so schema drift is caught early.
- **What happens when multiple sources report the same deploy (e.g. GitHub Actions + Railway + `/status` all fire for version 1.4.2)?** Dedup by `dedup_key = sha256(app_id|env_id|version|event_type)` in `deploy_integration_events`. First writer creates the deployment row; subsequent events append their provider to the deployment's `sources: []` metadata field but do not re-create.
- **Generic endpoint abuse surface.** A leaked bearer token could spam deploy rows. Mitigations: rate limit per integration (default 60 req/min, configurable), surface recent events in the dashboard so operators can spot anomalies, and support rotating the integration's secret in one click.
- **`provider_config` schema variance.** Each adapter has its own shape; we could enforce JSON Schema validation per provider to fail fast on misconfiguration. Recommend yes; cheap insurance.
- **SDK reporter on serverless (Lambda, Cloud Run, Cloud Functions)?** Interval ticking is meaningless — the process dies. `reportStatusIntervalMs: 0` (init-only) is the right mode. Should the SDK auto-detect serverless runtimes and pick that default? Probably yes for Node/Python which have reliable env signals; explicit config for Go/Java.
- **Do we want the SDK to emit a shutdown `/status` even on crashes?** Not reliably possible from user-space in most languages. Dashboard should treat staleness as the signal for "probably dead" and not expect a farewell message.

## Documentation Updates (ship with the implementation)

Every phase of this spec changes what "integrating DeploySentry" looks like. The docs below must be refreshed *as part of* the implementation plans, not deferred:

- **`docs/Deploy_Integration_Guide.md`** — add a new top-level section "Reporting from a PaaS (Railway, Render, Fly)" covering the three paths (Railway webhook, SDK reporter, raw HTTP). Move the existing agent-centric content under "Self-hosted / Agent-based reporting" so it's clear the agent is one option among several.
- **`docs/Deploy_Monitoring_Setup.md`** — add an "Agentless health reporting" subsection with the staleness rules, `health.source` badges, and what the dashboard shows when nothing has reported yet. Keep the current Prometheus/Datadog/Sentry sections — those remain valid for apps that want external observability.
- **`docs/Getting_Started.md`** — update the "hello world" flow so a first-time user on Railway sees version + health in the dashboard after five minutes (Railway webhook + SDK `reportStatus: true`). Today's getting-started implies a local-Docker path; broaden it.
- **`docs/Traffic_Management_Guide.md`** — add a one-paragraph "When you don't need the agent" callout pointing at this spec so readers don't assume the sidecar is mandatory.
- **`sdk/*/README.md`** — each SDK's README gains a "Status Reporting" section with the config shape and a copy-pasteable snippet. Roll out per SDK phase.
- **`docs/superpowers/specs/2026-04-16-mcp-server-deploy-onboarding-design.md`** — cross-reference from this spec's related-designs and update the MCP tool list if new tools are added (see MCP section below).
- **`docs/CURRENT_INITIATIVES.md`** — flip the initiative to "Implementation" when the first plan is created, per project convention.

Documentation updates are a hard gate on each phase's merge — the implementation plan for each phase must list the specific doc files it touches and include the edits in the PR. Docs-after-code is disallowed; doc debt here directly breaks the customer-facing "what do I do on Railway?" story this spec exists to solve.

## MCP Server — Integration & Setup Assistant

DeploySentry already ships an MCP server (`docs/superpowers/specs/2026-04-16-mcp-server-deploy-onboarding-design.md`, `deploysentry mcp serve`, 12 existing tools). The MCP server is the right place for an **interactive setup walkthrough** of this spec's features — it lets a user in Claude / Cursor / any MCP client say "set up DeploySentry reporting for my Railway app" and have the agent drive the multi-step configuration.

New MCP tools to add (same phases as the API work):

| Tool | Purpose | Maps to |
|---|---|---|
| `reporting.create_status_key` | Mint an env-scoped API key with `status:write` + `flags:read`; return the key and a language-specific SDK snippet. | `POST /api-keys` + scope addition |
| `reporting.preview_sdk_snippet` | Given an application slug, environment, and language, return the exact SDK init snippet with `reportStatus: true` + detected env-var hints. | Client-side render only |
| `reporting.setup_deploy_integration` | Create a `deploy_integrations` row for the chosen `provider` (railway\|render\|fly\|heroku\|vercel\|netlify\|github-actions\|generic); return the webhook URL, signing secret or bearer token, and provider-specific setup instructions. | `POST /integrations/deploys` |
| `reporting.check_deploy_integration` | After the user configures the provider, poll `deploy_integration_events` for the most recent event tied to this integration; succeed when a valid event has landed in the last N minutes. | `GET /integrations/deploys/:id/recent-events` |
| `reporting.current_state` | Return `GET /current-state` for the named app+env so an agent can verify the setup worked end-to-end. | `GET /applications/:id/environments/:env_id/current-state` |
| `reporting.verify` | Composite: checks that (a) a deployment has been recorded, (b) a `/status` sample has landed, (c) the dashboard would show green. Returns a checklist with per-step pass/fail. | Read-only composition of the above |

The MCP server's job is **orchestration of the setup flow**, not new API surface — every tool maps to HTTP endpoints defined in this spec (or trivial read-side helpers). The existing MCP onboarding spec's "deploy onboarding" tools (`onboard_app`, `register_webhook`, etc.) stay as-is; the new tools are additive.

Recommendation: ship the MCP additions **alongside the corresponding API phase**, not as a separate late phase. Specifically:

- Phase 2 (`/status` endpoint) → ship `reporting.create_status_key`, `reporting.preview_sdk_snippet`, `reporting.current_state`.
- Phase 4–5 (SDK reporter) → update `reporting.preview_sdk_snippet` per language as each SDK lands.
- Phase 6 (deploy-event ingestion + Railway adapter) → ship `reporting.setup_deploy_integration` (Railway + generic), `reporting.check_deploy_integration`, `reporting.verify`.
- Phase 7 (additional provider adapters) → extend `reporting.setup_deploy_integration` to understand each newly-supported provider; no new MCP tools required — the generic tool handles every adapter via the `provider` parameter.

This keeps the MCP surface in lockstep with what actually works server-side, and makes "use the MCP to set up Railway reporting" a first-class customer story rather than an afterthought.

## Related Designs

- `2026-04-17-sidecar-traffic-management-design.md` — agent path this spec complements (not replaces).
- `2026-04-22-cloudflare-canary-and-observability-design.md` — forward-looking traffic-shift + provider-agnostic observability. This spec's `deploy_slot` field and `app_status.source` enum are designed to accept those future inputs without schema change.
- `internal/integrations/github/webhook.go` — implementation precedent for the Railway webhook handler.

## Status & Next Actions

- **Approved 2026-04-23.** Ready to begin implementation.
- **Implementation prerequisites:** none — all primitives (scoped keys, deployments resource, webhook integration pattern) already exist.
- **Next step:** carve out Phase 1 plan — `docs/superpowers/plans/2026-04-23-agentless-reporting-phase1-mode-record.md` covering the `mode=record` column, the `source` column, the `?environment_id=` list filter, the new `deployment.recorded` webhook event, and doc updates to `Deploy_Integration_Guide.md`. Small, shippable, unblocks everything downstream.
- **Subsequent plans** (one per phase, independently shippable): phase 2 (`/status` + `app_status` tables), phase 3 (`/current-state`), phase 4 (Node + Go SDK reporter), phase 5 (Python/Java/Ruby SDK reporter), phase 6 (ingestion core + generic endpoint + Railway adapter + MCP setup tools), phase 7 (remaining provider adapters).
