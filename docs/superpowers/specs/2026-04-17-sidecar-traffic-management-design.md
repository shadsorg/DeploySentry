# Sidecar Traffic Management & Controlled Rollouts

**Date:** 2026-04-17
**Status:** Design

## Overview

DeploySentry already has a deployment engine with canary, blue-green, and rolling strategies, a desired-state API, and health monitoring with automatic rollback. What's missing is the bridge between the deployment engine's desired state and actual traffic routing — today the only reference implementation is a polling-based Nginx demo.

This design introduces a **DeploySentry Agent** — a Go sidecar binary that acts as an Envoy xDS control plane. The agent receives desired state from the DeploySentry API via SSE, pushes traffic weights to Envoy in real-time, and reports actual traffic metrics back via heartbeats. This gives operators a unified dashboard view showing both what the deployment engine asked for and what's actually happening.

The architecture works identically on local Docker (Envoy + agent as separate containers) and PaaS platforms like Render/Railway (agent as entrypoint within the service boundary, routing to co-located blue/green processes).

A key capability is **same-version deployments for flag canaries**: deploy identical code to blue and green, use `SERVICE_COLOR` to differentiate them in flag evaluation context, and canary-test a feature flag change through traffic splitting rather than percentage rollout.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Agent pattern | Integrated sidecar | Eliminates polling latency, provides heartbeat for observability, works on PaaS where a separate consumer process isn't possible |
| Proxy | Envoy | xDS API enables zero-downtime weight changes without restarts. `go-control-plane` library handles gRPC plumbing. Purpose-built for dynamic traffic management. |
| xDS control plane | Agent implements xDS (gRPC) | The "proper" Envoy integration — no config file templating, no hot restarts. Push new snapshots to SnapshotCache, Envoy picks up changes in milliseconds. |
| PaaS traffic control | Application-level routing | Agent is the entrypoint, receives all traffic, routes to blue/green processes internally. Platform sees one service. Consistent behavior regardless of platform. |
| Observability | Unified deployment + traffic view | Dashboard shows deployment phases (where you are in the plan) AND real-time traffic panel (what's actually happening). Agent heartbeat is the data source. |
| Flag canary mechanism | `SERVICE_COLOR` env var | Set at process startup, SDK auto-includes in evaluation context. No per-request header injection needed. Works for HTTP, background jobs, queues. |

## Components

### 1. DeploySentry Agent (`cmd/agent/`, `internal/agent/`)

A Go binary with three responsibilities:

**Receive desired state** — Connects to the DeploySentry API via SSE, subscribing to `deployments.deployment.phase_changed` events for its app ID. On startup, fetches current desired state via `GET /applications/:app_id/desired-state`. SSE keeps it in sync from there.

**Manage Envoy via xDS** — Implements a minimal xDS control plane using `go-control-plane`. Serves CDS (Cluster Discovery), EDS (Endpoint Discovery), and RDS (Route Discovery) over gRPC on port 18000. When desired state changes, the agent pushes a new snapshot to the `SnapshotCache` with updated cluster weights. Envoy connects to the agent as its control plane and picks up changes without restart.

The xDS config includes:
- **Weighted clusters** — blue/green with traffic percentages from the deployment phase
- **Header-match routes** — override rules (e.g., `X-Version: canary` → always route to green) for developer testing during rollout
- **Hash-based routing** — for sticky sessions when enabled (cookie or header based)

**Report actual state** — Sends heartbeats to `POST /api/v1/agents/:id/heartbeat` every 5 seconds with:
- Agent health (alive, connected, Envoy responsive)
- Actual traffic distribution (from Envoy stats API)
- Per-upstream metrics (request rate, error rate, P99/P50 latency)
- Current Envoy config version (so dashboard can show desired vs. applied)
- Active routing rules summary

**Configuration:**
```
DS_API_URL=http://localhost:8080       # DeploySentry API
DS_API_KEY=ds_key_xxx                  # Agent API key
DS_APP_ID=<uuid>                       # Application being managed
DS_ENVIRONMENT=production              # Environment
DS_UPSTREAMS=blue:localhost:8081,green:localhost:8082
DS_ENVOY_XDS_PORT=18000                # xDS gRPC listen port
DS_HEARTBEAT_INTERVAL=5s               # Heartbeat frequency
```

**Location:** `cmd/agent/main.go` for the binary entrypoint. `internal/agent/` for core logic:
- `internal/agent/xds/` — xDS server, snapshot management
- `internal/agent/sse/` — SSE client for desired-state stream
- `internal/agent/reporter/` — Heartbeat reporter, Envoy stats collector
- `internal/agent/config/` — Agent configuration

### 2. API Additions

All additions follow existing patterns (service/repository/handler in `internal/agent/`).

**Agent Registry:**
- `POST /api/v1/agents/register` — Agent registers on startup with app ID, environment, agent version, upstream config. Returns `agent_id`.
- `POST /api/v1/agents/:id/heartbeat` — Agent heartbeat with actual traffic state and metrics.
- `GET /api/v1/applications/:app_id/agents` — Dashboard fetches registered agents.
- `DELETE /api/v1/agents/:id` — Graceful shutdown deregistration.

**Agent States:** `connected` → `stale` (missed 2+ heartbeats / 10s) → `disconnected` (missed 6+ heartbeats / 30s). Stale/disconnected agents surface as warnings on the dashboard.

**Heartbeat Payload:**
```json
{
  "agent_id": "uuid",
  "deployment_id": "uuid",
  "config_version": 42,
  "actual_traffic": { "blue": 94.8, "green": 5.2 },
  "upstreams": {
    "blue":  { "rps": 1140, "error_rate": 0.08, "p99_ms": 32, "p50_ms": 8 },
    "green": { "rps": 60,   "error_rate": 0.10, "p99_ms": 45, "p50_ms": 12 }
  },
  "active_rules": {
    "weights": { "blue": 95, "green": 5 },
    "header_overrides": [{ "header": "X-Version", "value": "canary", "upstream": "green" }],
    "sticky_sessions": { "enabled": false }
  },
  "envoy_healthy": true
}
```

**New Database Tables:**

`agents`:
| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| app_id | uuid | FK to applications |
| environment_id | uuid | FK to environments |
| status | text | connected, stale, disconnected |
| version | text | Agent binary version |
| upstream_config | jsonb | Upstream host/port mapping |
| last_seen_at | timestamptz | Last heartbeat time |
| registered_at | timestamptz | Registration time |

`agent_heartbeats`:
| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| agent_id | uuid | FK to agents |
| deployment_id | uuid | FK to deployments (nullable) |
| payload | jsonb | Full heartbeat payload |
| created_at | timestamptz | Heartbeat time |

Rolling retention: keep last 100 heartbeats per agent per deployment. Pruned on each heartbeat insert — after writing the new row, delete rows beyond the 100 newest for that agent+deployment pair. No background job needed.

**SSE:** No new event types needed. The existing `deployments.deployment.phase_changed` event already carries `traffic_percent`. The agent subscribes to the SSE stream filtered by app ID.

### 3. Local Docker Compose Setup

New compose file `deploy/docker/docker-compose.deploy.yml` extending the base. Activated via `make dev-deploy`.

**Services:**

| Service | Image | Port | Role |
|---------|-------|------|------|
| `app-blue` | User's app | 8081 (internal) | Stable version, `SERVICE_COLOR=blue` |
| `app-green` | User's app | 8082 (internal) | New version (or same for flag canary), `SERVICE_COLOR=green` |
| `envoy` | `envoyproxy/envoy:v1.31` | 8080 (external) | Data plane — routes based on xDS |
| `deploysentry-agent` | `deploysentry/agent` | 18000 (xDS gRPC) | Control plane — receives desired state, pushes to Envoy |

**Envoy bootstrap:** Minimal static config that points Envoy at the agent's xDS server. All routing config (listeners, routes, clusters, weights) is dynamic via xDS — no static upstream definitions.

```yaml
dynamic_resources:
  cds_config:
    api_config_source:
      api_type: GRPC
      grpc_services:
        - envoy_grpc:
            cluster_name: xds_cluster
  lds_config:
    api_config_source:
      api_type: GRPC
      grpc_services:
        - envoy_grpc:
            cluster_name: xds_cluster
static_resources:
  clusters:
    - name: xds_cluster
      connect_timeout: 1s
      type: STATIC
      load_assignment:
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: deploysentry-agent
                      port_value: 18000
```

**Developer workflow:**
```bash
make dev-up          # Postgres, Redis, NATS (existing)
make dev-deploy      # Adds Envoy, agent, blue/green instances
make run-api         # DeploySentry API

# Create a deployment from dashboard or CLI:
deploysentry deploy create --strategy canary --version v2.0 --env production
# Watch traffic shift in real-time on the dashboard
```

### 4. PaaS Deployment (Render, Railway, Fly.io)

Same architecture, different packaging. The agent runs as the service entrypoint inside the platform's service boundary.

**Service structure:** The platform sees one service. Internally:
- The agent + Envoy handle incoming traffic on the platform-assigned port
- `app-blue` and `app-green` run as co-located processes on internal ports
- The agent connects outbound to the DeploySentry API for SSE and heartbeats

**Process management:** The agent manages the app processes (or they're managed by the platform's multi-process support — Render's `Procfile`, Railway's `nixpacks`, Fly.io's `processes` config). The agent doesn't need to start the apps — it just needs to know their addresses.

**Key constraint:** PaaS platforms control the external port. The agent must listen on the platform-assigned `PORT` env var and proxy to internal upstreams. This is a configuration detail, not an architectural change.

### 5. Same-Version Deployments for Flag Canaries

Deploy identical code to blue and green to canary-test a feature flag change through traffic splitting.

**How it works:**
1. Both `app-blue` and `app-green` run the same artifact version
2. `app-blue` starts with `SERVICE_COLOR=blue`, `app-green` with `SERVICE_COLOR=green`
3. All DeploySentry SDKs auto-detect the `SERVICE_COLOR` env var and include it in every evaluation context as `service_color`
4. Operator creates a flag targeting rule: `service_color eq green` → flag enabled
5. Traffic splitting via canary phases means 5% of users hit green (flag on), 95% hit blue (flag off)
6. Dashboard shows per-color flag evaluation results and metrics

**SDK convention:** If `SERVICE_COLOR` is set, SDKs automatically include `{ service_color: "<value>" }` in the evaluation context without developer code changes:

```go
// SDK auto-detects SERVICE_COLOR=green
client, _ := deploysentry.NewClient(
    deploysentry.WithAPIKey(os.Getenv("DEPLOYSENTRY_API_KEY")),
    // Every evaluation automatically includes service_color: "green"
)
```

**Deployment creation:**
```bash
# Flag canary — same version, testing a flag change
deploysentry deploy create \
  --strategy canary \
  --version v1.0.0 \
  --flag-test my-new-feature \
  --env production
```

The `--flag-test` parameter links the deployment to a flag. The dashboard highlights this as a flag canary deployment and shows the "Flags Under Test" section prominently.

### 6. Dashboard Changes

**Enhanced DeploymentDetailPage** — two-column layout:

**Left column (deployment lifecycle):**
- Phase progression timeline (existing, preserved)
- Agent Status — heartbeat freshness, Envoy health, config version, connection duration
- Rollback history (existing, preserved)

**Right column (real-time traffic panel — new):**
- **Traffic Distribution** — desired vs. actual horizontal bars per upstream, sourced from agent heartbeats
- **Traffic Rules** — compact summary of active routing rules (weights, header overrides, sticky sessions). Click to expand slide-out panel for viewing and editing rules mid-rollout
- **Per-Version Metrics** — side-by-side cards comparing blue/green on RPS, error rate, P99/P50 latency
- **Health Comparison** — automated green/yellow/red assessment comparing canary metrics against stable baseline
- **Traffic Over Time** — stacked bar chart showing split history with phase-change markers
- **Flags Under Test** (for flag canary deployments) — which flags have targeting rules referencing the deployment's upstream color, per-flag per-color evaluation results, flag-specific metrics

**Existing elements preserved:** Action buttons (Advance Phase, Promote 100%, Rollback, Pause, Resume, Cancel), confirmation dialogs, phase detail view.

## Scope Boundaries

**In scope:**
- Agent binary with xDS control plane, SSE client, heartbeat reporter
- Agent registry API (register, heartbeat, list, deregister)
- Agent and heartbeat database tables + migrations
- Local Docker Compose multi-instance setup with Envoy
- Dashboard enhancements (traffic panel, agent status, traffic rules, flags under test)
- SDK `SERVICE_COLOR` auto-detection (all SDKs)
- `--flag-test` deployment mode
- Envoy bootstrap config and Dockerfile

**Out of scope (future work):**
- PaaS-specific deployment templates (Render render.yaml, Railway nixpacks, Fly.io fly.toml) — document the pattern, don't ship templates yet
- Multi-agent coordination (multiple agents for the same app) — single agent per app for now
- Envoy features beyond weighted routing, header overrides, and sticky sessions (circuit breaking, rate limiting, retries)
- Prometheus metrics endpoint on the agent
- Agent auto-update mechanism

## Dependencies

- `github.com/envoyproxy/go-control-plane` — xDS gRPC server implementation
- `envoyproxy/envoy:v1.31` — Envoy proxy Docker image
- Existing: deployment engine, desired-state API, SSE streaming, health monitoring, rollback controller
