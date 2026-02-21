# DeploySentry — Sentinel Project Plan

## 1. Executive Summary

DeploySentry is a deploy release and feature flag management platform designed to give engineering teams full visibility and control over their software deployment lifecycle. The **Sentinel** initiative is the foundational project plan that defines the architecture, components, milestones, and operational strategy for building DeploySentry from the ground up.

### Vision

Provide a single pane of glass for deployment orchestration, progressive rollouts, feature flag management, and release health monitoring — enabling teams to ship faster with confidence.

### Goals

- **Safe deployments**: Canary releases, blue/green deployments, and automated rollbacks
- **Feature flag management**: Granular targeting, percentage rollouts, and kill switches
- **Release tracking**: End-to-end visibility from commit to production
- **Observability integration**: Connect deploys to metrics, logs, and error rates
- **Developer experience**: CLI-first workflow with dashboard for oversight

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        DeploySentry Platform                     │
├─────────────┬─────────────┬──────────────┬──────────────────────┤
│   Web UI    │   CLI Tool  │  REST API    │  Webhooks / Events   │
│  (React)    │  (Go/Rust)  │  (Go)        │  (async workers)     │
├─────────────┴─────────────┴──────────────┴──────────────────────┤
│                         API Gateway                              │
│                    (Auth, Rate Limiting, Routing)                 │
├──────────────────────────────────────────────────────────────────┤
│                        Core Services                             │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │  Deploy      │  │  Feature     │  │  Release           │     │
│  │  Service     │  │  Flag        │  │  Tracker           │     │
│  │              │  │  Service     │  │  Service            │     │
│  └──────────────┘  └──────────────┘  └────────────────────┘     │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │  Rollback    │  │  Health      │  │  Notification      │     │
│  │  Controller  │  │  Monitor     │  │  Service            │     │
│  └──────────────┘  └──────────────┘  └────────────────────┘     │
├──────────────────────────────────────────────────────────────────┤
│                     Data & Infrastructure                        │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌───────────────┐  │
│  │PostgreSQL│  │  Redis   │  │  NATS /   │  │  Object       │  │
│  │          │  │          │  │  Kafka    │  │  Storage (S3) │  │
│  └──────────┘  └──────────┘  └───────────┘  └───────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 Service Descriptions

| Service | Responsibility |
|---------|---------------|
| **Deploy Service** | Orchestrates deployment pipelines — canary, blue/green, rolling updates. Interfaces with container orchestrators (K8s) and cloud providers. |
| **Feature Flag Service** | CRUD for flags, evaluation engine, targeting rules, SDK integration. Low-latency evaluation path with local caching. |
| **Release Tracker Service** | Maps commits/PRs to releases, tracks promotion through environments (dev → staging → prod). |
| **Rollback Controller** | Automated and manual rollback triggers based on health signals. Maintains rollback state machine. |
| **Health Monitor** | Aggregates signals from APM, error tracking, and custom metrics to compute release health scores. |
| **Notification Service** | Slack, PagerDuty, email, and webhook integrations for deploy events and alerts. |

### 2.3 Technology Choices

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Backend services | Go | Performance, strong concurrency, proven in infrastructure tooling |
| CLI | Go (Cobra) | Single binary distribution, cross-platform |
| Web UI | React + TypeScript | Component ecosystem, type safety |
| Primary database | PostgreSQL 16 | JSONB for flexible schemas, strong consistency |
| Cache / ephemeral | Redis 7 | Flag evaluation cache, session data, rate limiting |
| Message broker | NATS JetStream | Lightweight, high throughput, at-least-once delivery |
| Container orchestration | Kubernetes | Industry standard, extensible via operators |
| CI/CD | GitHub Actions | Native integration, broad ecosystem |
| Observability | OpenTelemetry → Prometheus/Grafana | Vendor-neutral, comprehensive |
| Object storage | S3-compatible | Artifacts, audit logs, backups |

---

## 3. Core Components — Detailed Design

### 3.1 Deploy Service

#### 3.1.1 Deployment Strategies

**Canary Deployments**
```
Phase 1: Route 1% of traffic to canary  →  monitor 5 min
Phase 2: Route 5% of traffic to canary  →  monitor 5 min
Phase 3: Route 25% of traffic to canary →  monitor 10 min
Phase 4: Route 50% of traffic to canary →  monitor 10 min
Phase 5: Route 100% — full promotion
```

Each phase has configurable:
- Traffic percentage
- Duration / hold time
- Health check criteria (error rate, latency p99, custom metrics)
- Auto-promote vs. manual gate

**Blue/Green Deployments**
- Maintain two identical environments
- Deploy to inactive environment
- Run smoke tests against inactive
- Switch traffic atomically via load balancer
- Keep old environment warm for instant rollback

**Rolling Updates**
- Update instances in batches (configurable batch size)
- Health check each batch before proceeding
- Configurable max-unavailable and max-surge

#### 3.1.2 Deploy Pipeline Data Model

```sql
CREATE TABLE deploy_pipelines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    name            TEXT NOT NULL,
    strategy        TEXT NOT NULL CHECK (strategy IN ('canary', 'blue_green', 'rolling')),
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     UUID NOT NULL REFERENCES deploy_pipelines(id),
    release_id      UUID NOT NULL REFERENCES releases(id),
    environment     TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN (
                        'pending', 'in_progress', 'paused',
                        'promoting', 'completed', 'rolling_back', 'failed'
                    )),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    initiated_by    UUID REFERENCES users(id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deployment_phases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID NOT NULL REFERENCES deployments(id),
    phase_number    INT NOT NULL,
    traffic_pct     INT NOT NULL CHECK (traffic_pct BETWEEN 0 AND 100),
    duration_secs   INT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN (
                        'pending', 'active', 'passed', 'failed', 'skipped'
                    )),
    health_snapshot JSONB,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);
```

#### 3.1.3 Deploy Service API

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/deployments` | Create a new deployment |
| GET | `/api/v1/deployments/:id` | Get deployment status |
| POST | `/api/v1/deployments/:id/promote` | Advance to next phase |
| POST | `/api/v1/deployments/:id/rollback` | Trigger rollback |
| POST | `/api/v1/deployments/:id/pause` | Pause deployment |
| POST | `/api/v1/deployments/:id/resume` | Resume paused deployment |
| GET | `/api/v1/deployments` | List deployments (filtered) |
| GET | `/api/v1/projects/:id/deployments/active` | Get active deployments for project |

### 3.2 Feature Flag Service

#### 3.2.1 Flag Evaluation Engine

The flag evaluation engine is the hot path — it must be fast and highly available.

**Evaluation flow:**
1. SDK requests flag value for a given context (user ID, attributes)
2. Local cache checked first (Redis-backed, sub-millisecond)
3. If cache miss, fetch from PostgreSQL, populate cache
4. Evaluate targeting rules in priority order
5. Return resolved value + metadata (reason, rule matched)

**Targeting rule types:**
- **Percentage rollout**: Hash user ID, deterministic bucketing
- **User targeting**: Explicit include/exclude lists
- **Attribute matching**: Rules based on user/context attributes (country, plan, version)
- **Segment targeting**: Reusable audience segments
- **Schedule**: Time-based activation/deactivation

#### 3.2.2 Flag Data Model

```sql
CREATE TABLE feature_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    flag_type       TEXT NOT NULL CHECK (flag_type IN ('boolean', 'string', 'number', 'json')),
    default_value   JSONB NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    tags            TEXT[] DEFAULT '{}',
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at     TIMESTAMPTZ,
    UNIQUE (project_id, key)
);

CREATE TABLE flag_targeting_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id         UUID NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment     TEXT NOT NULL,
    priority        INT NOT NULL,
    rule_type       TEXT NOT NULL CHECK (rule_type IN (
                        'percentage', 'user_target', 'attribute', 'segment', 'schedule'
                    )),
    conditions      JSONB NOT NULL,
    serve_value     JSONB NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE flag_evaluation_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id         UUID NOT NULL,
    flag_key        TEXT NOT NULL,
    environment     TEXT NOT NULL,
    context_hash    TEXT NOT NULL,
    result_value    JSONB NOT NULL,
    rule_matched    UUID,
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partitioned by month for efficient cleanup
CREATE INDEX idx_flag_eval_log_time ON flag_evaluation_log (evaluated_at);
```

#### 3.2.3 SDK Design

SDKs will be provided for major languages/platforms:

| SDK | Priority | Transport |
|-----|----------|-----------|
| Go | P0 | gRPC + HTTP fallback |
| Node.js / TypeScript | P0 | HTTP + SSE for streaming updates |
| Python | P1 | HTTP |
| Java / Kotlin | P1 | gRPC |
| React (client-side) | P0 | HTTP + SSE, React context provider |
| Ruby | P2 | HTTP |

**SDK responsibilities:**
- Local flag cache with configurable TTL
- Streaming updates via SSE / gRPC streams
- Offline mode with stale cache
- Context enrichment hooks
- Evaluation telemetry (opt-in)

#### 3.2.4 Feature Flag API

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/flags` | Create a new flag |
| GET | `/api/v1/flags` | List flags (with filtering/search) |
| GET | `/api/v1/flags/:key` | Get flag details |
| PUT | `/api/v1/flags/:key` | Update flag |
| DELETE | `/api/v1/flags/:key` | Archive flag |
| POST | `/api/v1/flags/:key/toggle` | Toggle flag on/off |
| POST | `/api/v1/flags/evaluate` | Evaluate flag(s) for context |
| POST | `/api/v1/flags/:key/rules` | Add targeting rule |
| PUT | `/api/v1/flags/:key/rules/:ruleId` | Update targeting rule |
| DELETE | `/api/v1/flags/:key/rules/:ruleId` | Delete targeting rule |

### 3.3 Release Tracker Service

#### 3.3.1 Release Lifecycle

```
 Commit → Build → Artifact → Deploy:Dev → Deploy:Staging → Deploy:Prod
   │        │        │            │              │               │
   └── PR ──┘        │            │              │               │
                     Tag          Health OK      Health OK       Health OK
                                  Auto-promote   Manual gate     Canary
```

#### 3.3.2 Release Data Model

```sql
CREATE TABLE releases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    version         TEXT NOT NULL,
    commit_sha      TEXT NOT NULL,
    branch          TEXT,
    changelog       TEXT,
    artifact_url    TEXT,
    status          TEXT NOT NULL CHECK (status IN (
                        'building', 'built', 'deploying', 'deployed',
                        'healthy', 'degraded', 'rolled_back'
                    )),
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, version)
);

CREATE TABLE release_environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id      UUID NOT NULL REFERENCES releases(id),
    environment     TEXT NOT NULL,
    status          TEXT NOT NULL,
    deployed_at     TIMESTAMPTZ,
    health_score    NUMERIC(5,2),
    UNIQUE (release_id, environment)
);
```

### 3.4 Rollback Controller

#### 3.4.1 Rollback State Machine

```
                    ┌───────────┐
                    │  HEALTHY  │
                    └─────┬─────┘
                          │ health degraded
                    ┌─────▼─────┐
              ┌─────│ EVALUATING│─────┐
              │     └───────────┘     │
         auto-heal            threshold breached
              │                       │
        ┌─────▼─────┐          ┌──────▼─────┐
        │ RECOVERED │          │ ROLLING    │
        └───────────┘          │ BACK       │
                               └──────┬─────┘
                                      │
                               ┌──────▼─────┐
                               │ ROLLED     │
                               │ BACK       │
                               └────────────┘
```

**Rollback triggers:**
- Error rate exceeds threshold (configurable, e.g., > 5% for 2 min)
- Latency p99 exceeds threshold
- Health check failures
- Manual trigger via API/CLI/UI
- External signal (PagerDuty incident auto-created)

**Rollback strategies:**
- Re-deploy previous known-good version
- Traffic shift back to blue environment
- Feature flag kill switch (disable flag that gates new code)

### 3.5 Health Monitor

#### 3.5.1 Health Score Computation

The health score is a composite metric (0–100) computed from weighted signals:

| Signal | Default Weight | Source |
|--------|---------------|--------|
| Error rate (5xx) | 30% | APM / metrics |
| Latency p99 | 20% | APM / metrics |
| Error tracking (new errors) | 20% | Sentry / Bugsnag integration |
| Custom metrics | 15% | User-defined Prometheus queries |
| Synthetic checks | 15% | Uptime / smoke tests |

```
health_score = Σ (signal_score_i × weight_i)

where signal_score_i = 100 × max(0, 1 - (current_value / threshold_value))
```

If `health_score < 50` for the configured evaluation window → trigger rollback evaluation.

#### 3.5.2 Integration Points

- **Prometheus**: Pull metrics via PromQL queries
- **Sentry**: Webhook for new issues, error counts per release
- **Datadog**: API integration for metrics and monitors
- **PagerDuty**: Bi-directional (receive incidents, trigger incidents)
- **Custom**: HTTP endpoint polling with configurable response parsing

---

## 4. Infrastructure & Operations

### 4.1 Deployment Architecture

```
                    ┌─────────────────┐
                    │   CloudFlare    │
                    │   CDN / WAF     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Load Balancer  │
                    │  (L7 / Ingress) │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼────┐  ┌─────▼────┐  ┌──────▼────┐
        │ API Pod  │  │ API Pod  │  │ API Pod   │
        │ (Go)     │  │ (Go)     │  │ (Go)      │
        └─────┬────┘  └─────┬────┘  └──────┬────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼────┐  ┌─────▼────┐  ┌──────▼────┐
        │PostgreSQL│  │  Redis   │  │   NATS    │
        │ Primary  │  │ Cluster  │  │  Cluster  │
        │ + Replica│  │          │  │           │
        └──────────┘  └──────────┘  └───────────┘
```

### 4.2 Kubernetes Resources

Each service runs as a Kubernetes Deployment with:
- Horizontal Pod Autoscaler (HPA) based on CPU/memory and custom metrics
- Pod Disruption Budgets (PDB) for safe rollouts
- Resource requests and limits
- Readiness and liveness probes
- Service mesh sidecar (optional, for mTLS and observability)

### 4.3 Environment Strategy

| Environment | Purpose | Data | Deployment |
|------------|---------|------|------------|
| `dev` | Active development, integration | Synthetic / seeded | Auto on merge to `main` |
| `staging` | Pre-prod validation, QA | Anonymized prod mirror | Auto after dev health check |
| `prod` | Production | Real | Canary with manual promotion gate |

### 4.4 Observability Stack

```
Application Code
    │
    ├── OpenTelemetry SDK (traces, metrics, logs)
    │       │
    │       ▼
    │   OTel Collector
    │       │
    │       ├──► Prometheus (metrics)
    │       ├──► Jaeger / Tempo (traces)
    │       └──► Loki (logs)
    │
    └── Structured logging (JSON)
            │
            ▼
        Grafana (dashboards, alerts)
```

**Key dashboards:**
- Deployment overview: active deploys, success rate, mean time to deploy
- Feature flag usage: evaluation volume, flag age, stale flags
- Release health: per-release health scores, rollback frequency
- System health: API latency, error rates, resource utilization

### 4.5 Security

| Area | Approach |
|------|----------|
| Authentication | OAuth 2.0 / OIDC (support GitHub, Google, Okta SSO) |
| Authorization | RBAC with project-level and environment-level scopes |
| API keys | Scoped API keys for SDK and CI/CD integration |
| Secrets | External secrets manager (Vault / AWS Secrets Manager) |
| Audit log | Immutable append-only log of all mutations |
| Data encryption | TLS in transit, AES-256 at rest |
| Network | mTLS between services, network policies in K8s |

#### 4.5.1 RBAC Model

```
Roles:
  - org:owner       → full access to all projects and settings
  - org:admin       → manage projects, users, and billing
  - project:admin   → full access within a project
  - project:editor  → create/edit deploys, flags, releases
  - project:viewer  → read-only access
  - environment:deployer → deploy to specific environments only
```

---

## 5. API Design Principles

### 5.1 REST API Conventions

- **Versioning**: URL path (`/api/v1/...`)
- **Pagination**: Cursor-based (keyset pagination) for performance
- **Filtering**: Query parameters with field-level operators (`?status=active&created_after=2025-01-01`)
- **Sorting**: `?sort=created_at&order=desc`
- **Error format**: RFC 7807 Problem Details

```json
{
  "type": "https://deploysentry.io/errors/validation",
  "title": "Validation Error",
  "status": 422,
  "detail": "Flag key must be lowercase alphanumeric with hyphens",
  "instance": "/api/v1/flags",
  "errors": [
    {
      "field": "key",
      "message": "must match pattern ^[a-z0-9-]+$"
    }
  ]
}
```

### 5.2 Rate Limiting

| Tier | Requests/min | Burst |
|------|-------------|-------|
| Free | 60 | 10 |
| Pro | 600 | 50 |
| Enterprise | 6000 | 200 |
| Flag evaluation (SDK) | 10000 | 500 |

Rate limiting implemented via Redis sliding window with `X-RateLimit-*` response headers.

### 5.3 Webhook Events

```json
{
  "id": "evt_abc123",
  "type": "deployment.phase.completed",
  "created_at": "2026-02-21T10:30:00Z",
  "project_id": "proj_xyz",
  "data": {
    "deployment_id": "dep_456",
    "phase_number": 2,
    "traffic_pct": 25,
    "health_score": 97.5,
    "status": "passed"
  }
}
```

**Event types:**
- `deployment.created`, `deployment.phase.completed`, `deployment.completed`, `deployment.failed`
- `deployment.rollback.initiated`, `deployment.rollback.completed`
- `flag.created`, `flag.updated`, `flag.toggled`, `flag.archived`
- `release.created`, `release.promoted`, `release.health.degraded`
- `health.alert.triggered`, `health.alert.resolved`

---

## 6. CLI Design

### 6.1 Command Structure

```
deploysentry
├── auth
│   ├── login           # Authenticate via browser OAuth flow
│   ├── logout          # Clear local credentials
│   └── status          # Show current auth status
├── deploy
│   ├── create          # Create a new deployment
│   ├── status          # Show deployment status
│   ├── promote         # Advance canary to next phase
│   ├── rollback        # Trigger rollback
│   ├── pause           # Pause active deployment
│   ├── resume          # Resume paused deployment
│   ├── list            # List recent deployments
│   └── logs            # Stream deployment logs
├── flags
│   ├── create          # Create a new feature flag
│   ├── list            # List flags with filtering
│   ├── get             # Get flag details
│   ├── toggle          # Toggle flag on/off
│   ├── update          # Update flag configuration
│   ├── evaluate        # Test flag evaluation locally
│   └── archive         # Archive a flag
├── releases
│   ├── create          # Create a new release
│   ├── list            # List releases
│   ├── status          # Show release status across environments
│   └── promote         # Promote release to next environment
├── projects
│   ├── create          # Create a new project
│   ├── list            # List projects
│   └── config          # View/update project configuration
└── config
    ├── init            # Initialize project config (.deploysentry.yml)
    ├── set             # Set configuration value
    └── get             # Get configuration value
```

### 6.2 Example Workflows

**Deploy a new release with canary:**
```bash
# Create release from current commit
deploysentry releases create --version v1.2.3 --commit $(git rev-parse HEAD)

# Deploy to staging
deploysentry deploy create --release v1.2.3 --env staging --strategy canary

# Watch deployment progress
deploysentry deploy status --watch

# Promote to production after staging is healthy
deploysentry deploy create --release v1.2.3 --env prod --strategy canary

# Monitor canary phases
deploysentry deploy status --watch

# Manual promote past each gate
deploysentry deploy promote
```

**Manage feature flags:**
```bash
# Create a new flag
deploysentry flags create --key new-checkout-flow --type boolean --default false

# Enable for 10% of users
deploysentry flags update new-checkout-flow \
  --add-rule '{"type":"percentage","value":10,"serve":true}'

# Toggle on globally
deploysentry flags toggle new-checkout-flow --on

# Check flag evaluation
deploysentry flags evaluate new-checkout-flow --context '{"user_id":"123"}'
```

---

## 7. Data Model — Complete Schema

### 7.1 Core Entities

```sql
-- Organizations
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    plan            TEXT NOT NULL DEFAULT 'free',
    settings        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    avatar_url      TEXT,
    auth_provider   TEXT NOT NULL,
    auth_provider_id TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

-- Organization membership
CREATE TABLE org_members (
    org_id          UUID NOT NULL REFERENCES organizations(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    role            TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);

-- Projects
CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    description     TEXT,
    repo_url        TEXT,
    settings        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

-- Project membership
CREATE TABLE project_members (
    project_id      UUID NOT NULL REFERENCES projects(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    role            TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'viewer', 'deployer')),
    PRIMARY KEY (project_id, user_id)
);

-- Environments
CREATE TABLE environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    is_production   BOOLEAN NOT NULL DEFAULT false,
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    settings        JSONB NOT NULL DEFAULT '{}',
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

-- API Keys
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    name            TEXT NOT NULL,
    key_hash        TEXT NOT NULL UNIQUE,
    key_prefix      TEXT NOT NULL,  -- first 8 chars for identification
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    environment     TEXT,  -- optional environment restriction
    created_by      UUID REFERENCES users(id),
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit log
CREATE TABLE audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    project_id      UUID,
    user_id         UUID,
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     UUID,
    old_value       JSONB,
    new_value       JSONB,
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partition audit_log by month for manageability
CREATE INDEX idx_audit_log_time ON audit_log (created_at);
CREATE INDEX idx_audit_log_org ON audit_log (org_id, created_at);
```

### 7.2 Webhook Configuration

```sql
CREATE TABLE webhook_endpoints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    url             TEXT NOT NULL,
    secret          TEXT NOT NULL,  -- for HMAC signature verification
    events          TEXT[] NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id     UUID NOT NULL REFERENCES webhook_endpoints(id),
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    response_status INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ,
    attempts        INT NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 8. Project Milestones

### Phase 1 — Foundation (Weeks 1–4)

**Goal**: Core platform infrastructure and basic deployment pipeline.

| Week | Deliverable |
|------|------------|
| 1 | Project scaffolding: Go service structure, DB migrations, CI pipeline, dev environment (Docker Compose) |
| 2 | Auth system (OAuth 2.0 with GitHub), user/org/project CRUD, RBAC middleware |
| 3 | Deploy Service v1: rolling deployments to Kubernetes, basic health checks |
| 4 | Release Tracker v1: release creation, environment promotion, CLI `deploy` and `releases` commands |

**Exit criteria:**
- [ ] Can authenticate via GitHub OAuth
- [ ] Can create org/project and invite members
- [ ] Can create a release and deploy it (rolling strategy) to a K8s cluster
- [ ] Can view deploy status in CLI and API
- [ ] CI/CD pipeline runs tests, linting, and builds Docker images

### Phase 2 — Feature Flags & Advanced Deploys (Weeks 5–8)

**Goal**: Feature flag system and canary/blue-green deployment support.

| Week | Deliverable |
|------|------------|
| 5 | Feature Flag Service: CRUD, evaluation engine, Redis caching |
| 6 | Targeting rules engine: percentage rollouts, user targeting, attribute matching |
| 7 | Canary deployment strategy with phase-based promotion |
| 8 | Go SDK for feature flags, flag evaluation API, CLI `flags` commands |

**Exit criteria:**
- [ ] Can create and manage feature flags via API and CLI
- [ ] Can evaluate flags with targeting rules (percentage, user, attribute)
- [ ] Go SDK can evaluate flags with local caching and streaming updates
- [ ] Canary deployments work with configurable phases
- [ ] Automated promotion based on health thresholds

### Phase 3 — Health & Observability (Weeks 9–12)

**Goal**: Health monitoring, automated rollbacks, and observability integration.

| Week | Deliverable |
|------|------------|
| 9 | Health Monitor: Prometheus integration, health score computation |
| 10 | Rollback Controller: automated rollback triggers, rollback execution |
| 11 | Notification Service: Slack and webhook integrations |
| 12 | Web UI v1: deployment dashboard, flag management UI, release timeline |

**Exit criteria:**
- [ ] Health scores computed from Prometheus metrics
- [ ] Automated rollback triggers when health degrades
- [ ] Slack notifications for deploy events
- [ ] Web dashboard shows deployment status, flags, and releases
- [ ] End-to-end flow: commit → release → canary deploy → health monitoring → auto-promote/rollback

### Phase 4 — SDKs, Polish & GA (Weeks 13–16)

**Goal**: Multi-language SDK support, production hardening, documentation.

| Week | Deliverable |
|------|------------|
| 13 | Node.js/TypeScript SDK, React SDK with context provider |
| 14 | Python SDK, Java SDK |
| 15 | Blue/green deployment strategy, environment approval gates |
| 16 | Documentation site, API reference, onboarding flow, production hardening |

**Exit criteria:**
- [ ] SDKs available for Go, Node.js, Python, Java, React
- [ ] Blue/green deployments functional
- [ ] Documentation site live with API reference and guides
- [ ] Load testing completed (target: 10K flag evaluations/sec, 100 concurrent deployments)
- [ ] Security audit completed

---

## 9. Testing Strategy

### 9.1 Test Pyramid

```
         ┌─────────────┐
         │   E2E Tests  │   ~10% — Critical user flows
         │  (Playwright) │
         ├──────────────┤
         │ Integration   │   ~30% — Service interactions, DB queries
         │ Tests (Go)    │
         ├──────────────┤
         │  Unit Tests   │   ~60% — Business logic, evaluation engine
         │  (Go, Jest)   │
         └──────────────┘
```

### 9.2 Test Categories

| Category | Tool | Target |
|----------|------|--------|
| Unit tests | Go `testing` + `testify` | Business logic, flag evaluation, health score calculation |
| Integration tests | Go `testing` + `testcontainers` | Database queries, Redis cache, NATS messaging |
| API tests | Go `httptest` + `testify` | HTTP handlers, middleware, auth |
| SDK tests | Language-specific testing frameworks | SDK evaluation logic, caching, streaming |
| E2E tests | Playwright | Web UI critical flows |
| Load tests | k6 | Flag evaluation throughput, API latency under load |
| Contract tests | Pact | SDK ↔ API compatibility |

### 9.3 CI Pipeline

```yaml
# .github/workflows/ci.yml (conceptual)
stages:
  - lint:
      - golangci-lint
      - eslint + prettier (UI)
      - sqlfluff (migrations)
  - test:
      - unit tests (parallel by package)
      - integration tests (with testcontainers)
  - build:
      - Go binaries (linux/amd64, darwin/amd64, darwin/arm64)
      - Docker images
      - UI static bundle
  - deploy-dev:
      - Auto-deploy to dev on main merge
      - Run smoke tests
```

---

## 10. Non-Functional Requirements

### 10.1 Performance Targets

| Metric | Target |
|--------|--------|
| Flag evaluation (SDK, cached) | < 1ms p99 |
| Flag evaluation (API) | < 10ms p99 |
| Deployment creation API | < 200ms p99 |
| Dashboard page load | < 2s (LCP) |
| Deployment status update | < 500ms (SSE/WebSocket) |
| System availability | 99.9% uptime |

### 10.2 Scalability Targets

| Dimension | Target (GA) | Target (12 months) |
|-----------|-------------|-------------------|
| Flag evaluations | 10K/sec | 100K/sec |
| Concurrent deployments | 100 | 1000 |
| Feature flags per project | 1,000 | 10,000 |
| Projects per org | 50 | 500 |
| Users per org | 100 | 1,000 |
| Webhook deliveries | 1K/min | 10K/min |

### 10.3 Data Retention

| Data Type | Retention |
|-----------|-----------|
| Audit logs | 2 years (archived to S3 after 90 days) |
| Flag evaluation logs | 30 days (sampled at 10% after 7 days) |
| Deployment history | Indefinite |
| Webhook delivery logs | 30 days |
| Metrics (Prometheus) | 90 days (downsampled after 30 days) |

---

## 11. Project Structure

```
deploysentry/
├── cmd/
│   ├── api/                    # API server entrypoint
│   │   └── main.go
│   └── cli/                    # CLI entrypoint
│       └── main.go
├── internal/
│   ├── auth/                   # Authentication & authorization
│   │   ├── oauth.go
│   │   ├── middleware.go
│   │   └── rbac.go
│   ├── deploy/                 # Deploy service
│   │   ├── service.go
│   │   ├── strategies/
│   │   │   ├── canary.go
│   │   │   ├── bluegreen.go
│   │   │   └── rolling.go
│   │   ├── handler.go
│   │   └── repository.go
│   ├── flags/                  # Feature flag service
│   │   ├── service.go
│   │   ├── evaluator.go
│   │   ├── targeting.go
│   │   ├── handler.go
│   │   └── repository.go
│   ├── releases/               # Release tracker
│   │   ├── service.go
│   │   ├── handler.go
│   │   └── repository.go
│   ├── health/                 # Health monitor
│   │   ├── monitor.go
│   │   ├── scorer.go
│   │   └── integrations/
│   │       ├── prometheus.go
│   │       ├── sentry.go
│   │       └── datadog.go
│   ├── rollback/               # Rollback controller
│   │   ├── controller.go
│   │   └── strategies.go
│   ├── notifications/          # Notification service
│   │   ├── service.go
│   │   ├── slack.go
│   │   ├── webhook.go
│   │   └── email.go
│   ├── platform/               # Shared platform code
│   │   ├── database/
│   │   ├── cache/
│   │   ├── messaging/
│   │   ├── config/
│   │   └── middleware/
│   └── models/                 # Shared domain models
│       ├── deployment.go
│       ├── flag.go
│       ├── release.go
│       ├── user.go
│       └── organization.go
├── migrations/                 # SQL migrations (golang-migrate)
│   ├── 001_create_organizations.up.sql
│   ├── 001_create_organizations.down.sql
│   └── ...
├── sdk/
│   ├── go/                     # Go SDK
│   ├── node/                   # Node.js SDK
│   ├── python/                 # Python SDK
│   └── java/                   # Java SDK
├── web/                        # React web UI
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── api/
│   │   └── store/
│   ├── package.json
│   └── tsconfig.json
├── deploy/                     # Deployment manifests
│   ├── kubernetes/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── dev/
│   │       ├── staging/
│   │       └── prod/
│   ├── docker/
│   │   ├── Dockerfile.api
│   │   └── Dockerfile.web
│   └── docker-compose.yml      # Local dev environment
├── docs/                       # Documentation
│   └── PROJECT_PLAN.md
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── .deploysentry.yml           # Project config example
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 12. Development Workflow

### 12.1 Local Development

```bash
# Clone and setup
git clone https://github.com/your-org/deploysentry.git
cd deploysentry

# Start dependencies (PostgreSQL, Redis, NATS)
make dev-up  # docker-compose up -d

# Run migrations
make migrate-up

# Start API server (with hot reload)
make run-api

# Start web UI (with hot reload)
make run-web

# Run tests
make test           # all tests
make test-unit      # unit tests only
make test-int       # integration tests only

# Build
make build          # build all binaries
make docker-build   # build Docker images
```

### 12.2 Branching Strategy

- `main` — always deployable, auto-deploys to dev
- `release/*` — release candidates, auto-deploys to staging
- `feature/*` — feature branches, PR-based development
- Squash merges to main for clean history

### 12.3 Code Review Requirements

- 1 approval required for non-critical changes
- 2 approvals required for: database migrations, auth changes, deploy strategies
- CI must pass (lint + test) before merge
- No direct pushes to `main`

---

## 13. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Flag evaluation latency under load | Medium | High | Redis caching, SDK local cache, connection pooling, load testing early |
| Deployment orchestration failure mid-canary | Medium | High | Deployment state machine with idempotent operations, automatic rollback |
| Breaking SDK changes | Low | High | Semantic versioning, contract tests, deprecation policy |
| Data loss during rollback | Low | Critical | Immutable deployment records, audit logging, backup strategy |
| Health monitor false positives triggering unnecessary rollbacks | Medium | Medium | Configurable thresholds, evaluation windows, manual override capability |
| Third-party integration downtime (Prometheus, Slack) | Medium | Low | Circuit breakers, graceful degradation, retry with backoff |
| Security breach via API key compromise | Low | Critical | Key rotation, scoped permissions, audit logging, rate limiting |

---

## 14. Success Metrics

### 14.1 Platform Metrics

| Metric | Definition | Target |
|--------|-----------|--------|
| Deployment success rate | Successful deploys / total deploys | > 95% |
| Mean time to deploy (MTTD) | Commit merge to production traffic | < 30 min |
| Mean time to rollback (MTTR) | Degradation detected to rollback complete | < 5 min |
| Flag evaluation availability | Successful evaluations / total requests | > 99.99% |
| False positive rollback rate | Unnecessary rollbacks / total rollbacks | < 5% |

### 14.2 Adoption Metrics

| Metric | Target (3 months post-GA) |
|--------|--------------------------|
| Active projects | 50 |
| Daily deployments | 200 |
| Feature flags in use | 500 |
| SDK installations | 1000 |
| Active users | 200 |

---

## 15. Open Questions & Decisions

| # | Question | Options | Decision | Status |
|---|----------|---------|----------|--------|
| 1 | Self-hosted vs. SaaS-first? | SaaS-first with self-hosted later / Self-hosted from day 1 | SaaS-first | Decided |
| 2 | gRPC vs. REST for SDK communication? | gRPC primary / REST primary / Both | REST + SSE primary, gRPC for Go SDK | Decided |
| 3 | Multi-tenancy model? | Shared DB with row-level isolation / Schema-per-tenant / DB-per-tenant | Shared DB with row-level isolation | Decided |
| 4 | Message broker choice? | NATS JetStream / Kafka / RabbitMQ | NATS JetStream | Decided |
| 5 | Frontend state management? | Redux Toolkit / Zustand / React Query only | TBD | Open |
| 6 | Pricing model? | Per-seat / Per-flag-evaluation / Per-deployment | TBD | Open |

---

*This document is the authoritative reference for the DeploySentry Sentinel project. It should be updated as architectural decisions are made and requirements evolve.*
