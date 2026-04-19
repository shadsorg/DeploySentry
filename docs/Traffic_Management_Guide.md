# Traffic Management Guide

**Phase**: Implementation

> **New to DeploySentry?** Start with the [Deploy Monitoring Setup](./Deploy_Monitoring_Setup.md) guide to create your project, app, API key, and agent configuration.

### Related

- [Rollout Strategies](./Rollout_Strategies.md) — reusable templates that describe how traffic is shifted step by step during a deploy or config rollout.

## Overview

The DeploySentry agent is a Go binary that runs as a sidecar alongside your application. It acts as an Envoy xDS control plane, receiving desired traffic state from the DeploySentry API via SSE and programming Envoy's routing configuration in real time. The agent also sends periodic heartbeats back to the API, reporting actual traffic distribution, Envoy health, and per-version metrics.

This gives you infrastructure-level traffic splitting (blue/green, canary) managed through the same API and dashboard you already use for feature flags and deployments.

### Architecture

```
                        DeploySentry API
                       ┌───────────────┐
                       │               │
               SSE     │  Desired      │  Heartbeats
          (desired ──► │  State        │ ◄── (actual
           state)      │  Store        │      metrics)
                       └──┬────────▲───┘
                          │        │
                          │        │
                     ┌────▼────────┴────┐
                     │   Agent (Go)     │
                     │   xDS Control    │
                     │   Plane :18000   │
                     └────┬─────────────┘
                          │ xDS (CDS/EDS/RDS)
                          │
                     ┌────▼─────────────┐
                     │   Envoy Proxy    │
                     │   :8080          │
                     └──┬────────────┬──┘
                        │            │
              ┌─────────▼──┐  ┌─────▼──────────┐
              │ app-blue   │  │ app-green       │
              │ :8081      │  │ :8082           │
              └────────────┘  └────────────────┘
```

**Data flow:**

1. The API pushes desired traffic weights to the agent over SSE.
2. The agent translates weights into Envoy xDS cluster/route configuration.
3. Envoy splits incoming requests across app-blue and app-green according to the weights.
4. The agent collects per-upstream metrics from Envoy and reports them back to the API via heartbeats.

### Traffic Splitting vs Flag Percentage Rollout

| | Traffic Splitting (Envoy) | Flag Percentage Rollout (SDK) |
|---|---|---|
| **What changes** | Which backend instance serves the request | Which code path runs inside a single instance |
| **Granularity** | Per-request, at the load balancer | Per-user, deterministic hash |
| **Rollback speed** | Instant (shift traffic to 0%) | Instant (disable flag) |
| **Metrics isolation** | Full per-version metrics (RPS, latency, errors) | Shared process metrics, flag-level analytics only |
| **Infrastructure validation** | Yes (new binary, dependencies, config) | No (same binary, same process) |
| **Use when** | Deploying new versions, testing infrastructure changes | Rolling out features within a single version |

---

## Local Setup

### Prerequisites

- Docker and Docker Compose
- DeploySentry API running locally

### Step 1: Start backing services and API

```bash
# Terminal 1: Start PostgreSQL, Redis, NATS
make dev-up
make migrate-up

# Terminal 2: Start the API server
make run-api
```

### Step 2: Configure environment variables

The agent needs to know which application it manages and how to authenticate with the API.

**Find your app ID via CLI:**

```bash
deploysentry apps list
# Output:
# ID                                    NAME        SLUG
# a1b2c3d4-5678-90ab-cdef-1234567890ab  My App      my-app
```

**Or via API:**

```bash
curl -s http://localhost:8080/api/v1/orgs/my-org/projects/my-project/apps \
  -H "Authorization: ApiKey ds_key_xxxxxxxxxxxx" | jq '.[].id'
```

**Create an API key (if you don't have one):**

```bash
deploysentry apikeys create --name "agent" --scopes deploys:read,deploys:write,flags:read
```

Set the variables:

```bash
export DS_APP_ID="a1b2c3d4-5678-90ab-cdef-1234567890ab"
export DS_API_KEY="ds_key_xxxxxxxxxxxx"
```

### Step 3: Start the deployment stack

```bash
make dev-deploy
```

This starts four containers:

| Container | Port | Purpose |
|-----------|------|---------|
| app-blue | 8081 | Current production version |
| app-green | 8082 | New candidate version |
| envoy | 8080 | Front proxy, splits traffic |
| agent | 18000 | xDS control plane, manages Envoy |

### Step 4: Verify the agent connection

```bash
curl -s http://localhost:8080/api/v1/apps/$DS_APP_ID/agent-status \
  -H "Authorization: ApiKey $DS_API_KEY" | jq .
```

Expected output:

```json
{
  "connected": true,
  "last_seen": "2026-04-17T12:00:05Z",
  "envoy_healthy": true,
  "config_version": 1,
  "upstreams": {
    "blue": { "address": "localhost:8081", "weight": 100 },
    "green": { "address": "localhost:8082", "weight": 0 }
  }
}
```

### Step 5: Create a canary deployment and watch traffic shift

```bash
deploysentry deploy create \
  --app my-app \
  --strategy canary \
  --version v2.1.0 \
  --env production
```

Watch traffic shift in real time:

```bash
# Poll agent status to see weights change
watch -n 2 'curl -s http://localhost:8080/api/v1/apps/'$DS_APP_ID'/agent-status \
  -H "Authorization: ApiKey '$DS_API_KEY'" | jq .upstreams'
```

The canary strategy shifts traffic through phases: 1% -> 5% -> 25% -> 50% -> 100%, pausing at each phase for health checks.

---

## Agent Configuration Reference

| Variable | Default | Required | Description |
|---|---|---|---|
| `DS_API_URL` | `http://localhost:8080` | No | DeploySentry API base URL |
| `DS_API_KEY` | -- | Yes | API key with `deploys:read,deploys:write` scopes |
| `DS_APP_ID` | -- | Yes | Application UUID the agent manages |
| `DS_ENVIRONMENT` | `production` | No | Environment name for deployment lookups |
| `DS_UPSTREAMS` | `blue:localhost:8081,green:localhost:8082` | No | Comma-separated upstream definitions (`name:host:port`) |
| `DS_ENVOY_XDS_PORT` | `18000` | No | Port the agent listens on for Envoy xDS connections |
| `DS_ENVOY_LISTEN_PORT` | `8080` | No | Port Envoy listens on for inbound traffic |
| `DS_HEARTBEAT_INTERVAL` | `5s` | No | How often the agent reports metrics to the API |

---

## Traffic Splitting

### Weighted Routing

Envoy distributes requests across upstreams using weighted round-robin. The agent programs these weights via xDS whenever the desired state changes.

Example: a 90/10 canary split means Envoy sends 90% of requests to blue and 10% to green. The weights are integers that Envoy normalizes, so `90:10` and `9:1` produce the same distribution.

```bash
# Check current weights
curl -s http://localhost:8080/api/v1/apps/$DS_APP_ID/agent-status \
  -H "Authorization: ApiKey $DS_API_KEY" | jq '.upstreams | to_entries[] | {name: .key, weight: .value.weight}'
```

During a canary deployment, the API advances weights automatically based on phase configuration and health check results. You can also set weights manually:

```bash
# Manually set traffic to 50/50
curl -X PUT http://localhost:8080/api/v1/deployments/<deploy-id>/traffic \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"blue": 50, "green": 50}'
```

### Header Overrides

During a canary deployment, developers can bypass traffic weights to test the green version directly by sending a header:

```bash
# Route this request to the canary (green) regardless of weights
curl -H "X-Version: canary" http://localhost:8080/my-endpoint

# Route to the stable (blue) version explicitly
curl -H "X-Version: stable" http://localhost:8080/my-endpoint
```

This is configured in Envoy's route table by the agent. No weight changes are needed -- the header match takes precedence over weighted routing.

### Sticky Sessions

For stateful workflows where a user must stay on the same backend version for the duration of a session, enable cookie-based sticky sessions via the deployment API:

```bash
deploysentry deploy create \
  --app my-app \
  --strategy canary \
  --version v2.1.0 \
  --env production \
  --sticky-sessions
```

Once enabled, Envoy sets a `ds-version` cookie on the first response. Subsequent requests with that cookie are routed to the same upstream, bypassing weight-based distribution.

---

## Flag Canaries

Flag canaries combine traffic splitting with feature flags. Both app-blue and app-green run the same codebase, but the SDK uses the `SERVICE_COLOR` environment variable to apply different flag values per version.

### How SERVICE_COLOR Works

Each container sets `SERVICE_COLOR` to its version identifier:

```bash
# app-blue container
SERVICE_COLOR=blue

# app-green container
SERVICE_COLOR=green
```

The DeploySentry SDK automatically includes `service_color` in the evaluation context when this variable is set. No code changes needed -- the SDK reads `SERVICE_COLOR` from the environment at initialization.

### Create a Flag Canary

**Step 1: Create a feature flag.**

```bash
deploysentry flag create \
  --key my-feature \
  --name "My Feature" \
  --type boolean \
  --category release \
  --purpose "Canary test of new feature" \
  --owners team-backend \
  --expires-at 2026-05-01T00:00:00Z \
  --default false \
  --env production
```

**Step 2: Add a targeting rule that enables the flag only on green.**

```bash
curl -X POST http://localhost:8080/api/v1/flags/<flag-id>/rules \
  -H "Authorization: ApiKey $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "rule_type": "attribute",
    "attribute": "service_color",
    "operator": "eq",
    "value": "green",
    "result_value": "true",
    "priority": 1
  }'
```

**Step 3: Create a deployment with flag testing enabled.**

```bash
deploysentry deploy create \
  --app my-app \
  --strategy canary \
  --version v2.1.0 \
  --env production \
  --flag-test my-feature
```

The `--flag-test` option tells the dashboard to track this flag alongside the deployment, showing flag evaluation rates per version.

### Why Use Flag Canaries vs Flag Percentage Rollout

| Concern | Flag Canary | Flag Percentage Rollout |
|---|---|---|
| Per-version metrics | Yes -- blue and green have separate RPS, error rate, latency | No -- single process, shared metrics |
| Infrastructure validation | Yes -- green may have new dependencies, config, binary | No -- same binary serves both paths |
| Instant kill switch | Shift traffic to 0% green | Disable the flag |
| Cleanup | Remove traffic split + remove flag rule | Remove flag |
| Complexity | Higher -- requires agent, Envoy, two containers | Lower -- SDK only |

Use flag canaries when you need to validate that a feature works correctly on a new deployment target with full metric isolation. Use percentage rollouts when the change is purely in application logic and the same binary handles both code paths.

---

## Dashboard Observability

The deployment detail page in the web dashboard shows real-time traffic data when an agent is connected.

### Traffic Distribution Panel

Displays desired vs actual traffic weights as horizontal bars. The desired weight comes from the deployment phase configuration. The actual weight is computed from the agent's heartbeat data. A mismatch indicates Envoy is still converging or the agent is unhealthy.

### Per-Version Metrics

Each upstream (blue, green) shows:

| Metric | Description |
|---|---|
| RPS | Requests per second over the last heartbeat interval |
| Error Rate | Percentage of 5xx responses |
| P99 Latency | 99th percentile response time |
| P50 Latency | Median response time |

These metrics come from Envoy stats, collected by the agent and reported via heartbeats.

### Agent Status

| Indicator | Meaning |
|---|---|
| Connection | Whether the agent has an active SSE connection to the API |
| Last Seen | Timestamp of the most recent heartbeat |
| Envoy Health | Whether the agent can reach Envoy's admin interface |
| Config Version | The xDS configuration version currently applied |

### Traffic Rules

Shows the active routing configuration:

- **Weights** -- current blue/green split percentages
- **Header Overrides** -- any active `X-Version` header routing rules
- **Sticky Sessions** -- whether cookie-based session affinity is enabled

### Flags Under Test

Visible only for deployments created with `--flag-test`. Shows:

- Flag key and current enabled state
- Evaluation count per version (blue vs green)
- Per-version flag evaluation breakdown (enabled vs disabled)

---

## PaaS Deployment

On platforms like Heroku, Railway, or Fly.io, you cannot run Envoy as a separate service. Instead, the agent runs as the container entrypoint and manages Envoy as a child process. The platform sees a single service.

### Architecture

```
  Platform Edge LB
        │
        ▼
  ┌─────────────────────────────────┐
  │  Service Container              │
  │                                 │
  │  ┌───────────────────────────┐  │
  │  │  Agent (entrypoint)       │  │
  │  │  - Spawns Envoy           │  │
  │  │  - xDS control plane      │  │
  │  │  - SSE to DeploySentry    │  │
  │  └──────────┬────────────────┘  │
  │             │ xDS                │
  │  ┌──────────▼────────────────┐  │
  │  │  Envoy (:$PORT)           │  │
  │  └──────┬───────────┬───────┘  │
  │         │           │          │
  │  ┌──────▼───┐ ┌─────▼──────┐  │
  │  │ app-blue │ │ app-green  │  │
  │  │ :8081    │ │ :8082      │  │
  │  └──────────┘ └────────────┘  │
  └─────────────────────────────────┘
```

### Key Considerations

**PORT environment variable.** Most PaaS platforms assign a dynamic port via the `PORT` env var. Configure the agent to tell Envoy to listen on that port:

```bash
DS_ENVOY_LISTEN_PORT=$PORT
```

**Process management.** The agent is the PID 1 process. It spawns and supervises both Envoy and the application processes. If any child exits unexpectedly, the agent logs the failure, reports it via heartbeat, and restarts the process.

**Outbound connectivity.** The agent needs outbound HTTPS access to your DeploySentry API (or `api.dr-sentry.com` for the hosted version) for SSE and heartbeats. Ensure your platform does not block outbound connections on the agent's behalf.

**Health checks.** Configure your platform's health check to hit Envoy's listener port. The agent exposes a `/healthz` endpoint on its xDS port (18000) for internal monitoring:

```bash
curl http://localhost:18000/healthz
```

---

## Troubleshooting

| Issue | Cause | Fix |
|---|---|---|
| Agent not connecting to API | Wrong `DS_API_URL` or `DS_API_KEY` | Verify the URL is reachable: `curl $DS_API_URL/health`. Check the API key has `deploys:read` scope. |
| Envoy not picking up config | Agent cannot reach Envoy's xDS listener | Confirm Envoy is configured to use the agent's xDS port (`18000`). Check agent logs for xDS stream errors. |
| Traffic not shifting after deploy | Deployment is paused waiting for health check | Check deployment status: `deploysentry deploy status <id>`. Look for `health_check_pending` phase. Verify the health endpoint returns 200. |
| Dashboard not showing traffic panel | No agent connected for this app | The traffic panel only appears when the API has received at least one heartbeat. Check agent status: `curl /api/v1/apps/$DS_APP_ID/agent-status`. |
| Heartbeat gaps in dashboard | Agent losing SSE connection | Check agent logs for reconnection messages. Verify network stability. The agent reconnects with exponential backoff (1s initial, 30s max). |
| `SERVICE_COLOR` not in flag context | Environment variable not set in container | Verify the variable is set: `docker exec app-green env | grep SERVICE_COLOR`. The SDK reads it at initialization -- restart the app after setting it. |
| Blue and green serving same content | Both containers running the same image/version | Confirm each container is running the correct version: `docker exec app-green ./app --version`. For flag canaries, confirm the targeting rule matches `service_color eq green`. |
| Header override not working | `X-Version` header not reaching Envoy | Ensure no upstream proxy is stripping custom headers. Test directly against Envoy: `curl -H "X-Version: canary" http://localhost:8080/`. |

---

## Checklist

- [x] Overview and architecture diagram
- [x] Local setup steps
- [x] Agent configuration reference
- [x] Traffic splitting documentation
- [x] Flag canaries documentation
- [x] Dashboard observability
- [x] PaaS deployment guide
- [x] Troubleshooting table

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: `feature/groups-and-resource-authorization`
- **Committed**: No
- **Pushed**: No
- **CI Checks**: N/A
