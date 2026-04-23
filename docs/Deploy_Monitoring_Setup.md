# Deploy Monitoring Setup

This guide walks you through standing up DeploySentry deploy monitoring for an application. It is split into two tracks:

- **Developers (sections 1–4)** — get your first app monitored end-to-end.
- **Ops engineers (sections 5–8)** — set up a team-wide, multi-environment rollout.
- **Section 9** covers LLM-assisted setup via the MCP server and prompt templates.

Related docs:
- [Traffic_Management_Guide.md](./Traffic_Management_Guide.md) — canary, blue/green, and traffic shifting strategies.
- [Deploy_Integration_Guide.md](./Deploy_Integration_Guide.md) — SDK and API integration patterns.
- [Bootstrap_My_App.md](./Bootstrap_My_App.md) — first-app quickstart.

---

## 1. Prerequisites

Before starting you need:

- A DeploySentry account with an organization you belong to.
- A **project** and **application** created inside that org.
- The `deploysentry` CLI installed (`curl -fsSL https://api.dr-sentry.com/install.sh | sh`) and authenticated (`deploysentry auth login`).

Create a project and app via the dashboard (`Settings → Projects → Create`) or via the CLI:

```bash
deploysentry orgs list
deploysentry projects create --name "My Project" --slug my-project
deploysentry apps create --name "API Server" --slug api-server
```

Confirm the app exists:

```bash
deploysentry apps list --project my-project
```

---

## 2. Create a Scoped API Key

The agent talks to the DeploySentry API with an API key. Use the narrowest scope the agent needs — an application-scoped, environment-restricted key is the recommended default.

### Dashboard

1. Navigate to **Settings → API Keys**.
2. Click **Create API Key**.
3. **Name**: `agent-prod-api-server`.
4. **Scopes**: check `flags:read`, `deploys:read`, `deploys:write`.
5. **Project Scope**: select your project from the cascading dropdown.
6. **Application Scope**: select your app. The app selector only appears after a project is chosen.
7. **Environment Restrictions**: select `production`.
8. Click **Create**, then **copy the key immediately — it is shown once**.

### CLI equivalent

```bash
deploysentry apikeys create --name "agent-prod-api-server" \
  --scopes "flags:read,deploys:read,deploys:write" \
  --project my-project --app api-server --env production
```

### Why app- and env-scoped keys matter

When the key is bound to a specific application and environment, the agent doesn't need to be told which app/env it represents — the server infers it from the key. That means:

- No manual `application_id` UUID plumbing in config.
- A key leaked from a dev agent cannot be used to push production traffic.
- Rotating keys per environment is isolated from the others.

---

## 3. Configure the Agent

The agent only requires two environment variables in the common case:

```bash
export DS_API_KEY=ds_xxxxxxxx
export DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082
```

`DS_UPSTREAMS` is a comma-separated list of `name:host:port` entries describing the upstream versions the agent can route between.

### Platform-specific start

**Docker Compose (local dev):**

```bash
make dev-deploy
```

This brings up the agent, Envoy, and two upstream containers wired to the compose network.

**Render:**

- Create a **Web Service** from your repo.
- Set env vars `DS_API_KEY` and `DS_UPSTREAMS` in the Render dashboard (mark `DS_API_KEY` as secret / no-sync).
- Start command: `/usr/local/bin/deploysentry-agent`.

**Railway:**

- Push the agent image or use the `Dockerfile.agent`.
- Set env vars via `railway variables set DS_API_KEY=ds_xxx` and `railway variables set DS_UPSTREAMS=...`.
- Start command: `deploysentry-agent`.

### Env var reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DS_API_KEY` | yes | — | Scoped API key created in section 2. |
| `DS_UPSTREAMS` | yes | — | `name:host:port` list of upstream versions. |
| `DS_API_URL` | no | `https://api.dr-sentry.com` | DeploySentry control plane URL. |
| `DS_ENVOY_XDS_PORT` | no | `18000` | xDS port Envoy connects to. Must match `envoy-bootstrap.yaml`. |
| `DS_ENVOY_ADMIN_PORT` | no | `9901` | Envoy admin interface port. |
| `DS_HEARTBEAT_INTERVAL` | no | `15s` | How often the agent heartbeats. |
| `DS_LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error`. |
| `DS_AGENT_ID` | no | auto | Override the agent's registered ID. Leave empty to auto-register. |

---

## 4. Verify & First Deployment

Once the agent is running, open the dashboard and navigate to your app. Look for the **Agent Status** indicator:

- **Green dot** — connected, heartbeating within the last interval.
- **Yellow** — stale (heartbeat missed but recently seen).
- **Red** — disconnected.

When green, run your first canary:

```bash
deploysentry deploy create --strategy canary --version v2.0 --env production
```

Watch the **Traffic Distribution** panel on the Deployment Detail page. You should see traffic shift from `blue` → `green` in the steps defined by your canary policy (e.g., 10% → 25% → 50% → 100%).

If traffic doesn't shift, jump to section 8 for troubleshooting.

---

## 5. Key Management Strategy

Use the most specific scope that accomplishes the task. The table below is the recommended baseline for a team:

| Role | Key Scope | Scopes |
|---|---|---|
| CI/CD pipeline | Project | `deploys:write` |
| Agent (per env) | Application + Environment | `deploys:read`, `deploys:write`, `flags:read` |
| Developer | Project | `flags:read`, `flags:write` |
| Readonly dashboard | Org-wide | `flags:read`, `deploys:read` |
| Admin / emergency | Org-wide | `admin` |

**Rule of thumb:** scope the key to the smallest resource it needs. If a key is compromised, the blast radius is bounded by its scope.

Rotate agent keys at least quarterly, and immediately when an agent host is decommissioned. Use `deploysentry apikeys revoke <id>` to invalidate.

---

## 6. Multi-Environment Setup

Run **one agent per environment, per application**. Each agent gets its own environment-scoped key. That way a misconfigured dev agent physically cannot push traffic changes to production.

```bash
# Dev agent
DS_API_KEY=ds_dev_key DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082 deploysentry-agent

# Staging agent
DS_API_KEY=ds_staging_key DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082 deploysentry-agent

# Production agent
DS_API_KEY=ds_prod_key DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082 deploysentry-agent
```

Even if an operator runs `deploysentry deploy create --env production` against the dev agent's host, the API will reject it because the dev key's environment restriction is `development`.

---

## 7. Platform Deployment Guides

### Docker Compose (local dev)

Local dev uses the repo's compose stack.

1. `make dev-up` — starts PostgreSQL, Redis, NATS.
2. `make migrate-up` — applies migrations into the `deploy` schema.
3. `make run-api` — starts the control plane API on `:8080`.
4. `make dev-deploy` — brings up the agent, Envoy, and two upstream containers.

Containers involved:

- `ds-agent` — the DeploySentry agent; streams xDS to Envoy and SSE to the control plane.
- `envoy` — L7 proxy; routes requests to upstreams based on the agent's current traffic split.
- `app-blue`, `app-green` — sample upstream versions representing old and new.

### Render

```yaml
# render.yaml
services:
  - type: web
    name: my-app-agent
    env: docker
    dockerfilePath: ./deploy/docker/Dockerfile.agent
    envVars:
      - key: DS_API_KEY
        sync: false
      - key: DS_UPSTREAMS
        value: blue:app-blue:8081,green:app-green:8082
      - key: DS_API_URL
        value: https://api.dr-sentry.com
```

Commit `render.yaml` and connect the repo in the Render dashboard. `sync: false` prevents the secret from being written into the synced blueprint.

### Railway

Use `deploy/docker/Dockerfile.agent` or a `nixpacks.toml` that installs the agent binary.

```bash
railway variables set DS_API_KEY=ds_xxx
railway variables set DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082
railway up
```

Set the start command to `deploysentry-agent` in Railway's service settings.

### Fly.io

```toml
# fly.toml
app = "my-app-agent"
primary_region = "sjc"

[build]
  dockerfile = "deploy/docker/Dockerfile.agent"

[processes]
  agent = "deploysentry-agent"

[env]
  DS_API_URL = "https://api.dr-sentry.com"
  DS_UPSTREAMS = "blue:app-blue:8081,green:app-green:8082"
```

Set the API key as a Fly secret:

```bash
fly secrets set DS_API_KEY=ds_xxx
fly deploy
```

---

## 8. Monitoring & Troubleshooting

### Dashboard panels

- **Deployment Detail** — traffic distribution, per-version request rate, error rate, p50/p95/p99 latency.
- **Agent Status** — connected / stale / disconnected indicator with last-heartbeat timestamp.
- **Flags Under Test** — only shown for flag-driven canary deployments; lists flags currently being rolled out.

### Troubleshooting

| Issue | Cause | Fix |
|---|---|---|
| Agent not connecting | Wrong API URL or invalid key | Verify `DS_API_KEY` hasn't been revoked and `DS_API_URL` is reachable from the agent host. |
| `Agent not authorized for application` | Key scoped to a different app | Create a new key scoped to the correct app (section 2). |
| Envoy not routing | xDS port mismatch | `DS_ENVOY_XDS_PORT` must match the `clusters` entry in `envoy-bootstrap.yaml`. |
| `Stream error: terminated` | Cloudflare / proxy buffering SSE | The server disables `WriteTimeout` for SSE endpoints — verify you are on the latest agent version. |
| No flag updates reaching SDK | SSE event-type mismatch between SDK and server | Use SDK version `1.1.0` or later. |
| Traffic split ignored | Envoy started before agent first xDS push | Restart Envoy, or add `--wait-for-agent` flag; the agent will resend on Envoy reconnect. |

Enable `DS_LOG_LEVEL=debug` on the agent when diagnosing. The control plane logs the agent's `application_id` and environment on every heartbeat — mismatches usually indicate a misscoped key.

---

## 9. LLM-Assisted Setup

### MCP Server (automated)

For Claude Code, Cursor, or any MCP-compatible client, DeploySentry ships a built-in MCP server:

```bash
deploysentry mcp serve
```

Then prompt the LLM:

> "Set up deploy monitoring for my app. My org is `acme`, project is `backend`, app is `api-server`, environment is `production`."

The LLM calls the MCP tools in sequence:

- `ds_list_orgs` — discover orgs you belong to.
- `ds_list_projects` — find the project.
- `ds_list_apps` — find the app.
- `ds_create_api_key` — create a scoped key.
- `ds_setup_local_deploy` — emit the agent env var block and platform-specific start command.

**Operational loop (day 2+):**

- `ds_create_deployment` — start a canary.
- `ds_get_traffic_state` — read the current traffic split and per-version health.
- `ds_promote_deployment` — advance to the next step or complete the rollout.
- `ds_rollback_deployment` — abort and restore the prior version.

### Prompt Templates (no MCP)

If your LLM client can't run MCP, use these prompts with the REST API directly.

**Bootstrap prompt:**

> "I want to set up DeploySentry deploy monitoring. My API key is `ds_xxx` and the base URL is `https://api.dr-sentry.com`. Please: 1) list my orgs via `GET /api/v1/orgs`, 2) list projects for my chosen org, 3) list apps, 4) walk me through creating a scoped API key via `POST /api/v1/api-keys`, 5) give me the agent env var config."

**Deploy prompt:**

> "Using my DeploySentry API key `ds_xxx`, create a canary deployment for app `X` in environment `Y` with version `v2.0`. Then poll the traffic state every 30 seconds and tell me when to promote or if I should rollback based on error rates."

**Debug prompt:**

> "My DeploySentry agent isn't connecting. Using API key `ds_xxx`, check: 1) agents registered for my app via `GET /api/v1/applications/:app_id/agents`, 2) recent heartbeats via `GET /api/v1/agents/:id/heartbeats`, 3) API key scope via `GET /api/v1/api-keys`. Diagnose what's wrong."

---

When onboarding is complete, move on to [Traffic_Management_Guide.md](./Traffic_Management_Guide.md) to tune canary step policies and automated rollback thresholds.
