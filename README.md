# DeploySentry

Deploy release and feature flag management platform. DeploySentry gives engineering teams full visibility and control over their deployment lifecycle — safe rollouts, feature flags with granular targeting, release tracking, and automated rollbacks.

## Quick Start

```bash
# 1. Start backing services (PostgreSQL, Redis, NATS)
make dev-up

# 2. Run database migrations
make migrate-up

# 3. Copy and configure environment variables
cp .env.example .env

# 4. Start the API server (default: localhost:8080)
make run-api
```

## Architecture

```
 Dashboard UI / Mobile App / CLI
         │
     HTTPS/REST
         │
 ┌───────▼────────┐
 │ DeploySentry API│
 │   (Go / Gin)   │
 └──┬──────┬────┬──┘
    │      │    │
 ┌──▼──┐ ┌▼──┐ ┌▼──────────┐
 │ PG  │ │Redis│ │   NATS    │
 │     │ │    │ │ JetStream │
 └─────┘ └────┘ └─────┬─────┘
                       │
                ┌──────▼──────┐
                │  Sentinel   │
                │  Server     │
                └──┬──┬──┬──┬─┘
              SSE/gRPC streams
                │  │  │  │
            SDK clients (Go, Node, Python, etc.)
```

**Core services:** Deploy Service, Feature Flag Service, Release Tracker, Health Monitor, Rollback Controller, Notification Service.

**Stack:** Go 1.22+, PostgreSQL 16, Redis 7, NATS JetStream, React + TypeScript (web), Flutter (mobile).

---

## Feature Flags

### Flag Categories

Every flag has a **category** that describes its intent and lifecycle. This makes it easy to filter, audit, and clean up flags across your projects.

| Category | Purpose | Typical Lifecycle |
|----------|---------|-------------------|
| `release` | Gates code shipping with a release. Remove once fully rolled out. | Days to weeks. **Requires expiration date.** |
| `feature` | Controls long-lived product features (plan-gated, A/B permanent). | Weeks to permanent. |
| `experiment` | A/B tests and experiments with a defined end date. | Days to weeks. |
| `ops` | Operational controls — maintenance mode, rate limits, circuit breakers. | Permanent or event-driven. |
| `permission` | Gates access based on entitlements or roles. | Permanent. |

### Flag Metadata

Every flag carries metadata that makes it self-documenting and accountable:

| Field | Type | Description |
|-------|------|-------------|
| `category` | string | `release`, `feature`, `experiment`, `ops`, or `permission` |
| `purpose` | string | Human-readable description of why this flag exists |
| `owners` | string[] | Team or individuals responsible (e.g. `["team-frontend", "jane@example.com"]`) |
| `is_permanent` | boolean | If true, flag is not expected to expire |
| `expires_at` | timestamp | When this flag should be cleaned up (required for `release` flags) |
| `tags` | string[] | Arbitrary labels for filtering |

### How Flags Work

Feature flags let you decouple deployment from release. Ship code behind a flag, then control who sees it — by user, percentage, attribute, segment, or schedule — without redeploying.

**Evaluation flow:**
1. SDK checks local in-memory cache (sub-millisecond)
2. On cache miss, SDK calls `POST /api/v1/flags/evaluate`
3. API checks Redis cache, falls back to PostgreSQL
4. Result cached at both layers and returned

**Real-time updates:**
When a flag changes, the API publishes to NATS JetStream. The Sentinel server broadcasts to all connected SDKs via SSE or gRPC streaming. SDKs invalidate their local cache immediately — no polling required.

### Creating Flags

**Via CLI:**

```bash
# Release flag — gates a new checkout flow, owned by payments team, expires in 30 days
deploysentry flag create \
  --key new-checkout-flow \
  --name "New Checkout Flow" \
  --type boolean \
  --category release \
  --purpose "Progressive rollout of redesigned checkout" \
  --owners team-payments,alice@example.com \
  --expires-at 2026-04-21T00:00:00Z \
  --default false \
  --env production

# Feature flag — permanent plan-gated functionality
deploysentry flag create \
  --key advanced-analytics \
  --name "Advanced Analytics" \
  --type boolean \
  --category feature \
  --purpose "Enterprise-only analytics dashboard" \
  --owners team-analytics \
  --permanent \
  --default false \
  --env production

# Ops flag — circuit breaker for external API
deploysentry flag create \
  --key vendor-api-circuit-breaker \
  --name "Vendor API Circuit Breaker" \
  --type boolean \
  --category ops \
  --purpose "Kill switch for vendor API calls during outages" \
  --owners team-platform \
  --permanent \
  --default false \
  --env production
```

**Via API:**

```bash
curl -X POST http://localhost:8080/api/v1/flags \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "<project-uuid>",
    "environment_id": "<environment-uuid>",
    "key": "new-checkout-flow",
    "name": "New Checkout Flow",
    "flag_type": "boolean",
    "category": "release",
    "purpose": "Progressive rollout of redesigned checkout",
    "owners": ["team-payments", "alice@example.com"],
    "expires_at": "2026-04-21T00:00:00Z",
    "default_value": "false"
  }'
```

### Targeting Rules

Rules are evaluated in priority order (lower number = higher precedence). First match wins.

| Rule Type | Description | Example |
|-----------|-------------|---------|
| `percentage` | Hash-based rollout (deterministic per user+flag) | Roll out to 25% of users |
| `user_target` | Match specific user IDs | Enable for `user-123`, `user-456` |
| `attribute` | Match context attributes with operators | `plan eq enterprise` |
| `segment` | Match pre-defined user segments | Beta testers segment |
| `schedule` | Time-window activation | Enable Mon-Fri 9am-5pm |

**Attribute operators:** `eq`, `neq`, `contains`, `starts_with`, `ends_with`, `in`, `gt`, `gte`, `lt`, `lte`

---

## Integrating Into Your Project

### 1. Get an API Key

Create an API key with at minimum `flags:read` scope. If reporting evaluation telemetry, also add `flags:write`.

```bash
deploysentry apikey create --name "my-service" --scopes flags:read
```

### 2. Install the SDK

| Language | Package | Install |
|----------|---------|---------|
| Go | `github.com/deploysentry/deploysentry-go` | `go get github.com/deploysentry/deploysentry-go` |
| Node.js | `@deploysentry/sdk` | `npm install @deploysentry/sdk` |
| Python | `deploysentry` | `pip install deploysentry` |
| Java | `io.deploysentry:deploysentry-java` | Maven/Gradle (see below) |
| React | `@deploysentry/react` | `npm install @deploysentry/react` |
| Flutter | `deploysentry_flutter` | `flutter pub add deploysentry_flutter` |
| Ruby | `deploysentry` | `gem install deploysentry` |

### 3. Initialize and Evaluate

All SDKs follow the same pattern: initialize with your API key, evaluate flags with user context, and access rich metadata.

#### Go

```go
import deploysentry "github.com/deploysentry/deploysentry-go"

client, err := deploysentry.NewClient(
    deploysentry.WithAPIKey("ds_key_xxxxxxxxxxxx"),
    deploysentry.WithBaseURL("https://deploysentry.example.com"),
    deploysentry.WithEnvironment("production"),
    deploysentry.WithProject("my-project"),
)
defer client.Close()

client.Initialize()

evalCtx := deploysentry.NewEvaluationContext().
    UserID("user-123").
    Set("plan", "enterprise").
    Build()

// Simple evaluation
darkMode := client.BoolValue(ctx, "enable-dark-mode", false, evalCtx)

// Rich evaluation with metadata
result := client.Detail(ctx, "new-checkout-flow", evalCtx)
fmt.Printf("value=%s category=%s owners=%v expires=%v\n",
    result.Value, result.Metadata.Category, result.Metadata.Owners, result.Metadata.ExpiresAt)

// Query flags by category
releaseFlags := client.FlagsByCategory(deploysentry.CategoryRelease)
expiredFlags := client.ExpiredFlags()
owners := client.FlagOwners("new-checkout-flow")
```

#### Node.js / TypeScript

```typescript
import { DeploySentryClient } from '@deploysentry/sdk';

const client = new DeploySentryClient({
  apiKey: 'ds_key_xxxxxxxxxxxx',
  baseURL: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
});
await client.initialize();

// Simple evaluation
const darkMode = await client.boolValue('enable-dark-mode', false, {
  userId: 'user-123',
  attributes: { plan: 'enterprise' },
});

// Rich evaluation with metadata
const result = await client.detail('new-checkout-flow');
console.log(result.metadata.category);  // "release"
console.log(result.metadata.owners);    // ["team-payments", "alice@example.com"]
console.log(result.metadata.expiresAt); // "2026-04-21T00:00:00Z"

// Query by category
const releaseFlags = client.flagsByCategory('release');
const expiredFlags = client.expiredFlags();

await client.close();
```

#### Python

```python
from deploysentry import DeploySentryClient, EvaluationContext, FlagCategory

client = DeploySentryClient(
    api_key="ds_key_xxxxxxxxxxxx",
    base_url="https://deploysentry.example.com",
    environment="production",
    project="my-project",
)
client.initialize()

ctx = EvaluationContext(user_id="user-123", attributes={"plan": "enterprise"})

# Simple evaluation
dark_mode = client.bool_value("enable-dark-mode", default=False, context=ctx)

# Rich evaluation with metadata
result = client.detail("new-checkout-flow", context=ctx)
print(result.metadata.category)   # FlagCategory.RELEASE
print(result.metadata.owners)     # ["team-payments", "alice@example.com"]
print(result.metadata.expires_at) # datetime

# Query by category
release_flags = client.flags_by_category(FlagCategory.RELEASE)
expired_flags = client.expired_flags()

client.close()
```

#### Java

```java
import io.deploysentry.*;

var options = ClientOptions.builder()
    .apiKey("ds_key_xxxxxxxxxxxx")
    .baseURL("https://deploysentry.example.com")
    .environment("production")
    .project("my-project")
    .build();

try (var client = new DeploySentryClient(options)) {
    client.initialize();

    var ctx = EvaluationContext.builder()
        .userId("user-123")
        .attribute("plan", "enterprise")
        .build();

    // Simple evaluation
    boolean darkMode = client.boolValue("enable-dark-mode", false, ctx);

    // Rich evaluation with metadata
    EvaluationResult<Boolean> result = client.detail("new-checkout-flow", ctx);
    System.out.println(result.getMetadata().getCategory());  // RELEASE
    System.out.println(result.getMetadata().getOwners());     // [team-payments, alice@example.com]

    // Query by category
    List<Flag> releaseFlags = client.flagsByCategory(FlagCategory.RELEASE);
    List<Flag> expiredFlags = client.expiredFlags();
}
```

#### React

```tsx
import { DeploySentryProvider, useFlag, useFlagDetail, useFlagsByCategory } from '@deploysentry/react';

function App() {
  return (
    <DeploySentryProvider
      apiKey="ds_key_xxxxxxxxxxxx"
      baseURL="https://deploysentry.example.com"
      environment="production"
      project="my-project"
      user={{ userId: 'user-123', attributes: { plan: 'enterprise' } }}
    >
      <Dashboard />
    </DeploySentryProvider>
  );
}

function Dashboard() {
  // Simple flag evaluation — re-renders on SSE updates
  const darkMode = useFlag('enable-dark-mode', false);

  // Rich metadata access
  const { value, enabled, metadata, loading } = useFlagDetail('new-checkout-flow');
  // metadata.category === 'release'
  // metadata.owners === ['team-payments', 'alice@example.com']

  // Query by category
  const releaseFlags = useFlagsByCategory('release');

  return <div className={darkMode ? 'dark' : 'light'}>...</div>;
}
```

#### Flutter / Dart

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

final client = DeploySentryClient(
  apiKey: 'ds_key_xxxxxxxxxxxx',
  baseURL: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
);
await client.initialize();

final ctx = EvaluationContext(
  userId: 'user-123',
  attributes: {'plan': 'enterprise'},
);

// Simple evaluation
final darkMode = client.boolValue('enable-dark-mode', defaultValue: false, context: ctx);

// Rich evaluation with metadata
final result = client.detail('new-checkout-flow', context: ctx);
print(result.metadata?.category);  // FlagCategory.release
print(result.metadata?.owners);    // ['team-payments', 'alice@example.com']

// Query by category
final releaseFlags = client.flagsByCategory(FlagCategory.release);
final expiredFlags = client.expiredFlags();
```

#### Ruby

```ruby
require 'deploysentry'

client = DeploySentry::Client.new(
  api_key: 'ds_key_xxxxxxxxxxxx',
  base_url: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
)
client.initialize!

ctx = DeploySentry::EvaluationContext.new(
  user_id: 'user-123',
  attributes: { 'plan' => 'enterprise' },
)

# Simple evaluation
dark_mode = client.bool_value('enable-dark-mode', default: false, context: ctx)

# Rich evaluation with metadata
result = client.detail('new-checkout-flow', context: ctx)
puts result.metadata.category   # "release"
puts result.metadata.owners     # ["team-payments", "alice@example.com"]

# Query by category
release_flags = client.flags_by_category(DeploySentry::FlagCategory::RELEASE)
expired_flags = client.expired_flags

client.close
```

### 4. Add Project Config

Drop a `.deploysentry.yml` in your project root to set defaults for CLI and SDK:

```yaml
org: "my-organization"
project: "my-project"
env: "staging"

api:
  url: "https://deploysentry.example.com"

flags:
  default_env: "staging"
  stale_threshold: "30d"
```

### 5. SDK Behavior

All SDKs follow the same pattern:

- **Initialization:** Fetches all flag definitions for the configured project/environment and populates an in-memory cache.
- **Evaluation:** Reads from local cache first (< 1ms). Falls back to API on cache miss.
- **Real-time updates:** Opens an SSE stream to the Sentinel server. When a flag changes, the local cache is invalidated and refreshed automatically.
- **Offline mode:** If the API is unreachable, the SDK continues serving stale cached values and reconnects with exponential backoff.
- **Metadata:** All evaluation results include flag metadata (category, purpose, owners, expiration) for logging and observability.
- **Cleanup:** Call `close()` on shutdown to release the streaming connection.

---

## Monitoring Flags

### Flag Lifecycle Management

Use flag categories and metadata to keep your flag inventory healthy:

```bash
# Find all release flags that should have been cleaned up
deploysentry flag list --category release --expired

# Find all flags owned by a specific team
deploysentry flag list --owner team-payments

# Find stale flags (not evaluated in 30+ days)
deploysentry flag list --stale

# Archive a flag that's fully rolled out
deploysentry flag archive new-checkout-flow
```

SDKs also expose these queries programmatically:

| Method | Description |
|--------|-------------|
| `flagsByCategory(category)` | All cached flags of a given category |
| `expiredFlags()` | Flags past their `expires_at` date |
| `flagOwners(key)` | Owners for a specific flag |
| `allFlags()` | All cached flags with full metadata |

### Flag Health Dashboard

The web dashboard provides real-time visibility into flag state:

- **Active flags** — which flags are enabled, per environment, grouped by category
- **Evaluation metrics** — how often each flag is evaluated, hit rates by rule
- **Expiration tracking** — flags approaching or past their expiration date
- **Owner accountability** — flags grouped by owner for cleanup assignments
- **Stale flag detection** — flags not evaluated within the configured `stale_threshold`
- **Audit log** — every flag change recorded with who, when, and what changed

### API Endpoints for Monitoring

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/flags?project_id=<uuid>` | List all flags with metadata |
| `GET` | `/api/v1/flags?project_id=<uuid>&category=release` | Filter by category |
| `GET` | `/api/v1/flags/:id` | Flag details including rules and metadata |
| `GET` | `/health` | Full health check (DB, Redis, NATS) |
| `GET` | `/ready` | Lightweight readiness probe |

### Observability Integration

DeploySentry connects flag state to your existing observability stack:

- **Structured logging** — All flag evaluations and changes emit structured JSON logs including category, owners, and purpose. Forward to your log aggregator (Datadog, Splunk, ELK) for correlation.
- **NATS events** — Subscribe to `flag.changed.<project_id>` events from NATS JetStream to trigger custom alerting or dashboards.
- **Webhooks** — Configure outbound webhooks to receive HTTP POST callbacks on flag changes. Payloads are signed with HMAC-SHA256 (`X-DeploySentry-Signature` header).
- **Prometheus metrics** (planned) — Flag evaluation counters, cache hit rates, and streaming connection health exposed at `/metrics`.

### Webhook Notifications

Configure webhooks in the dashboard to get notified of flag changes:

```json
{
  "event_type": "flag.toggled",
  "timestamp": "2026-03-21T12:00:00Z",
  "project_id": "<uuid>",
  "data": {
    "flag_key": "new-checkout-flow",
    "category": "release",
    "owners": ["team-payments"],
    "enabled": true
  }
}
```

Supported events: `flag.changed`, `deploy.started`, `deploy.completed`, `deploy.failed`, `deploy.rolled_back`

### Slack Integration

DeploySentry sends deploy and flag change notifications to Slack. Configure via `.deploysentry.yml`:

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

## Deployments

DeploySentry supports three deployment strategies:

| Strategy | Description | Use Case |
|----------|-------------|----------|
| **Canary** | Gradually shift traffic (1% → 5% → 25% → 50% → 100%) with health checks at each phase | Production releases requiring validation |
| **Blue/Green** | Atomic traffic switch with instant rollback | Zero-downtime releases |
| **Rolling** | Batch-based instance updates | Stateless services, fast rollouts |

```bash
# Create a canary deployment
deploysentry deploy create --strategy canary --version v2.1.0 --env production

# Check deployment status
deploysentry deploy status <deploy-id>

# Promote to full traffic
deploysentry deploy promote <deploy-id>

# Rollback if something goes wrong
deploysentry deploy rollback <deploy-id>
```

### Automated Rollbacks

The Health Monitor continuously checks error rates, latency, error tracking signals, and custom metrics. If health drops below the configured threshold during a deployment, the Rollback Controller automatically reverts traffic.

---

## Authentication

| Method | Use Case | Header |
|--------|----------|--------|
| JWT Bearer Token | Dashboard UI, mobile app | `Authorization: Bearer <jwt>` |
| API Key | Services, SDKs, CI/CD | `Authorization: ApiKey <key>` |

**API key scopes:** `flags:read`, `flags:write`, `deploys:read`, `deploys:write`, `releases:read`, `releases:write`, `admin`

**OAuth providers:** GitHub, Google

---

## Database

DeploySentry is designed for **shared database environments** and uses the PostgreSQL `deploy` schema namespace to isolate its tables from other applications.

### Schema Namespace

All tables live in the `deploy` schema, not `public`:

```sql
-- Example: Tables are created like this
CREATE SCHEMA IF NOT EXISTS deploy;
CREATE TABLE deploy.feature_flags (...);
```

**Connection Configuration:**
- Application DSN: `postgres://user:pass@host:port/db?search_path=deploy`
- Migration DSN: `postgres://user:pass@host:port/db?search_path=deploy`
- All queries automatically target the `deploy` schema

### Shared Database Benefits

✅ **Isolation:** Complete separation from other applications
✅ **Security:** Schema-level permissions and access controls
✅ **Naming:** No table/index conflicts with other services
✅ **Operations:** Independent migrations and maintenance

### Database Requirements

- **PostgreSQL 16+** with `pgcrypto` extension
- **Schema permissions:** `USAGE` on `deploy` schema
- **Table permissions:** `ALL PRIVILEGES` on tables in `deploy` schema
- **Index naming:** All indexes prefixed with `deploy_` to prevent conflicts

### Production Setup

For shared database environments, create dedicated users:

```sql
-- Database owner (for migrations)
CREATE USER deploysentry_owner WITH LOGIN PASSWORD 'secure-password';
GRANT CREATE ON DATABASE production_db TO deploysentry_owner;

-- Application user (for runtime)
CREATE USER deploysentry_app WITH LOGIN PASSWORD 'secure-password';
GRANT USAGE ON SCHEMA deploy TO deploysentry_app;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA deploy TO deploysentry_app;
```

Then run migrations as the owner:
```bash
# migrations/025_secure_deploy_schema.up.sql handles permission setup
make migrate-up
```

---

## Development

```bash
make dev-up          # Start PostgreSQL, Redis, NATS
make dev-down        # Stop infrastructure
make migrate-up      # Run migrations
make run-api         # Start API server
make run-web         # Start web frontend dev server
make test            # Run all tests
make test-unit       # Unit tests only
make test-int        # Integration tests only
make lint            # Run linters
make build           # Build binaries (linux, darwin amd64/arm64)
make docker-build    # Build Docker images
```

### Environment Variables

See [`.env.example`](.env.example) for all configuration options. Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DS_SERVER_PORT` | `8080` | API server port |
| `DS_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `DS_DATABASE_SCHEMA` | `deploy` | PostgreSQL schema namespace (for shared databases) |
| `DS_REDIS_HOST` | `localhost` | Redis host (flag evaluation cache) |
| `DS_NATS_URL` | `nats://localhost:4222` | NATS server (real-time push updates) |
| `DS_AUTH_JWT_SECRET` | — | JWT signing secret (change in production) |

---

## Project Status

The backend services, database layer, CLI tool, client SDKs, and core business logic are complete. See [`docs/PROJECT_PLAN.md`](docs/PROJECT_PLAN.md) for the full plan and [`docs/checklists/`](docs/checklists/) for detailed progress tracking.

| Component | Status |
|-----------|--------|
| Project Scaffolding | Complete |
| Data Model & Migrations | Complete |
| Auth & RBAC | Complete |
| Deploy Service | Complete |
| Feature Flag Service | Complete |
| Release Tracker | Complete |
| Health & Rollback | Complete |
| Notification Service | Complete |
| CLI Tool | Complete |
| Client SDKs (Go, Node, Python, Java, React, Flutter, Ruby) | Complete |
| Web Dashboard | In Progress |
| Infrastructure & Ops | In Progress |
| Testing & Quality | In Progress |
| Mobile App | Planned |

## License

See [LICENSE](LICENSE) for details.
