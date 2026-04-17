# DeploySentry

Deploy release and feature flag management platform. DeploySentry gives engineering teams full visibility and control over their deployment lifecycle — safe rollouts, feature flags with granular targeting, release tracking, and automated rollbacks.

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Applications](#applications)
- [Feature Flags](#feature-flags)
  - [Flag Categories](#flag-categories)
  - [Flag Metadata](#flag-metadata)
  - [How Flags Work](#how-flags-work)
  - [Creating Flags](#creating-flags)
  - [Targeting Rules](#targeting-rules)
  - [Percentage Rollout Algorithm](#percentage-rollout-algorithm)
- [Integrating Into Your Project](#integrating-into-your-project)
  - [Install the CLI](#1-install-the-cli)
  - [Get an API Key](#2-get-an-api-key)
  - [Install the SDK](#3-install-the-sdk)
  - [Initialize and Evaluate](#4-initialize-and-evaluate) (Go, Node, Python, Java, React, Flutter, Ruby)
  - [Best Practices for SDK Integration](#7-best-practices-for-sdk-integration)
  - [Evaluation API](#evaluation-api)
  - [SSE Streaming Protocol](#sse-streaming-protocol)
  - [Rule Management Endpoints](#rule-management-endpoints)
  - [Session Consistency](#session-consistency)
- [Monitoring Flags](#monitoring-flags)
  - [Flag Lifecycle Management](#flag-lifecycle-management)
  - [Flag Health Dashboard](#flag-health-dashboard)
  - [API Endpoints](#api-endpoints-for-monitoring)
  - [Member Management API](#member-management-api)
  - [Observability Integration](#observability-integration)
  - [Webhooks & Slack](#webhook-notifications)
- [Deployments](#deployments)
  - [CLI Commands](#cli-commands)
  - [Automated Rollbacks](#automated-rollbacks)
- [Authentication](#authentication)
- [Database](#database)
- [Development](#development)
- [Project Status](#project-status)

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

**Core services:** Deploy Service, Feature Flag Service, Release Tracker, Health Monitor, Rollback Controller, Notification Service, Member Management, Settings Management, Entity Management (Orgs/Projects/Apps), Analytics, Ratings.

**Stack:** Go 1.22+, PostgreSQL 16, Redis 7, NATS JetStream, React + TypeScript (web), Flutter (mobile).

## Applications

DeploySentry provides multiple client applications:

- **Web Dashboard** (`/web`) - Full-featured React + TypeScript web application
- **Mobile App** (`/mobile`) - Native Flutter mobile application for iOS/Android
- **CLI Tool** (`/cmd/cli`) - Command-line interface for CI/CD integration

> **Note:** The web dashboard and mobile app maintain full feature parity. All functionality available in the web interface is also available in the mobile app, ensuring a consistent experience across platforms.

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

#### Percentage Rollout Algorithm

Percentage rollouts use a deterministic hash-based algorithm to assign users to buckets:

1. Compute `SHA256("{flag_key}:{user_id}")`
2. Take the first 8 bytes of the hash and convert to a `uint64`
3. Compute `bucket = uint64 % 101` (yields a value 0-100)
4. The user matches the rule if `bucket <= percentage`

**Properties:**

- **Consistency** — The same user always gets the same bucket for a given flag, ensuring a stable experience across requests and sessions.
- **Uniform distribution** — SHA256 produces a uniform hash, so buckets are evenly distributed across the user population.
- **Independence between flags** — Because the flag key is part of the hash input, a user's bucket for one flag is independent of their bucket for another flag. This prevents correlated rollouts.

---

## Integrating Into Your Project

### 1. Install the CLI

```bash
# One-line install (Linux / macOS)
curl -fsSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/install.sh | sh

# Or build from source
go install ./cmd/cli
```

### 2. Get an API Key

Create an API key with at minimum `flags:read` scope. If reporting evaluation telemetry, also add `flags:write`.

```bash
deploysentry apikeys create --name "my-service" --scopes flags:read
```

### 3. Install the SDK

| Language | Package | Install |
|----------|---------|---------|
| Go | `github.com/deploysentry/deploysentry-go` | `go get github.com/deploysentry/deploysentry-go` |
| Node.js | `@deploysentry/sdk` | `npm install @deploysentry/sdk` |
| Python | `deploysentry` | `pip install deploysentry` |
| Java | `io.deploysentry:deploysentry-java` | Maven/Gradle (see below) |
| React | `@deploysentry/react` | `npm install @deploysentry/react` |
| Flutter | `deploysentry_flutter` | `flutter pub add deploysentry_flutter` |
| Ruby | `deploysentry` | `gem install deploysentry` |

### 4. Initialize and Evaluate

All SDKs follow the same pattern: initialize with your API key, evaluate flags with user context, and access rich metadata.

#### Go

```go
import deploysentry "github.com/deploysentry/deploysentry-go"

client, err := deploysentry.NewClient(
    deploysentry.WithAPIKey("ds_key_xxxxxxxxxxxx"),
    deploysentry.WithBaseURL("https://api.dr-sentry.com"),
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
  baseURL: 'https://api.dr-sentry.com',
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
    base_url="https://api.dr-sentry.com",
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
    .baseURL("https://api.dr-sentry.com")
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
      baseURL="https://api.dr-sentry.com"
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
  baseURL: 'https://api.dr-sentry.com',
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
  base_url: 'https://api.dr-sentry.com',
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

### 5. Add Project Config

Drop a `.deploysentry.yml` in your project root to set defaults for CLI and SDK:

```yaml
org: "my-organization"
project: "my-project"
env: "staging"

api:
  url: "https://api.dr-sentry.com"

flags:
  default_env: "staging"
  stale_threshold: "30d"
```

### 6. SDK Behavior

All SDKs follow the same pattern:

- **Initialization:** Fetches all flag definitions for the configured project/environment and populates an in-memory cache.
- **Evaluation:** Reads from local cache first (< 1ms). Falls back to API on cache miss.
- **Real-time updates:** Opens an SSE stream to the Sentinel server. When a flag changes, the local cache is invalidated and refreshed automatically.
- **Offline mode:** If the API is unreachable, the SDK continues serving stale cached values and reconnects with exponential backoff.
- **Metadata:** All evaluation results include flag metadata (category, purpose, owners, expiration) for logging and observability.
- **Cleanup:** Call `close()` on shutdown to release the streaming connection.

### 7. Best Practices for SDK Integration

**Initialize once, evaluate many.** Create a single client instance at application startup and reuse it throughout the process. The client maintains a local cache and SSE connection — creating multiple instances wastes memory and connections.

```go
// GOOD — singleton client
var flagClient *deploysentry.Client

func main() {
    flagClient, _ = deploysentry.NewClient(
        deploysentry.WithAPIKey(os.Getenv("DEPLOYSENTRY_API_KEY")),
        deploysentry.WithEnvironment("production"),
        deploysentry.WithProject("my-project"),
    )
    defer flagClient.Close()
    flagClient.Initialize()
    // pass flagClient to your handlers/services
}
```

**Use evaluation context consistently.** Always pass user context when evaluating flags so targeting rules work correctly. Build a helper that extracts context from your request:

```go
func evalCtxFromRequest(r *http.Request) deploysentry.EvaluationContext {
    user := auth.UserFromContext(r.Context())
    return deploysentry.NewEvaluationContext().
        UserID(user.ID).
        Set("plan", user.Plan).
        Set("org_id", user.OrgID).
        Build()
}

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := evalCtxFromRequest(r)
    if flagClient.BoolValue(r.Context(), "new-checkout-flow", false, ctx) {
        serveNewCheckout(w, r)
    } else {
        serveOldCheckout(w, r)
    }
}
```

**Log flag evaluations for observability.** Use `Detail()` instead of `BoolValue()` when you need to correlate flag state with application behavior:

```go
result := flagClient.Detail(ctx, "new-checkout-flow", evalCtx)
logger.Info("flag evaluated",
    "flag", "new-checkout-flow",
    "value", result.Value,
    "category", result.Metadata.Category,
    "user", evalCtx.UserID,
)
```

**Handle the offline case.** SDKs serve stale cached values when the API is unreachable. Choose sensible defaults for each flag — the default is what users see during an outage:

```go
// Safe default: false means the old (stable) code path runs during outages
enabled := flagClient.BoolValue(ctx, "experimental-feature", false, evalCtx)

// Dangerous default: true means the new (untested) path runs during outages
enabled := flagClient.BoolValue(ctx, "experimental-feature", true, evalCtx)  // avoid
```

**Clean up flags proactively.** Use the lifecycle management commands to prevent flag debt:

```bash
# Add to your CI pipeline or run weekly
deploysentry flags list --category release --expired   # Find stale release flags
deploysentry flags list --stale                         # Flags not evaluated in 30+ days
```

**Scope flags to environments.** Use per-environment flag states to test in staging before enabling in production:

```bash
# Enable in staging first
curl -X PUT /api/v1/flags/<id>/environments/<staging-env-id> \
  -d '{"enabled": true}'

# After validation, enable in production
curl -X PUT /api/v1/flags/<id>/environments/<prod-env-id> \
  -d '{"enabled": true}'
```

---

### Evaluation API

#### Single Flag Evaluation

**`POST /api/v1/flags/evaluate`** — Evaluate a single flag for a given context.

**Request:**

```json
{
  "flag_key": "new-checkout-flow",
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "environment_id": "660e8400-e29b-41d4-a716-446655440000",
  "context": {
    "user_id": "user-123",
    "org_id": "org-456",
    "attributes": {
      "plan": "enterprise",
      "country": "US"
    }
  },
  "session_id": "sess-user123-v2.1.0"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `flag_key` | string | Yes | The unique key of the flag to evaluate |
| `project_id` | UUID | No | Project scope (inferred from API key if omitted) |
| `environment_id` | UUID | No | Environment scope (uses default if omitted) |
| `context.user_id` | string | No | User identifier for targeting and rollout bucketing |
| `context.org_id` | string | No | Organization identifier for targeting |
| `context.attributes` | object | No | Arbitrary key-value pairs for attribute-based targeting |
| `session_id` | string | No | Session identifier for evaluation consistency caching |

**Response:**

```json
{
  "flag_key": "new-checkout-flow",
  "value": "true",
  "enabled": true,
  "reason": "TARGETING_MATCH",
  "metadata": {
    "category": "release",
    "purpose": "Progressive rollout of redesigned checkout",
    "owners": ["team-payments", "alice@example.com"],
    "tags": ["checkout", "frontend"],
    "expires_at": "2026-04-21T00:00:00Z"
  }
}
```

#### Batch Evaluation

**`POST /api/v1/flags/batch-evaluate`** — Evaluate multiple flags in a single request.

**Request:**

```json
{
  "flag_keys": ["new-checkout-flow", "enable-dark-mode", "advanced-analytics"],
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "environment_id": "660e8400-e29b-41d4-a716-446655440000",
  "context": {
    "user_id": "user-123",
    "org_id": "org-456",
    "attributes": {
      "plan": "enterprise"
    }
  },
  "session_id": "sess-user123-v2.1.0"
}
```

**Response:**

```json
{
  "results": [
    {
      "flag_key": "new-checkout-flow",
      "value": "true",
      "enabled": true,
      "reason": "TARGETING_MATCH",
      "metadata": { "category": "release", "purpose": "Progressive rollout of redesigned checkout", "owners": ["team-payments"], "tags": ["checkout"], "expires_at": "2026-04-21T00:00:00Z" }
    },
    {
      "flag_key": "enable-dark-mode",
      "value": "false",
      "enabled": false,
      "reason": "FLAG_DISABLED",
      "metadata": { "category": "feature", "purpose": "Dark mode UI toggle", "owners": ["team-frontend"], "tags": ["ui"], "expires_at": null }
    },
    {
      "flag_key": "advanced-analytics",
      "value": "true",
      "enabled": true,
      "reason": "PERCENTAGE_ROLLOUT",
      "metadata": { "category": "feature", "purpose": "Enterprise-only analytics dashboard", "owners": ["team-analytics"], "tags": [], "expires_at": null }
    }
  ]
}
```

#### Evaluation Reasons

The `reason` field indicates how the evaluation result was determined:

| Reason | Description |
|--------|-------------|
| `TARGETING_MATCH` | A targeting rule matched the provided context |
| `PERCENTAGE_ROLLOUT` | The user fell within the percentage rollout bucket |
| `DEFAULT_VALUE` | No rules matched; the flag's default value was returned |
| `FLAG_DISABLED` | The flag is disabled in this environment |
| `NOT_FOUND` | The flag key does not exist |
| `ERROR` | An error occurred during evaluation |
| `SESSION_CACHED` | The result was served from the session consistency cache |

---

### SSE Streaming Protocol

SDKs receive real-time flag updates via Server-Sent Events (SSE).

**Endpoint:** `GET /api/v1/flags/stream?project_id=<uuid>&token=<jwt>`

**Event format:**

```
event: flag_change
data: {"type":"flag.toggled","flag_key":"new-checkout-flow","project_id":"550e8400-...","enabled":true,"timestamp":"2026-03-21T12:00:00Z"}
```

#### Event Types

| Event Type | Description |
|------------|-------------|
| `flag.toggled` | A flag was enabled or disabled |
| `flag.created` | A new flag was created |
| `flag.updated` | Flag configuration was modified (name, metadata, default value) |
| `flag.deleted` | A flag was permanently deleted |
| `flag.archived` | A flag was archived (soft-deleted) |
| `flag.bulk_toggled` | Multiple flags were toggled in a single operation |
| `rule.created` | A targeting rule was added to a flag |
| `rule.updated` | A targeting rule was modified |
| `rule.deleted` | A targeting rule was removed from a flag |

#### Heartbeat

The server sends a heartbeat comment every 15 seconds to keep the connection alive:

```
: heartbeat
```

#### Reconnection

If the SSE connection drops, clients should reconnect using exponential backoff:

| Parameter | Value |
|-----------|-------|
| Initial delay | 1 second |
| Maximum delay | 30 seconds |
| Backoff factor | 2x |
| Jitter | +/- 20% |

---

### Rule Management Endpoints

Create, update, and delete targeting rules on a flag.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/flags/:id/rules` | Create a new targeting rule |
| `PUT` | `/api/v1/flags/:id/rules/:ruleId` | Update an existing rule |
| `DELETE` | `/api/v1/flags/:id/rules/:ruleId` | Delete a rule |

#### Rule Types

| Rule Type | Required Fields | Optional Fields |
|-----------|----------------|-----------------|
| `percentage` | `percentage` (0-100) | — |
| `user_target` | `user_ids` (string array) | — |
| `attribute` | `attribute`, `operator`, `value` | `negate` |
| `segment` | `segment_id` | — |
| `schedule` | `start_time`, `end_time` | `timezone`, `days_of_week` |
| `compound` | `rules` (nested rule array), `operator` (`AND`/`OR`) | — |

#### Attribute Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `eq` | Equals | `plan eq enterprise` |
| `neq` | Not equals | `status neq inactive` |
| `contains` | String contains | `email contains @acme.com` |
| `starts_with` | String starts with | `name starts_with admin` |
| `ends_with` | String ends with | `email ends_with .edu` |
| `in` | Value in list | `country in [US, CA, GB]` |
| `gt` | Greater than | `age gt 18` |
| `gte` | Greater than or equal | `version gte 2.0` |
| `lt` | Less than | `risk_score lt 50` |
| `lte` | Less than or equal | `retries lte 3` |

---

### Session Consistency

Session consistency ensures that a user sees the same flag values for the duration of a session, even if flag configurations change mid-session. This prevents jarring UX changes during an active workflow.

#### How It Works

1. The SDK sends a `session_id` with each evaluation request.
2. The server checks Redis for a cached evaluation result keyed by `session:{session_id}:{flag_key}`.
3. **Cache hit:** The cached value is returned immediately with reason `SESSION_CACHED`.
4. **Cache miss:** The flag is evaluated fresh, the result is cached in Redis, and the evaluated value is returned.
5. SSE updates are still received by the SDK, but cached session values take precedence until the session expires or is refreshed.

#### Session TTL

Sessions use a **30-minute sliding TTL** by default. Each evaluation request resets the TTL. Configure via:

```
DS_SESSION_TTL=30m
```

#### Session ID Composition

How you compose the `session_id` determines the consistency boundary:

| Strategy | Session ID Example | Behavior |
|----------|-------------------|----------|
| Per-user | `user-123` | Same flags for the user across all devices/versions |
| Per-user per app version | `user-123:v2.1.0` | Flags stay consistent within an app version |
| Per-user per device | `user-123:iphone-14` | Flags consistent per device |
| Per-session | `sess-abc123` | Flags consistent for a single browser/app session |

#### Refreshing a Session

To force re-evaluation (e.g., after a user upgrades their plan), call `refreshSession()`:

**Go:**

```go
// Force re-evaluation for this session
client.RefreshSession(ctx, "sess-user123-v2.1.0")

// Or refresh with a new session ID
client.SetSessionID("sess-user123-v2.2.0")
```

**Node.js:**

```typescript
// Force re-evaluation for this session
await client.refreshSession('sess-user123-v2.1.0');

// Or refresh with a new session ID
client.setSessionID('sess-user123-v2.2.0');
```

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

### Member Management API

Manage organization and project membership. Requires `org:manage` or `project:manage` permission.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/orgs/:orgSlug/members` | List org members (with user profile) |
| `POST` | `/api/v1/orgs/:orgSlug/members` | Add member by email (`{ "email", "role" }`) |
| `PUT` | `/api/v1/orgs/:orgSlug/members/:userId` | Update member role (`{ "role" }`) |
| `DELETE` | `/api/v1/orgs/:orgSlug/members/:userId` | Remove member |
| `GET` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/members` | List project members |
| `POST` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/members` | Add project member |
| `PUT` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/members/:userId` | Update project role |
| `DELETE` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/members/:userId` | Remove project member |

**Org roles:** `owner`, `admin`, `member`, `viewer`
**Project roles:** `admin`, `developer`, `viewer`

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

See the [Traffic Management Guide](docs/Traffic_Management_Guide.md) for controlling traffic splitting with the DeploySentry agent sidecar and Envoy proxy, including flag canary testing.

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

### CLI Commands

The CLI covers the full platform — flags, deployments, releases, and now organization and application management:

```bash
# Organization management
deploysentry orgs list                          # List your organizations
deploysentry orgs create --name "Acme" --slug acme  # Create an org
deploysentry orgs set acme                      # Set active org in config

# Application management
deploysentry apps list                          # List apps in current project
deploysentry apps create --name "Web App" --slug web-app
deploysentry apps get web-app                   # Get app details

# Settings management
deploysentry settings list --scope org --target <org-id>
deploysentry settings set --scope org --target <org-id> --key theme --value '"dark"'
deploysentry settings delete <setting-id>

# API key management
deploysentry apikeys list                       # List API keys
deploysentry apikeys create --name "ci" --scopes "flags:read,deploys:write"
deploysentry apikeys revoke <key-id>
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

### Quick Start for New Developers

```bash
# One-command setup (first time only)
make dev-setup

# Daily development workflow
make dev-cli start     # Start services
make dev-cli api       # Start API server (separate terminal)
make dev-cli web       # Start web UI (separate terminal)
```

### Development CLI

All development tasks are available through the unified CLI:

```bash
make dev-cli help               # Show all available commands
make dev-cli start              # Start PostgreSQL, Redis, NATS
make dev-cli stop               # Stop all services
make dev-cli test               # Run all tests
make dev-cli test unit          # Unit tests only
make dev-cli reset-db           # Reset database
make dev-cli logs postgres      # Show service logs
make dev-cli debug 2345        # Start API with debugger
```

### Traditional Make Commands

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

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for comprehensive development documentation.

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
| Web Dashboard | Complete |
| Member Management | Complete |
| Settings Management | Complete |
| Infrastructure & Ops | In Progress |
| Testing & Quality | In Progress |
| Mobile App | In Progress |

## License

See [LICENSE](LICENSE) for details.
