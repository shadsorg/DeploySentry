# DeploySentry

Deploy release and feature flag management platform. Orchestrate safe deployments with canary releases, blue/green strategies, and automated rollbacks. Manage feature flags with granular targeting, percentage rollouts, and kill switches.

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Core Features](#core-features)
  - [Deployment Strategies](#deployment-strategies)
  - [Feature Flags](#feature-flags)
  - [Rollback System](#rollback-system)
  - [Health Monitoring](#health-monitoring)
  - [Notifications](#notifications)
- [CLI Reference](#cli-reference)
- [API Reference](#api-reference)
- [SDKs](#sdks)
- [Configuration](#configuration)
- [Development](#development)
- [Project Structure](#project-structure)

---

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.22+
- Node.js 18+ (for the web UI)
- [golang-migrate](https://github.com/golang-migrate/migrate) (for database migrations)

### 1. Clone and configure

```bash
git clone https://github.com/shadsorg/deploysentry.git
cd deploysentry
cp .env.example .env
```

### 2. Start infrastructure

```bash
# Start PostgreSQL, Redis, and NATS
make dev-up

# Run database migrations
make migrate-up
```

### 3. Start the API server

```bash
make run-api
# API available at http://localhost:8080
```

### 4. Start the web UI (optional)

```bash
make run-web
# Web UI available at http://localhost:5173
```

### 5. Install the CLI

```bash
go install ./cmd/cli
deploysentry config init
deploysentry auth login
```

### 6. Create your first deployment

```bash
# Create a project
deploysentry projects create --name "my-app" --org "my-org"

# Deploy with canary strategy
deploysentry deploy create \
  --project my-app \
  --env production \
  --strategy canary \
  --image my-app:v1.2.0

# Check deployment status
deploysentry deploy status
```

### 7. Create your first feature flag

```bash
deploysentry flags create \
  --project my-app \
  --key "enable-dark-mode" \
  --name "Enable Dark Mode" \
  --env staging

# Toggle it on
deploysentry flags toggle --key "enable-dark-mode" --env staging
```

---

## Architecture

```
                    ┌──────────────────────────────────────────────┐
                    │            DeploySentry Platform             │
                    ├──────────┬──────────┬────────────┬──────────┤
                    │  Web UI  │ Mobile   │  CLI Tool  │   SDKs   │
                    │ (React)  │(Flutter) │ (Go/Cobra) │(7 langs) │
                    ├──────────┴──────────┴────────────┴──────────┤
                    │         REST API (Go / Gin)                 │
                    │    Auth  ·  Rate Limiting  ·  CORS          │
                    ├─────────────────────────────────────────────┤
                    │              Core Services                  │
                    │  ┌─────────┐ ┌─────────┐ ┌──────────────┐  │
                    │  │ Deploy  │ │ Feature │ │   Release    │  │
                    │  │ Service │ │  Flags  │ │   Tracker    │  │
                    │  └─────────┘ └─────────┘ └──────────────┘  │
                    │  ┌─────────┐ ┌─────────┐ ┌──────────────┐  │
                    │  │Rollback │ │ Health  │ │Notifications │  │
                    │  │Controller│ │Monitor │ │   Service    │  │
                    │  └─────────┘ └─────────┘ └──────────────┘  │
                    ├─────────────────────────────────────────────┤
                    │            Infrastructure                   │
                    │  PostgreSQL  ·  Redis  ·  NATS JetStream   │
                    └─────────────────────────────────────────────┘
```

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Backend | Go 1.22+ / Gin | API server, business logic |
| Web UI | React 18 + TypeScript / Vite | Dashboard and management console |
| Mobile | Flutter / Dart | iOS and Android management app |
| CLI | Go / Cobra | Developer workflow tool |
| Database | PostgreSQL 16 | Primary data store with JSONB |
| Cache | Redis 7 | Flag evaluation cache, rate limiting |
| Messaging | NATS JetStream | Real-time push updates, event streaming |
| Auth | JWT + OAuth2 | Token-based auth with SSO support |
| Observability | OpenTelemetry | Metrics, traces, logs |

---

## Core Features

### Deployment Strategies

#### Canary Deployments

Gradually shift traffic to the new version while monitoring health metrics.

```yaml
# .deploysentry.yml
deploy:
  strategy: "canary"
  canary:
    initial_weight: 5      # Start with 5% traffic
    increment: 10           # Increase by 10% per phase
    interval: "5m"          # Wait 5 minutes between phases
    auto_promote: true      # Auto-promote if healthy
```

Traffic progression: 5% -> 15% -> 25% -> 35% -> ... -> 100%

Health checks run at each phase. If error rate exceeds thresholds, the deployment is automatically rolled back.

#### Blue/Green Deployments

Deploy to an inactive environment, then atomically switch traffic.

```bash
deploysentry deploy create --strategy blue-green --image my-app:v2.0.0
# Validates the green environment, then switches traffic
# Instant rollback by switching back to blue
```

#### Rolling Updates

Update instances in configurable batches with health checks between each batch.

```bash
deploysentry deploy create --strategy rolling --batch-size 2 --max-unavailable 1
```

### Feature Flags

The feature flag system supports multiple flag types with a priority-ordered evaluation engine.

**Flag types:**
- **Boolean** -- simple on/off toggles
- **Percentage rollout** -- deterministic user bucketing (consistent experience per user)
- **User targeting** -- explicit include/exclude lists
- **Attribute matching** -- target by country, plan, app version, etc.
- **Segment targeting** -- reusable audience definitions
- **Schedule-based** -- activate flags at specific times

**Evaluation flow:**

1. Check kill switch (overrides everything)
2. Check explicit user targeting (include/exclude lists)
3. Evaluate targeting rules in priority order
4. Apply percentage rollout (deterministic hash of user ID)
5. Fall back to default value

Flag evaluations are cached in Redis for sub-millisecond latency. Real-time updates are pushed via NATS to all connected SDKs.

### Rollback System

Automated rollback with a state machine:

```
HEALTHY -> EVALUATING -> ROLLING_BACK -> ROLLED_BACK
```

**Automatic triggers (configurable):**
- Error rate exceeds 5%
- Latency p99 exceeds 2 seconds
- Health check failures exceed threshold

**Rollback strategies:**
- **Instant** -- immediately revert to previous version
- **Gradual** -- reverse the canary progression
- **Canary revert** -- shift traffic back through canary phases

```bash
# Manual rollback
deploysentry deploy rollback --deployment-id <id>

# Check rollback history
deploysentry deploy status --deployment-id <id>
```

### Health Monitoring

The health monitor aggregates signals from APM integrations to compute a deployment health score.

**Integrations:**
- Error tracking (Sentry, Datadog, etc.)
- Latency metrics (p50, p95, p99)
- Custom metrics via webhooks
- HTTP health check endpoints

```yaml
deploy:
  health_check:
    path: "/health"
    expected_status: 200
    interval: "10s"
    threshold: 3    # consecutive successes required
```

### Notifications

Get alerted on deployment and flag events through multiple channels.

**Channels:** Slack, PagerDuty, Email, Webhooks

**Events:**
- `deploy.started`, `deploy.completed`, `deploy.failed`, `deploy.rolled_back`
- `flag.changed`, `flag.created`, `flag.deleted`

```yaml
notifications:
  slack_webhook: "https://hooks.slack.com/services/T.../B.../xxx"
  events:
    - "deploy.started"
    - "deploy.completed"
    - "deploy.failed"
    - "deploy.rolled_back"
    - "flag.changed"
```

---

## CLI Reference

```
deploysentry config init                  Initialize project configuration
deploysentry auth login                   Authenticate with DeploySentry
deploysentry auth logout                  Clear stored credentials

deploysentry deploy create                Start a new deployment
deploysentry deploy list                  List deployments
deploysentry deploy status                Check deployment status
deploysentry deploy promote               Advance canary to next phase
deploysentry deploy rollback              Trigger rollback
deploysentry deploy pause                 Pause an active deployment
deploysentry deploy resume                Resume a paused deployment

deploysentry flags list                   List feature flags
deploysentry flags create                 Create a new flag
deploysentry flags toggle                 Toggle a flag on/off
deploysentry flags rule add               Add a targeting rule

deploysentry releases list                List releases
deploysentry releases promote             Promote a release to next environment

deploysentry projects list                List projects
deploysentry projects create              Create a new project

deploysentry version                      Print version information
deploysentry completion bash|zsh|fish     Generate shell completions
```

---

## API Reference

### Deployments

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/deployments` | Create a new deployment |
| `GET` | `/api/v1/deployments/:id` | Get deployment details |
| `GET` | `/api/v1/deployments` | List deployments |
| `POST` | `/api/v1/deployments/:id/promote` | Promote to next phase |
| `POST` | `/api/v1/deployments/:id/rollback` | Trigger rollback |
| `POST` | `/api/v1/deployments/:id/pause` | Pause deployment |
| `POST` | `/api/v1/deployments/:id/resume` | Resume deployment |
| `GET` | `/api/v1/projects/:id/deployments/active` | Get active deployments |

### Feature Flags

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/flags` | Create a flag |
| `GET` | `/api/v1/flags/:id` | Get flag details |
| `POST` | `/api/v1/flags/:id/toggle` | Toggle a flag |
| `POST` | `/api/v1/flags/:id/rules` | Add targeting rule |
| `GET` | `/api/v1/flags/evaluate` | Evaluate a single flag |
| `POST` | `/api/v1/flags/batch-evaluate` | Evaluate multiple flags |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Full health check |
| `GET` | `/ready` | Readiness probe |

**Authentication:** All API endpoints (except `/health` and `/ready`) require either a JWT token or API key:

```bash
# Using API key
curl -H "Authorization: ApiKey ds_key_xxx" http://localhost:8080/api/v1/flags

# Using JWT
curl -H "Authorization: Bearer <jwt-token>" http://localhost:8080/api/v1/flags
```

---

## SDKs

DeploySentry provides official SDKs for 7 platforms. All SDKs support feature flag evaluation, local caching, real-time push updates via NATS, and offline mode.

| SDK | Language | Install | Source |
|-----|----------|---------|--------|
| **Go** | Go | `go get github.com/deploysentry/deploysentry/sdk/go` | [`sdk/go`](sdk/go) |
| **Node.js** | TypeScript / JavaScript | `npm install @deploysentry/sdk` | [`sdk/node`](sdk/node) |
| **Python** | Python 3.9+ | `pip install deploysentry` | [`sdk/python`](sdk/python) |
| **Java** | Java | Maven / Gradle | [`sdk/java`](sdk/java) |
| **Ruby** | Ruby | `gem install deploysentry` | [`sdk/ruby`](sdk/ruby) |
| **React** | TypeScript / React | `npm install @deploysentry/react` | [`sdk/react`](sdk/react) |
| **Flutter** | Dart / Flutter | `deploysentry_flutter` (pub.dev) | [`sdk/flutter`](sdk/flutter) |

### Go SDK

```go
import ds "github.com/deploysentry/deploysentry/sdk/go"

client, err := ds.NewClient(ds.Config{
    APIKey:      "ds_key_xxx",
    ProjectID:   "project-uuid",
    Environment: "production",
})
defer client.Close()

enabled, err := client.IsEnabled("enable-dark-mode", ds.Context{
    UserID: "user-123",
    Attributes: map[string]interface{}{
        "plan": "premium",
    },
})
```

### Node.js SDK

```typescript
import { DeploySentry } from '@deploysentry/sdk';

const client = new DeploySentry({
  apiKey: 'ds_key_xxx',
  projectId: 'project-uuid',
  environment: 'production',
});

const enabled = await client.isEnabled('enable-dark-mode', {
  userId: 'user-123',
  attributes: { plan: 'premium' },
});
```

### Python SDK

```python
from deploysentry import DeploySentryClient

client = DeploySentryClient(
    api_key="ds_key_xxx",
    project_id="project-uuid",
    environment="production",
)

enabled = client.is_enabled("enable-dark-mode", context={
    "user_id": "user-123",
    "attributes": {"plan": "premium"},
})
```

### React SDK

```tsx
import { DeploySentryProvider, useFlag } from '@deploysentry/react';

function App() {
  return (
    <DeploySentryProvider
      apiKey="ds_key_xxx"
      projectId="project-uuid"
      environment="production"
      user={{ id: 'user-123', attributes: { plan: 'premium' } }}
    >
      <MyComponent />
    </DeploySentryProvider>
  );
}

function MyComponent() {
  const darkMode = useFlag('enable-dark-mode');
  return <div className={darkMode ? 'dark' : 'light'}>...</div>;
}
```

### Flutter SDK

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

final client = DeploySentryClient(
  apiKey: 'ds_key_xxx',
  projectId: 'project-uuid',
  environment: 'production',
);

final enabled = await client.isEnabled('enable-dark-mode', context: {
  'userId': 'user-123',
  'attributes': {'plan': 'premium'},
});
```

For detailed SDK integration guides, see [docs/sdk-onboarding.md](docs/sdk-onboarding.md).

---

## Configuration

### Environment Variables

All environment variables use the `DS_` prefix. Copy `.env.example` to `.env` to get started.

| Variable | Default | Description |
|----------|---------|-------------|
| `DS_SERVER_HOST` | `0.0.0.0` | API server bind address |
| `DS_SERVER_PORT` | `8080` | API server port |
| `DS_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `DS_DATABASE_PORT` | `5432` | PostgreSQL port |
| `DS_DATABASE_USER` | `deploysentry` | PostgreSQL user |
| `DS_DATABASE_PASSWORD` | `deploysentry` | PostgreSQL password |
| `DS_DATABASE_NAME` | `deploysentry` | PostgreSQL database name |
| `DS_DATABASE_SSL_MODE` | `disable` | PostgreSQL SSL mode |
| `DS_REDIS_HOST` | `localhost` | Redis host |
| `DS_REDIS_PORT` | `6379` | Redis port |
| `DS_NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `DS_AUTH_JWT_SECRET` | (change in prod) | JWT signing secret |
| `DS_AUTH_JWT_EXPIRATION` | `24h` | JWT token lifetime |
| `DS_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `DS_LOG_FORMAT` | `json` | Log format (json, text) |

### Project Configuration File

Place `.deploysentry.yml` in your project root or at `~/.deploysentry.yml`:

```yaml
org: "my-organization"
project: "my-project"
env: "staging"

api:
  url: "https://api.deploysentry.example.com"

deploy:
  strategy: "canary"
  canary:
    initial_weight: 5
    increment: 10
    interval: "5m"
    auto_promote: true
  health_check:
    path: "/health"
    expected_status: 200
    interval: "10s"
    threshold: 3

flags:
  default_env: "staging"
  stale_threshold: "30d"

notifications:
  events:
    - "deploy.started"
    - "deploy.completed"
    - "deploy.failed"
    - "deploy.rolled_back"
    - "flag.changed"
```

---

## Development

### Make Targets

| Command | Description |
|---------|-------------|
| `make dev-up` | Start infrastructure (PostgreSQL, Redis, NATS) |
| `make dev-down` | Stop infrastructure |
| `make migrate-up` | Run database migrations |
| `make migrate-down` | Roll back last migration |
| `make run-api` | Start API server |
| `make run-web` | Start web UI dev server |
| `make test` | Run all tests with race detection |
| `make test-unit` | Run unit tests only |
| `make test-int` | Run integration tests only |
| `make build` | Build binaries for Linux and macOS |
| `make docker-build` | Build Docker images |
| `make lint` | Run Go linters |
| `make clean` | Remove build artifacts |

### Docker Compose Services

```
PostgreSQL:  localhost:5432  (user: deploysentry, password: deploysentry)
Redis:       localhost:6379
NATS:        localhost:4222  (client)  /  localhost:8222  (monitoring)
```

### Running Tests

```bash
# All tests
make test

# Unit tests only (fast)
make test-unit

# Integration tests (requires running infrastructure)
make test-int
```

### Building

```bash
# Build for all platforms
make build
# Outputs to bin/:
#   deploysentry-linux-amd64, deploysentry-cli-linux-amd64
#   deploysentry-darwin-amd64, deploysentry-cli-darwin-amd64
#   deploysentry-darwin-arm64, deploysentry-cli-darwin-arm64

# Docker images
make docker-build
```

---

## Project Structure

```
cmd/
  api/                  API server entry point
  cli/                  CLI tool (auth, deploy, flags, releases, projects, config)
internal/
  auth/                 Authentication, OAuth2, RBAC, API keys
  deploy/               Deployment orchestration and strategies (canary, blue/green, rolling)
  flags/                Feature flag evaluation engine, targeting rules, repository
  releases/             Release tracking and environment promotion
  rollback/             Automated rollback controller and strategies
  health/               Health monitoring, scoring, APM integrations
  notifications/        Slack, PagerDuty, email, webhook notifications
  models/               Domain models (org, project, user, deployment, flag, release)
  platform/
    config/             Configuration loading
    database/           PostgreSQL connection and migrations
    cache/              Redis caching layer
    messaging/          NATS message broker
    middleware/         CORS, rate limiting
sdk/                    Multi-language SDKs (Go, Node, Python, Java, Ruby, React, Flutter)
web/                    React + TypeScript web dashboard
mobile/                 Flutter mobile app
deploy/                 Docker Compose, Dockerfiles, Kubernetes manifests
migrations/             PostgreSQL migration files
docs/                   Project plan, SDK onboarding guide, implementation checklists
```

---

## License

Apache-2.0
