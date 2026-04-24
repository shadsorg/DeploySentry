# Deploy Integration Guide: GitHub Actions

**Phase**: Complete

> **Agent-based monitoring:** For traffic splitting with the DeploySentry agent sidecar, see the [Deploy Monitoring Setup](./Deploy_Monitoring_Setup.md) guide.

---

## Reporting deploys from a PaaS (Railway, Render, Fly, Heroku, …)

If your application is hosted on a PaaS that performs its own rollouts (Railway, Render, Fly, Heroku, Vercel, Netlify) you don't need to run a sidecar agent and you don't need DeploySentry to drive the deploy — you just want DeploySentry to **record** what shipped so history, version, and (later) health show up on the dashboard.

`POST /api/v1/deployments` supports an optional `mode` field for this:

| `mode` | Behavior |
|---|---|
| `orchestrate` (default) | DeploySentry drives the rollout through the phase engine. A `strategy` is required. Emits `deployment.created`. |
| `record` | The platform already deployed the artifact. The record is inserted with `status=completed`, `traffic_percent=100`, and timestamps set to now. The phase engine is bypassed. Emits `deployment.recorded`. `strategy` is optional. |

### Recording a platform-driven deploy via curl

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

`source` is a free-form audit trail — conventional values include `railway-webhook`, `render-webhook`, `github-actions`, `manual`. It's optional; when provided it's stored on the deployment row and included in the outbound `deployment.recorded` webhook payload.

### Listing deploy history per environment

```bash
# All deploys for the app
curl -H "Authorization: Bearer $DS_API_KEY" \
  "https://api.deploysentry.com/api/v1/deployments?app_id=$APP_ID"

# Scoped to a single environment (new)
curl -H "Authorization: Bearer $DS_API_KEY" \
  "https://api.deploysentry.com/api/v1/deployments?app_id=$APP_ID&environment_id=$ENV_ID&limit=50"
```

`limit` defaults to 20 and is capped at 100 server-side; `offset` is supported for pagination.

### Reporting live app status (version + health)

`POST /api/v1/applications/:app_id/status` accepts a self-reported status sample from the running app. Use an env-scoped API key with the `status:write` scope; the environment is inferred from the key.

```bash
curl -X POST https://api.deploysentry.com/api/v1/applications/$APP_ID/status \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.4.2",
    "commit_sha": "abc123",
    "health": "healthy",
    "health_score": 0.99
  }'
```

Fields:

- `version` (required) — what the app reports it's running.
- `commit_sha` — optional, recommended.
- `health` (required) — one of `healthy`, `degraded`, `unhealthy`, `unknown`.
- `health_score` — optional number in `[0, 1]`.
- `health_reason` — optional free-text note, shown on the dashboard when health is not `healthy`.
- `deploy_slot` — optional (`stable` / `canary`) — forward-compat with traffic-shift canaries.
- `tags` — optional string map (e.g. `{"region": "us-east"}`).

Behavior:

- Latest sample per `(application, environment)` is upserted into `app_status`.
- A retained sample is appended to `app_status_history` (used by later phases for sparkline/forensics).
- The **first** `/status` report with a version that DeploySentry has never seen for this `(app, environment)` auto-creates a `mode=record` deployment with `source="app-push"`. Subsequent reports of the same version are idempotent — they do not create duplicate deployments. This means: even if the Railway webhook path isn't wired, your deploy history still populates as soon as your app calls `/status` on startup.

Constraints:

- The API key **must** be scoped to the requested application (`api_key_app_id` matches `:app_id`) and to exactly one environment. Keys scoped to zero or multiple environments are rejected with `400` — ambiguity here would make the stored sample meaningless.
- Session-based (JWT) callers must pass `?environment_id=<uuid>` on the URL; this is primarily a testing convenience.

A recommended startup + periodic pattern for an SDK-less service:

```bash
# on boot, and every 30s
curl -sS -X POST "$DS_API/api/v1/applications/$APP_ID/status" \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"version\":\"$APP_VERSION\",\"health\":\"healthy\"}" \
  >/dev/null
```

### Wiring a PaaS deploy webhook

Every supported provider + the generic canonical endpoint funnel into the same internal `mode=record` plumbing — deploy rows are created idempotently from authenticated webhook deliveries.

**Two auth modes.** Every provider adapter accepts either:

- `auth_mode=hmac` (default) — provider's native signing header (`X-Railway-Signature`, `Render-Webhook-Signature`, `x-vercel-signature`, etc.) validated with a shared secret.
- `auth_mode=bearer` — `Authorization: Bearer <secret>` header on the inbound request. Use this when the provider no longer offers HMAC-signed webhooks in their public API but lets you set custom headers on the delivery (Railway's notification rules, for example).

Both modes are equivalent in security terms (both rely on a shared secret over TLS); pick whichever the provider's outbound webhook configuration supports.

**1. Create a deploy integration**

Easiest path — the CLI, which accepts slugs on every flag and prints the webhook URL inline:

```bash
# Generate a random HMAC signing secret; store it in your password manager
# AND paste it into the provider dashboard in step 2.
WEBHOOK_SECRET=$(openssl rand -hex 32)

deploysentry integrations deploy create \
  --app api-server --provider railway \
  --webhook-secret "$WEBHOOK_SECRET" \
  --provider-config '{"service_id":"svc-abc-123"}' \
  --env-mapping production=prod,staging=stg
```

Output:
```
Deploy integration created.
  ID:           f47ac10b-58cc-4372-a567-0e02b2c3d479
  Webhook URL:  https://api.deploysentry.com/api/v1/integrations/railway/webhook
```

Raw API equivalent (for CI pipelines that don't install the CLI):

```bash
curl -X POST https://api.deploysentry.com/api/v1/integrations/deploys \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "application_id": "…",
    "provider": "railway",
    "auth_mode": "hmac",
    "webhook_secret": "…",
    "provider_config": { "service_id": "…" },
    "env_mapping": { "production": "…", "staging": "…" }
  }'
```

The `webhook_secret` is AES-256-encrypted at rest using `DS_SECURITY_ENCRYPTION_KEY` and is never returned on subsequent reads — store it yourself before the POST.

**2. Wire the provider webhook**

- **Railway**: Railway has retired the service-level HMAC-signed webhook API and replaced it with **workspace-level notification rules** that deliver *unsigned* POSTs but let you set custom HTTP headers on the rule. Use `auth_mode=bearer` when creating the integration:
  ```bash
  deploysentry integrations deploy create --app api-server --provider railway \
    --auth-mode bearer \
    --webhook-secret "$(openssl rand -hex 32)" \
    --provider-config '{"service_id":"<RAILWAY_SERVICE_ID>"}' \
    --env-mapping production=prod,staging=stg
  ```
  Then in Railway: Project Settings → Notification Rules → add a rule on "Deployment Succeeded/Failed/Crashed" events, target URL = `https://api.deploysentry.com/api/v1/integrations/railway/webhook`, add a custom header `Authorization: Bearer <the webhook_secret you generated>`. Match key: `provider_config.service_id`.

  Legacy HMAC path (only if you still have a working service-level webhook from before Railway's deprecation): `auth_mode=hmac` with signing header `X-Railway-Signature: sha256=<hex>`. This path is kept for compatibility but Railway no longer lets you create new HMAC-signed webhooks via their GraphQL or dashboard.
- **Render**: service → Settings → Webhooks. Signing header: `Render-Webhook-Signature: t=<ts>,v1=<hex>` (HMAC over `ts + "." + body`). Match key: `provider_config.service_id`.
- **Fly.io**: POST the Fly-shaped payload from your deploy pipeline. Signing header: `X-Fly-Signature: sha256=<hex>`. Match key: `provider_config.app_name`.
- **Heroku**: `heroku webhooks:add -u <URL> -i api:release --secret <secret>`. Signing header: `Heroku-Webhook-Hmac-SHA256: <base64>`. Match key: `provider_config.app_name`.
- **Vercel**: project → Settings → Webhooks. Signing header: `x-vercel-signature: <hex>` (no prefix). Match key: `provider_config.project_id`.
- **Netlify**: site → Build & deploy → Deploy notifications → Outgoing webhook (HMAC signed). Signing header: `x-webhook-signature: sha256=<hex>`. Match key: `provider_config.site_id`.
- **GitHub Actions**: repo/org settings → Webhooks, content type `application/json`, subscribe to `workflow_run`. Signing header: `X-Hub-Signature-256: sha256=<hex>`. Match key: `provider_config.repository` (e.g. `"acme/api"`); optional `provider_config.workflow_name` filter to isolate the deploy workflow from CI-only runs; optional `provider_config.environment` pins deliveries to a specific env-mapping key.
- **Generic / CI**: point your CI job at `https://api.deploysentry.com/api/v1/integrations/deploys/webhook` and send the canonical `DeployEvent` payload plus `X-DeploySentry-Integration-Id: <id>`. Use `Authorization: Bearer <webhook_secret>` if `auth_mode=bearer`, or an HMAC signature in `X-DeploySentry-Signature: sha256=<hex>` if `auth_mode=hmac`.

Example generic payload from a GitHub Actions deploy job:

```bash
curl -X POST "$DS_API/api/v1/integrations/deploys/webhook" \
  -H "Authorization: Bearer $DS_INTEGRATION_TOKEN" \
  -H "X-DeploySentry-Integration-Id: $DS_INTEGRATION_ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"event_type\": \"deploy.succeeded\",
    \"environment\": \"production\",
    \"version\": \"${GITHUB_SHA}\",
    \"commit_sha\": \"${GITHUB_SHA}\"
  }"
```

**3. Verify deliveries**

```bash
curl -H "Authorization: Bearer $DS_API_KEY" \
  "https://api.deploysentry.com/api/v1/integrations/deploys/$DS_INTEGRATION_ID/events?limit=10"
```

Behavior:

- A successful delivery with `event_type=deploy.succeeded` auto-creates a `mode=record` deployment row (`source="railway-webhook"` or `"generic-webhook"`) and records the event for dedup.
- Duplicate deliveries (same app + env + version + event_type) are idempotent — they return the existing deployment without re-creating anything.
- `deploy.failed` / `deploy.crashed` events are recorded but do not create a deployment row.
- Unknown environment names (not in `env_mapping`) respond with `202 Accepted` and the payload is stored but no deployment is created — fail closed, not silent.

### Streaming GitHub Actions build/test status to the Status board

The deploy-event webhook above records a deploy **once it has succeeded**.
To also show CI build/test progress (pending → running → completed) as a
pill on the Org Status board, subscribe GitHub to DeploySentry's dedicated
`workflow_run` endpoint. Each workflow on each commit appears as its own
lane, so build / test / e2e runs don't collide with each other.

**1. Mint a scoped API key** with `status:write` scope bound to one
application and one environment. The same key style works for both SDK
health pushes and this webhook — you can reuse an existing key if its
scope matches.

**2. Add a repo webhook** at GitHub repo settings → Webhooks → Add webhook:

- Payload URL: `https://api.deploysentry.com/api/v1/applications/<APP_ID>/integrations/github/workflow`
- Content type: `application/json`
- Secret: leave blank unless you want HMAC (see below).
- SSL verification: enabled.
- Which events: "Let me select individual events" → check only **Workflow runs**.

Under the webhook's "Additional HTTP headers" (via GitHub's API or a proxy
if your repo UI doesn't expose it), add:

```
Authorization: Bearer <ds_...>
```

If your org pins webhook delivery through a gateway that strips
Authorization, store the bearer in the webhook's Secret field and terminate
at a small forwarding layer — or move the integration to a GitHub App,
which supports custom headers natively.

**3. Verify deliveries** from GitHub's "Recent Deliveries" tab and on the
DeploySentry side:

```bash
curl -H "Authorization: Bearer $DS_API_KEY" \
  "https://api.deploysentry.com/api/v1/applications/$APP_ID/versions?limit=5"
```

Each run appears as a record-mode `deployments` row with
`source="github-actions:<workflow-name>"`. The Org Status board renders it
as a small pill next to the env chip — `⏱ build` while running, `✓ build`
on success, `✗ build` on failure (click-through to the run's GitHub page).

Behavior:

- Upsert key is `(application_id, environment_id, commit_sha, workflow_name)`.
  Re-runs of the same workflow update the same row; build + test + e2e
  workflows run alongside each other as separate lanes.
- `requested` / `in_progress` events set `status=running`.
- `completed` events set `status=completed` / `failed` / `cancelled` per
  the run's conclusion.
- `ping` events return 200 immediately so GitHub shows the webhook as
  healthy without creating a row.
- Optional HMAC: if the API key has a `github_secret` stored in metadata,
  the endpoint verifies `X-Hub-Signature-256`. Bearer auth is required
  regardless.

### Using a first-party SDK

If you're already using one of the DeploySentry SDKs for feature flags, enabling status reporting is a one-line config change — no new dependency, no handwritten HTTP call. All server SDKs share the same contract: set `reportStatus` / `report_status` / `WithReportStatus(true)`, pass the application UUID, and optionally supply a health provider.

- Node / TypeScript — [`sdk/node/README.md`](../sdk/node/README.md#status-reporting-optional)
- Go — [`sdk/go/README.md`](../sdk/go/README.md#status-reporting)
- Python — [`sdk/python/README.md`](../sdk/python/README.md#status-reporting-optional)
- Java — [`sdk/java/README.md`](../sdk/java/README.md#status-reporting)
- Ruby — [`sdk/ruby/README.md`](../sdk/ruby/README.md#status-reporting-optional)

See [`docs/archives/2026-04-23-agentless-deploy-reporting-design.md`](./archives/2026-04-23-agentless-deploy-reporting-design.md) for the full agentless reporting design + completion record.

### Viewing current state for an environment

One read assembles everything the dashboard needs: current deployment, health, and recent history.

```bash
curl -H "Authorization: Bearer $DS_API_KEY" \
  "https://api.deploysentry.com/api/v1/applications/$APP_ID/environments/$ENV_ID/current-state?limit=10"
```

Response shape:

```jsonc
{
  "environment":  { "id": "…", "slug": "production", "name": "Production" },
  "current_deployment": {
    "id": "…",
    "version": "1.4.2",
    "commit_sha": "abc123",
    "status": "completed",
    "mode": "record",
    "source": "app-push",
    "traffic_percent": 100,
    "started_at":   "…",
    "completed_at": "…"
  },
  "health": {
    "state": "healthy",
    "score": 0.99,
    "source": "app-push",       // app-push | agent | observability | unknown
    "last_reported_at": "…",
    "staleness": "fresh"        // fresh (<60s) | stale (<5m) | missing
  },
  "recent_deployments": [
    { "id": "…", "version": "1.4.2", "status": "completed", "mode": "record", "completed_at": "…" },
    …
  ],
  "active_rollout": null        // populated by a future phase when a rollout is in flight
}
```

Notes:

- `limit` on the recent list defaults to 10 and is capped at 50 server-side.
- When no status has ever been reported the `health` block returns `state="unknown"`, `source="unknown"`, `staleness="missing"`.
- When the most recent deploy is already terminal (e.g. `completed`), `current_deployment` falls back to that row rather than returning `null` — this matches operator intuition ("what's running right now").

---

## Overview

This guide covers how to connect a GitHub repository to DeploySentry so that deployments are automatically created, monitored, and (optionally) rolled back when you push or release code. It covers setup on three sides: DeploySentry, GitHub, and your repository.

> **Traffic splitting:** For controlling traffic between versions with Envoy and the DeploySentry agent sidecar, see the [Traffic Management Guide](./Traffic_Management_Guide.md).

---

## How It Works

```
 Your Repository                  GitHub                      DeploySentry
 ┌─────────────┐          ┌──────────────────┐         ┌──────────────────────┐
 │ git push     │────────► │ GitHub Actions   │         │                      │
 │ main branch  │          │ builds + deploys │         │   dr-sentry.com      │
 └─────────────┘          │ to your infra    │         │                      │
                          └────────┬─────────┘         │  ┌────────────────┐  │
                                   │                    │  │ Phase Engine   │  │
                                   │ POST /deployments  │  │ (drives canary │  │
                                   ├───────────────────►│  │  phases auto-  │  │
                                   │                    │  │  matically)    │  │
                                   │                    │  └───────┬────────┘  │
                                   │                    │          │           │
                          ┌────────┴─────────┐         │  ┌───────▼────────┐  │
                          │ GitHub Webhook   │◄────────│  │ Health Monitor │  │
                          │ (status checks)  │         │  │ (checks your   │  │
                          └──────────────────┘         │  │  app health)   │  │
                                                       │  └───────┬────────┘  │
                                                       │          │           │
                                                       │  ┌───────▼────────┐  │
                                                       │  │ Rollback Ctrl  │  │
                                                       │  │ (auto-rollback │  │
                                                       │  │  if unhealthy) │  │
                                                       │  └────────────────┘  │
                                                       │                      │
                                                       │  Notifications ──►   │
                                                       │  Slack / Email /     │
                                                       │  PagerDuty / Webhook │
                                                       └──────────────────────┘
```

**The deployment model**: DeploySentry does not perform the actual deployment to your infrastructure — your CI/CD pipeline (GitHub Actions) does that. DeploySentry tracks the deployment lifecycle: it records what was deployed, monitors its health, drives canary traffic phases, and triggers rollbacks if something goes wrong.

Think of it as a deployment control plane sitting alongside your existing CI/CD.

---

## Quickstart: LLM-Assisted Setup (Recommended)

If you're using Claude Code or another LLM tool with MCP support, you can set up deployment tracking in one conversation instead of following the manual steps below.

### One-time: Install CLI, authenticate, add the MCP server

```bash
# 1. Install the CLI (skip if already installed)
curl -fsSL https://api.dr-sentry.com/install.sh | sh

# 2. Create an API key in the dashboard (Org → API Keys) and pass it
#    with --token. Do NOT run `deploysentry auth login` bare — the
#    interactive prompt blocks non-terminal sessions (LLM agents, CI,
#    etc.) and pre-2026-04-23 binaries open a browser to a stale page.
deploysentry auth login --token ds_live_xxxxxxxxxxxx
# — or equivalently —
export DEPLOYSENTRY_API_KEY=ds_live_xxxxxxxxxxxx
deploysentry auth login

# 3. Add the MCP server to Claude Code. It reads the same credentials
#    file the CLI just wrote, so no separate auth ceremony is needed.
claude mcp add deploysentry -- deploysentry mcp serve
```

> **Troubleshooting.** The CLI uses API keys, not browser OAuth. If
> `auth login` opens a browser that 404s, your binary is stale — rebuild
> with `go install github.com/deploysentry/deploysentry/cmd/cli@main`,
> or skip `auth login` entirely and just `export DEPLOYSENTRY_API_KEY=…`
> (every other CLI command and the MCP server fall back to that env
> var). `docs/Getting_Started.md` is canonical.

### Then just ask

Open Claude Code in your repository and say:

> "Set up deployment tracking for this repo with DeploySentry"

The LLM will use the MCP tools to:
1. Discover your org, project, app, and environments
2. Create an environment-scoped API key
3. Set the GitHub secrets via `gh` CLI
4. Generate a `.github/workflows/deploy.yml` step
5. Summarize what was set up

If something is missing (CLI not authenticated, `gh` not installed), the tools return actionable instructions.

### Available MCP Tools

| Tool | What it does |
|------|-------------|
| `ds_status` | Check CLI auth and config, show issues |
| `ds_list_orgs` / `ds_list_projects` / `ds_list_apps` / `ds_list_environments` | Discover your DeploySentry resources |
| `ds_create_api_key` | Create an API key (optionally env-scoped) |
| `ds_get_app_deploy_status` | Check if an app already has deployments |
| `ds_generate_workflow` | Generate a GitHub Actions YAML step |
| `ds_list_flags` / `ds_get_flag` / `ds_create_flag` / `ds_toggle_flag` | Manage feature flags |

If you prefer manual setup, continue below.

---

## Prerequisites

Before starting, you need:

- A DeploySentry account at dr-sentry.com
- An organization, project, and application set up in DeploySentry
- At least one environment configured (e.g., `production`, `staging`)
- A GitHub repository with Actions enabled

---

## Step 1: DeploySentry Setup

### 1.1 Create an API Key

In the DeploySentry dashboard:

1. Navigate to your project
2. Go to **Settings > API Keys**
3. Click **Create API Key**
4. Give it a name like `github-actions-deploy`
5. Select scopes: `flags:read`, `deploys:read`, `deploys:write`
6. Copy the key (starts with `ds_live_` or `ds_test_`)

Save this key — you'll add it to GitHub secrets in Step 2.

### 1.2 Note Your IDs

You'll need these values for the GitHub Action. Find them in the dashboard or via the API:

| Value | Where to find it |
|-------|-----------------|
| Application ID | Dashboard > Project > Application, or `GET /api/v1/orgs/:org/projects/:project/apps/:app` |
| Environment ID | Dashboard > Org Settings > Environments, or `GET /api/v1/orgs/:org/environments` |

### 1.3 Configure a Webhook (Optional)

If you want DeploySentry to post deployment status back to a Slack channel or other service:

1. Go to **Org Settings > Webhooks**
2. Click **Create Webhook**
3. Enter the target URL
4. Select events: `deployment.created`, `deployment.completed`, `deployment.failed`, `deployment.rolled_back`
5. Save — DeploySentry will deliver signed payloads to this URL on each event

### 1.4 Configure Health Monitoring (Optional)

If you want automated rollbacks based on health signals:

1. Go to **Project Settings**
2. Configure a health check integration (Prometheus, Datadog, Sentry, or a custom HTTP endpoint)
3. Set the health threshold (default: 95% — deployments roll back if health drops below this)

---

## Step 2: GitHub Setup

### 2.1 Add Repository Secrets

In your GitHub repository:

1. Go to **Settings > Secrets and variables > Actions**
2. Add these secrets:

| Secret Name | Value |
|-------------|-------|
| `DS_API_KEY` | Your DeploySentry API key (`ds_live_...`) |
| `DS_APP_ID` | Your DeploySentry Application ID (UUID) |
| `DS_ENV_ID` | Your DeploySentry Environment ID (UUID) |

Optionally:

| Secret Name | Value | Default |
|-------------|-------|---------|
| `DS_API_URL` | DeploySentry API URL | `https://dr-sentry.com` |
| `DS_STRATEGY` | Deployment strategy | `rolling` |

### 2.2 Choose Your Integration Pattern

There are two ways to connect GitHub to DeploySentry:

**Pattern A: GitHub Action step (recommended)** — Your workflow calls the DeploySentry API after your deployment step completes. You control exactly when the deployment is recorded.

**Pattern B: GitHub Webhook (automatic)** — DeploySentry listens for push/release events from GitHub and creates deployments automatically. Less configuration in the repo, but less control over timing.

Most teams use **Pattern A** because it lets you record the deployment at the right moment — after the build succeeds and the deploy to infrastructure completes.

---

## Step 3: Repository Setup

### Pattern A: GitHub Action Step

Add a deployment notification step to your existing workflow. This goes **after** your build and deploy steps.

#### Basic Setup (Rolling Deploy)

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # ── Your existing build + deploy steps ──
      - name: Build
        run: |
          # your build commands here
          docker build -t myapp:${{ github.sha }} .

      - name: Deploy to infrastructure
        run: |
          # your deploy commands here (kubectl, aws, etc.)
          kubectl set image deployment/myapp myapp=myapp:${{ github.sha }}

      # ── Record the deployment in DeploySentry ──
      - name: Record deployment
        if: success()
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://dr-sentry.com' }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          DEPLOY_RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"rolling\",
              \"artifact\": \"${{ github.repository }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\",
              \"description\": \"Deployed from GitHub Actions (${GITHUB_REF_NAME})\"
            }")

          DEPLOY_ID=$(echo "$DEPLOY_RESPONSE" | jq -r '.deployment.id // .id')
          echo "DEPLOY_ID=${DEPLOY_ID}" >> $GITHUB_ENV
          echo "Deployment created: ${DEPLOY_ID}"
```

#### Canary Deploy with Monitoring

For canary deployments, DeploySentry drives the traffic phases automatically. Your workflow creates the deployment, and the Phase Engine takes over:

```yaml
# .github/workflows/deploy-canary.yml
name: Canary Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build and push image
        run: |
          docker build -t myapp:${{ github.sha }} .
          docker push myapp:${{ github.sha }}

      # Deploy the canary instance (1% traffic initially)
      - name: Deploy canary
        run: |
          kubectl apply -f k8s/canary.yaml

      # Record in DeploySentry — Phase Engine takes over from here
      - name: Create canary deployment
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://dr-sentry.com' }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          DEPLOY_RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"canary\",
              \"artifact\": \"myapp:${{ github.sha }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\"
            }")

          DEPLOY_ID=$(echo "$DEPLOY_RESPONSE" | jq -r '.deployment.id // .id')
          echo "Canary deployment created: ${DEPLOY_ID}"
          echo "DeploySentry will drive phases: 1% → 5% → 25% → 50% → 100%"
          echo "Monitor at: ${DS_API_URL}/deployments/${DEPLOY_ID}"
```

After creation, DeploySentry's Phase Engine automatically:
1. Transitions the deployment to `running`
2. Drives through canary phases (1% → 5% → 25% → 50% → 100%)
3. Checks health at each phase boundary
4. Rolls back if health drops below threshold
5. Completes the deployment when all phases pass

#### Wait for Deployment Completion (Optional)

If you want your workflow to wait for DeploySentry to finish monitoring:

```yaml
      - name: Wait for deployment completion
        timeout-minutes: 30
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://dr-sentry.com' }}
        run: |
          echo "Waiting for deployment ${DEPLOY_ID} to complete..."
          while true; do
            STATUS=$(curl -sf "${DS_API_URL}/api/v1/deployments/${DEPLOY_ID}" \
              -H "Authorization: ApiKey ${DS_API_KEY}" \
              | jq -r '.deployment.status // .status')

            echo "Status: ${STATUS}"

            case "$STATUS" in
              completed)
                echo "Deployment completed successfully"
                exit 0
                ;;
              failed|rolled_back|cancelled)
                echo "Deployment ${STATUS}"
                exit 1
                ;;
            esac

            sleep 30
          done
```

#### With the DeploySentry CLI

If you prefer the CLI over raw API calls:

```yaml
      - name: Install DeploySentry CLI
        run: curl -fsSL https://api.dr-sentry.com/install.sh | sh

      - name: Create deployment
        env:
          DEPLOYSENTRY_API_KEY: ${{ secrets.DS_API_KEY }}
          DEPLOYSENTRY_URL: ${{ secrets.DS_API_URL || 'https://dr-sentry.com' }}
        run: |
          deploysentry deploy create \
            --release "${{ github.sha }}" \
            --env production \
            --strategy canary \
            --description "Deploy from ${{ github.ref_name }}"

      - name: Watch deployment
        env:
          DEPLOYSENTRY_API_KEY: ${{ secrets.DS_API_KEY }}
        run: |
          deploysentry deploy status --watch
```

---

### Pattern B: GitHub Webhook (Automatic)

With this pattern, GitHub sends push/release events directly to DeploySentry, which creates deployments automatically.

#### Setup on DeploySentry

1. Go to **Org Settings** in the dashboard
2. Navigate to the GitHub integration section
3. Configure:
   - **Webhook Secret**: Generate a strong secret (you'll add this to GitHub)
   - **Default Strategy**: `rolling`, `canary`, or `blue_green`
   - **Deploy Branches**: `main`, `master` (or your deploy branch)
   - **Auto Deploy**: Enable to create deployments on every push

#### Setup on GitHub

1. Go to your repository **Settings > Webhooks**
2. Click **Add webhook**
3. Configure:
   - **Payload URL**: `https://dr-sentry.com/api/v1/integrations/github/webhook`
   - **Content type**: `application/json`
   - **Secret**: The webhook secret from DeploySentry
   - **Events**: Select "Pushes" and "Releases"

#### How It Works

When you push to a configured branch:

1. GitHub sends a `push` event to DeploySentry
2. DeploySentry verifies the HMAC-SHA256 signature
3. A deployment is created with:
   - **Version**: The commit SHA
   - **Artifact**: The repository full name (e.g., `org/repo`)
   - **Strategy**: Your configured default
4. The Phase Engine takes over (for canary deployments)

When you publish a release:

1. GitHub sends a `release` event
2. DeploySentry creates a deployment with the release tag as the version
3. Pre-releases are handled based on your Auto Deploy setting

---

## Deployment Strategies

### Rolling (Default)

Replaces instances in batches. Simplest strategy — good for most services.

```
Phase 1: Replace batch 1 (1 of 3 instances) ──► health check ──► continue
Phase 2: Replace batch 2 (2 of 3 instances) ──► health check ──► continue
Phase 3: Replace batch 3 (3 of 3 instances) ──► health check ──► complete
```

Configuration defaults: batch size 1, 30s delay between batches, max 1 unavailable.

### Canary

Routes a small percentage of traffic to the new version, increasing gradually. Best for critical services where you need confidence before full rollout.

```
Phase 1:   1% traffic for  5 min ──► health check ──► auto-promote
Phase 2:   5% traffic for  5 min ──► health check ──► auto-promote
Phase 3:  25% traffic for 10 min ──► health check ──► auto-promote
Phase 4:  50% traffic for 10 min ──► health check ──► auto-promote
Phase 5: 100% traffic             ──► complete
```

If health drops below 95% at any phase, the deployment is automatically rolled back to 0% traffic.

Phases can also be manual gates — the Phase Engine pauses and waits for you to advance via the dashboard or API (`POST /deployments/:id/advance`).

### Blue-Green

Deploys to a standby environment, verifies health, then switches all traffic at once. Best for zero-downtime requirements.

```
Deploy to green ──► warmup (2 min) ──► health check ──► switch all traffic ──► complete
                                            │
                                            └──► unhealthy ──► rollback (switch back to blue)
```

---

## Monitoring & Automated Rollback

### How Health Monitoring Works

Once a deployment is created, DeploySentry's health monitor periodically checks your application:

1. **Health checks run** on a configurable interval (default: every 30 seconds)
2. **Multiple sources** can feed health data: Prometheus, Datadog, Sentry, or a custom HTTP endpoint
3. **Scores are aggregated** into a single 0.0–1.0 health score
4. The **Rollback Controller** watches this score

### Automatic Rollback Triggers

| Trigger | Default Threshold | Evaluation Window |
|---------|-------------------|-------------------|
| Error rate | > 5% | 2 minutes sustained |
| P99 latency | > 2 seconds | 2 minutes sustained |
| Health score | < 95% | 2 minutes sustained |

If any trigger fires for the full evaluation window, the rollback controller:

1. Transitions the deployment to `rolled_back`
2. Sets traffic to 0% on the new version
3. Creates a `RollbackRecord` with `automatic: true`
4. Publishes a `deployment.rolled_back` event
5. Notifies all configured channels (Slack, email, PagerDuty)
6. Enters a 5-minute cooldown before allowing another rollback

### Manual Rollback

From the dashboard, CLI, or API:

```bash
# CLI
deploysentry deploy rollback <deployment-id> --reason "Elevated error rates"

# API
curl -X POST https://dr-sentry.com/api/v1/deployments/<id>/rollback \
  -H "Authorization: ApiKey <key>" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Elevated error rates"}'
```

---

## Deployment Lifecycle Events

DeploySentry publishes events at each state transition. These drive notifications, webhooks, and the dashboard UI.

| Event | When |
|-------|------|
| `deployment.created` | Deployment recorded |
| `deployment.started` | Phase Engine begins driving phases |
| `deployment.phase.completed` | A canary phase passed health checks |
| `deployment.completed` | All phases passed, deployment is live |
| `deployment.paused` | User paused the deployment |
| `deployment.resumed` | User resumed a paused deployment |
| `deployment.failed` | Deployment encountered an error |
| `deployment.rolled_back` | Automatic or manual rollback triggered |
| `deployment.cancelled` | User cancelled a pending deployment |
| `health.degraded` | Health score dropped below threshold |
| `health.alert.triggered` | Health alert fired |
| `health.alert.resolved` | Health recovered |

---

## Notifications

Configure notification channels in **Org Settings > Notifications**:

| Channel | Setup |
|---------|-------|
| **Slack** | Provide a Slack webhook URL. Events post as formatted messages with deployment details. |
| **Email** | Configure SMTP or provide email addresses. Sends on deployment completion, failure, and rollback. |
| **PagerDuty** | Provide a PagerDuty routing key. Creates incidents on rollback and health degradation. |
| **Webhooks** | Provide any HTTP endpoint. Payloads are signed with HMAC-SHA256 (`X-DeploySentry-Signature` header). |

---

## API Quick Reference

All endpoints require `Authorization: ApiKey <key>` header.

### Create a Deployment

```
POST /api/v1/deployments
```

```json
{
  "application_id": "uuid",
  "environment_id": "uuid",
  "strategy": "rolling | canary | blue_green",
  "artifact": "docker-image:tag or repo name",
  "version": "v1.2.3 or commit SHA",
  "commit_sha": "full 40-char SHA",
  "description": "optional human-readable note"
}
```

### Check Deployment Status

```
GET /api/v1/deployments/:id
```

Returns the full deployment object including `status`, `traffic_percent`, `started_at`, `completed_at`.

### List Deployments

```
GET /api/v1/deployments?app_id=<uuid>&limit=20
```

### Promote / Advance

```
POST /api/v1/deployments/:id/promote    # Promote to 100% traffic
POST /api/v1/deployments/:id/advance    # Advance past a manual canary gate
```

### Pause / Resume

```
POST /api/v1/deployments/:id/pause
POST /api/v1/deployments/:id/resume
```

### Rollback

```
POST /api/v1/deployments/:id/rollback
{"reason": "optional reason string"}
```

### Desired State (for controllers/agents)

```
GET /api/v1/deployments/:id/desired-state
GET /api/v1/applications/:app_id/desired-state
```

Returns what should be running — artifact, version, traffic percent, current phase. Useful for custom controllers that reconcile actual vs. desired state.

---

## Complete Example: Full Workflow

Here's a complete `.github/workflows/deploy.yml` that builds, deploys, records the deployment, and waits for DeploySentry to confirm health:

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]

env:
  DS_API_URL: https://dr-sentry.com

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image_tag: ${{ steps.build.outputs.tag }}
    steps:
      - uses: actions/checkout@v4

      - name: Build and push Docker image
        id: build
        run: |
          TAG="${{ github.sha }}"
          docker build -t myapp:${TAG} .
          docker push myapp:${TAG}
          echo "tag=${TAG}" >> $GITHUB_OUTPUT

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/myapp \
            myapp=myapp:${{ needs.build.outputs.image_tag }}
          kubectl rollout status deployment/myapp --timeout=120s

      - name: Record deployment in DeploySentry
        id: ds_deploy
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"canary\",
              \"artifact\": \"myapp:${{ needs.build.outputs.image_tag }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\",
              \"description\": \"${{ github.event.head_commit.message }}\"
            }")

          DEPLOY_ID=$(echo "$RESPONSE" | jq -r '.deployment.id // .id')
          echo "deploy_id=${DEPLOY_ID}" >> $GITHUB_OUTPUT
          echo "::notice::Deployment ${DEPLOY_ID} created. Canary phases starting."

      - name: Wait for canary to complete
        timeout-minutes: 45
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DEPLOY_ID: ${{ steps.ds_deploy.outputs.deploy_id }}
        run: |
          while true; do
            RESULT=$(curl -sf "${DS_API_URL}/api/v1/deployments/${DEPLOY_ID}" \
              -H "Authorization: ApiKey ${DS_API_KEY}")

            STATUS=$(echo "$RESULT" | jq -r '.deployment.status // .status')
            TRAFFIC=$(echo "$RESULT" | jq -r '.deployment.traffic_percent // .traffic_percent')

            echo "$(date '+%H:%M:%S') Status: ${STATUS} | Traffic: ${TRAFFIC}%"

            case "$STATUS" in
              completed)
                echo "::notice::Deployment completed successfully at 100% traffic"
                exit 0
                ;;
              failed|rolled_back|cancelled)
                echo "::error::Deployment ${STATUS}"
                exit 1
                ;;
            esac

            sleep 30
          done
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `401 Unauthorized` | Invalid or missing API key | Check `DS_API_KEY` secret. Key must have `deploys:write` scope. |
| `404 Not Found` on create | Invalid application or environment ID | Verify `DS_APP_ID` and `DS_ENV_ID` match UUIDs in the dashboard |
| Deployment stuck in `pending` | Phase Engine not running or NATS disconnected | Check DeploySentry server health at `/health` |
| No rollback despite errors | Health monitoring not configured | Set up a health check integration in project settings |
| Webhook not received | Signature mismatch | Verify the webhook secret matches between GitHub and DeploySentry |
| `409 Conflict` | Active deployment already exists for this app+env | Wait for the current deployment to complete, or cancel it first |

## Checklist

- [x] Integration architecture diagram
- [x] DeploySentry setup (API key, IDs, webhooks, health monitoring)
- [x] GitHub setup (secrets, webhook)
- [x] Pattern A: GitHub Action step with full workflow examples
- [x] Pattern B: GitHub Webhook automatic mode
- [x] All three deployment strategies explained
- [x] Health monitoring and automated rollback model
- [x] Event lifecycle reference
- [x] Notification channels
- [x] Full API quick reference
- [x] Complete end-to-end workflow example
- [x] Troubleshooting table

## Completion Record

- **Branch**: `feature/groups-and-resource-authorization`
- **Committed**: No (pending review)
- **Pushed**: No
- **CI Checks**: N/A (documentation only)
