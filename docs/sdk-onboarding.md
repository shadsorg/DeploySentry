# DeploySentry SDK Onboarding Guide

This guide walks through setting up local feature flags, integrating the DeploySentry SDK into a backend service, and understanding the API endpoints used by the UI and the Sentinel server push-update mechanism.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development Setup](#local-development-setup)
3. [Configuring Local Flags](#configuring-local-flags)
4. [Integrating the SDK into a Backend Service](#integrating-the-sdk-into-a-backend-service)
   - [Go SDK](#go-sdk)
   - [Node.js / TypeScript SDK](#nodejs--typescript-sdk)
   - [Python SDK](#python-sdk)
   - [Flutter / Dart SDK](#flutter--dart-sdk)
5. [API Endpoints for UI Flag Management](#api-endpoints-for-ui-flag-management)
6. [Sentinel Server Push Updates](#sentinel-server-push-updates)
7. [Authentication](#authentication)
8. [Evaluation Context and Targeting](#evaluation-context-and-targeting)
9. [Architecture Overview](#architecture-overview)

---

## Prerequisites

- Docker and Docker Compose (for local infrastructure)
- Go 1.22+ (for the API server and Go SDK)
- An API key or JWT token for authenticating with the DeploySentry API
- Access to your organization's project and environment IDs

## Local Development Setup

Start the backing services (PostgreSQL, Redis, NATS) and run database migrations:

```bash
# Start PostgreSQL, Redis, and NATS
make dev-up

# Run database migrations
make migrate-up

# Start the API server (default: 0.0.0.0:8080)
make run-api
```

The API server reads configuration from environment variables prefixed with `DS_` or from a `.deploysentry.yml` config file. Copy the example env file to get started:

```bash
cp .env.example .env
# Edit .env with your local values, then:
source .env
```

Key environment variables:

| Variable | Default | Description |
|---|---|---|
| `DS_SERVER_PORT` | `8080` | API server listen port |
| `DS_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `DS_DATABASE_PORT` | `5432` | PostgreSQL port |
| `DS_REDIS_HOST` | `localhost` | Redis host (used for flag evaluation cache) |
| `DS_REDIS_PORT` | `6379` | Redis port |
| `DS_NATS_URL` | `nats://localhost:4222` | NATS server URL (used for real-time push updates) |
| `DS_AUTH_JWT_SECRET` | (change in prod) | Secret for signing JWT tokens |

## Configuring Local Flags

### Creating a flag via the API

```bash
curl -X POST http://localhost:8080/api/v1/flags \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <your-api-key>" \
  -d '{
    "project_id": "<project-uuid>",
    "environment_id": "<environment-uuid>",
    "key": "enable-dark-mode",
    "name": "Enable Dark Mode",
    "description": "Toggle the dark mode UI theme",
    "flag_type": "boolean",
    "default_value": "false"
  }'
```

### Toggling a flag

```bash
curl -X POST http://localhost:8080/api/v1/flags/<flag-id>/toggle \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <your-api-key>" \
  -d '{"enabled": true}'
```

### Adding a targeting rule

Targeting rules let you roll out a flag to specific users, a percentage of traffic, or based on context attributes:

```bash
# Percentage rollout (25% of users)
curl -X POST http://localhost:8080/api/v1/flags/<flag-id>/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <your-api-key>" \
  -d '{
    "rule_type": "percentage",
    "priority": 1,
    "value": "true",
    "percentage": 25
  }'

# Target specific users
curl -X POST http://localhost:8080/api/v1/flags/<flag-id>/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <your-api-key>" \
  -d '{
    "rule_type": "user_target",
    "priority": 0,
    "value": "true",
    "target_values": ["user-123", "user-456"]
  }'
```

### Project configuration file

The `.deploysentry.yml` file in your project root sets defaults for CLI and SDK operations:

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

## Integrating the SDK into a Backend Service

The SDK handles flag evaluation locally with a cache, connects to the DeploySentry API for flag definitions, and receives real-time updates when flags change on the Sentinel server.

### Go SDK

```go
package main

import (
    "context"
    "fmt"
    "log"

    deploysentry "github.com/deploysentry/deploysentry/sdk/go"
)

func main() {
    // Initialize the client with your API key and the DeploySentry API URL.
    client, err := deploysentry.NewClient(
        deploysentry.WithAPIKey("ds_key_xxxxxxxxxxxx"),
        deploysentry.WithBaseURL("http://localhost:8080"),
        deploysentry.WithEnvironment("staging"),
    )
    if err != nil {
        log.Fatalf("failed to initialize deploysentry client: %v", err)
    }
    defer client.Close()

    // Build an evaluation context for the current user/request.
    ctx := context.Background()
    evalCtx := deploysentry.NewContext().
        SetUserID("user-123").
        SetAttribute("plan", "enterprise").
        SetAttribute("country", "US")

    // Evaluate a boolean flag with a default fallback.
    darkMode := client.BoolValue(ctx, "enable-dark-mode", false, evalCtx)
    fmt.Printf("dark mode enabled: %v\n", darkMode)

    // Evaluate a string flag.
    bannerText := client.StringValue(ctx, "promo-banner-text", "Welcome!", evalCtx)
    fmt.Printf("banner: %s\n", bannerText)

    // Evaluate a JSON flag.
    featureConfig := client.JSONValue(ctx, "checkout-config", `{}`, evalCtx)
    fmt.Printf("config: %s\n", featureConfig)
}
```

**How it works internally:**

1. On initialization, the SDK fetches all flag definitions for the configured project/environment and populates an in-memory cache.
2. `BoolValue` / `StringValue` / `JSONValue` evaluate against the local cache first (sub-millisecond latency).
3. On cache miss, the SDK calls `POST /api/v1/flags/evaluate` on the API server.
4. The SDK opens a streaming connection (gRPC stream or SSE) to receive push updates from the Sentinel server. When a flag changes, the local cache is invalidated and refreshed.
5. If the API is unreachable, the SDK serves stale values from the local cache (offline mode).

### Node.js / TypeScript SDK

```typescript
import { DeploySentryClient } from '@dr-sentry/sdk';

const client = new DeploySentryClient({
  apiKey: 'ds_key_xxxxxxxxxxxx',
  baseURL: 'http://localhost:8080',
  environment: 'staging',
});

await client.initialize();

// Evaluate flags with user context
const darkMode = await client.boolValue('enable-dark-mode', false, {
  userId: 'user-123',
  attributes: { plan: 'enterprise', country: 'US' },
});

console.log(`dark mode: ${darkMode}`);

// The SDK listens for SSE push updates automatically.
// When a flag changes on the server, the local cache updates
// and subsequent evaluations reflect the new value.

// Clean up on shutdown
await client.close();
```

**React integration:**

```tsx
import { DeploySentryProvider, useFlag } from '@dr-sentry/react';

function App() {
  return (
    <DeploySentryProvider
      apiKey="ds_key_xxxxxxxxxxxx"
      baseURL="http://localhost:8080"
      environment="staging"
      user={{ userId: 'user-123', attributes: { plan: 'enterprise' } }}
    >
      <Dashboard />
    </DeploySentryProvider>
  );
}

function Dashboard() {
  // Re-renders automatically when the flag value changes via SSE push.
  const darkMode = useFlag('enable-dark-mode', false);

  return <div className={darkMode ? 'dark' : 'light'}>...</div>;
}
```

### Python SDK

```python
from deploysentry import DeploySentryClient, EvaluationContext

client = DeploySentryClient(
    api_key="ds_key_xxxxxxxxxxxx",
    base_url="http://localhost:8080",
    environment="staging",
)
client.initialize()

ctx = EvaluationContext(
    user_id="user-123",
    attributes={"plan": "enterprise", "country": "US"},
)

dark_mode = client.bool_value("enable-dark-mode", default=False, context=ctx)
print(f"dark mode: {dark_mode}")

client.close()
```

### Flutter / Dart SDK

Add the DeploySentry Flutter SDK to your `pubspec.yaml`:

```yaml
dependencies:
  deploysentry_flutter: ^1.0.0
```

**Initialize the client** in your app's startup (typically in `main.dart` or an early widget):

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final client = DeploySentryClient(
    apiKey: 'ds_key_xxxxxxxxxxxx',
    baseURL: 'http://localhost:8080',
    environment: 'staging',
  );
  await client.initialize();

  runApp(
    DeploySentryProvider(
      client: client,
      child: const MyApp(),
    ),
  );
}
```

**Evaluate flags** using the provider or directly via the client:

```dart
import 'package:deploysentry_flutter/deploysentry_flutter.dart';
import 'package:flutter/material.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context) {
    // Reactive — rebuilds automatically when the flag changes via SSE push.
    final darkMode = DeploySentry.of(context).boolValue(
      'enable-dark-mode',
      defaultValue: false,
      context: EvaluationContext(
        userId: 'user-123',
        attributes: {'plan': 'enterprise', 'country': 'US'},
      ),
    );

    return MaterialApp(
      theme: darkMode ? ThemeData.dark() : ThemeData.light(),
      home: const Scaffold(body: Center(child: Text('Dashboard'))),
    );
  }
}
```

**Using hooks** (with the `flutter_hooks` or `riverpod` pattern):

```dart
// With a ChangeNotifier / ValueNotifier approach
class FlagNotifier extends ValueNotifier<bool> {
  FlagNotifier(this._client, this._key, bool defaultValue)
      : super(defaultValue) {
    _client.onFlagChanged(_key, (newValue) {
      value = newValue == 'true';
    });
  }

  final DeploySentryClient _client;
  final String _key;
}
```

**Evaluate other flag types:**

```dart
final client = DeploySentry.of(context);

// String flag
final bannerText = client.stringValue('promo-banner-text', defaultValue: 'Welcome!');

// JSON flag (returns a decoded Map<String, dynamic>)
final checkoutConfig = client.jsonValue('checkout-config', defaultValue: {});

// Integer flag
final maxRetries = client.intValue('max-retries', defaultValue: 3);
```

**How it works internally:**

1. On `initialize()`, the SDK fetches all flag definitions for the configured project/environment and stores them in an in-memory cache.
2. The SDK opens an SSE connection to `GET /api/v1/flags/stream` to receive real-time push updates from the Sentinel server.
3. Flag evaluations read from the local cache (sub-millisecond). On cache miss, the SDK calls `POST /api/v1/flags/evaluate`.
4. When the SSE stream delivers a `flag.changed` event, the local cache is invalidated and the `DeploySentryProvider` notifies dependent widgets to rebuild.
5. If the network is unavailable, the SDK continues serving stale cached values (offline mode) and reconnects with exponential backoff when connectivity is restored.

**Cleanup on app disposal:**

```dart
@override
void dispose() {
  DeploySentry.of(context).client.close();
  super.dispose();
}
```

## API Endpoints for UI Flag Management

These are the REST endpoints exposed by the DeploySentry API server for managing feature flags from the dashboard UI. All endpoints are mounted under `/api/v1` and require authentication via JWT bearer token or API key.

### Flag CRUD

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/flags` | Create a new feature flag |
| `GET` | `/api/v1/flags?project_id=<uuid>` | List flags for a project (paginated) |
| `GET` | `/api/v1/flags/:id` | Get a single flag by ID |
| `PUT` | `/api/v1/flags/:id` | Update flag name, description, or default value |
| `POST` | `/api/v1/flags/:id/archive` | Archive a flag (disables and marks archived) |
| `POST` | `/api/v1/flags/:id/toggle` | Toggle a flag on or off |

### Flag Evaluation

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/flags/evaluate` | Evaluate a flag for a given context |

**Request body for `/api/v1/flags/evaluate`:**

```json
{
  "project_id": "<uuid>",
  "environment_id": "<uuid>",
  "flag_key": "enable-dark-mode",
  "context": {
    "user_id": "user-123",
    "org_id": "org-456",
    "attributes": {
      "plan": "enterprise",
      "country": "US"
    }
  }
}
```

**Response:**

```json
{
  "flag_key": "enable-dark-mode",
  "enabled": true,
  "value": "true",
  "reason": "rule_match",
  "rule_id": "<uuid>"
}
```

Possible `reason` values:
- `"default"` -- Flag is enabled but no targeting rules matched; returning the default value.
- `"rule_match"` -- A targeting rule matched the evaluation context.
- `"flag_disabled"` -- Flag is disabled or archived; returning the default value.

### Targeting Rules

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/flags/:id/rules` | Add a targeting rule to a flag |
| `PUT` | `/api/v1/flags/:id/rules/:ruleId` | Update an existing targeting rule |
| `DELETE` | `/api/v1/flags/:id/rules/:ruleId` | Delete a targeting rule |

### Deployments (related context)

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/deployments` | Create a new deployment |
| `GET` | `/api/v1/deployments?project_id=<uuid>` | List deployments for a project |
| `GET` | `/api/v1/deployments/:id` | Get deployment details |
| `POST` | `/api/v1/deployments/:id/promote` | Promote deployment to full traffic |
| `POST` | `/api/v1/deployments/:id/rollback` | Rollback a deployment |
| `POST` | `/api/v1/deployments/:id/pause` | Pause a running deployment |
| `POST` | `/api/v1/deployments/:id/resume` | Resume a paused deployment |

### Releases

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/releases` | Create a new release |
| `GET` | `/api/v1/releases?project_id=<uuid>` | List releases for a project |
| `GET` | `/api/v1/releases/:id` | Get release details |
| `POST` | `/api/v1/releases/:id/promote` | Promote a release to an environment |

### Authentication

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/auth/github` | Initiate GitHub OAuth login |
| `GET` | `/auth/github/callback` | GitHub OAuth callback |
| `GET` | `/auth/google` | Initiate Google OAuth login |
| `GET` | `/auth/google/callback` | Google OAuth callback |

### Health

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Full health check (database, Redis, NATS) |
| `GET` | `/ready` | Lightweight readiness probe |

## Sentinel Server Push Updates

The Sentinel server is the DeploySentry component responsible for broadcasting real-time flag changes to all connected SDK clients. This eliminates polling and keeps flag evaluations consistent across your fleet within seconds of a change.

### How it works

```
 Dashboard UI                     DeploySentry API              NATS JetStream
 +-----------+                    +----------------+            +-------------+
 |  Toggle   | --HTTP POST-----→ |  Update flag   | --publish→ | flag.changed|
 |  flag     |                    |  in PostgreSQL |            |   stream    |
 +-----------+                    +---+---+--------+            +------+------+
                                      |   |                            |
                                      |   | invalidate                 |
                                      |   ↓                            |
                                      | Redis cache                    |
                                      |                                |
                                      |                                ↓
                                 +----+----------------------------+--------+
                                 |         Sentinel Server                  |
                                 |  Subscribes to NATS "flag.changed"      |
                                 |  Broadcasts via SSE / gRPC streams      |
                                 +----+----+-------+-------+---------------+
                                      |    |       |       |
                                      ↓    ↓       ↓       ↓
                                   SDK A  SDK B   SDK C   SDK D
                                 (Go svc)(Node) (React) (Flutter)
```

1. **Flag change** -- A user toggles a flag or updates a targeting rule via the UI or API.
2. **Persist + publish** -- The API server writes the change to PostgreSQL, invalidates the Redis cache, and publishes a `flag.changed` event to NATS JetStream.
3. **Sentinel broadcasts** -- The Sentinel server is a durable NATS consumer subscribed to flag change events. On receiving an event, it broadcasts the update to all connected SDK clients via:
   - **SSE (Server-Sent Events)** for browser-based and HTTP clients (Node.js, Python, React, Flutter SDKs)
   - **gRPC streaming** for backend services (Go, Java SDKs)
4. **SDK cache refresh** -- Each SDK receives the push notification, invalidates its local cache entry, and fetches the updated flag definition. Subsequent evaluations immediately reflect the new value.

### SDK endpoints for push updates

These are the endpoints that SDK clients connect to for receiving real-time flag updates:

| Protocol | Endpoint | Description |
|---|---|---|
| SSE | `GET /api/v1/flags/stream?project_id=<uuid>&environment_id=<uuid>` | Server-Sent Events stream for flag changes |
| gRPC | `FlagService.StreamUpdates` | Bidirectional gRPC stream for flag change notifications |

**SSE event format:**

```
event: flag.changed
data: {"flag_key":"enable-dark-mode","action":"updated","timestamp":"2026-02-21T12:00:00Z"}

event: flag.changed
data: {"flag_key":"enable-dark-mode","action":"toggled","enabled":true,"timestamp":"2026-02-21T12:01:00Z"}

event: flag.changed
data: {"flag_key":"checkout-config","action":"rule_added","rule_id":"<uuid>","timestamp":"2026-02-21T12:02:00Z"}
```

**SSE event `action` values:**

| Action | Description |
|---|---|
| `updated` | Flag metadata (name, description, default value) was changed |
| `toggled` | Flag was enabled or disabled |
| `archived` | Flag was archived |
| `rule_added` | A targeting rule was added to the flag |
| `rule_updated` | A targeting rule was modified |
| `rule_deleted` | A targeting rule was removed |

### NATS subjects

The following NATS JetStream subjects are used internally for the push-update pipeline:

| Subject | Publisher | Consumer | Description |
|---|---|---|---|
| `flag.changed.<project_id>` | API server | Sentinel server | Emitted when any flag property changes |
| `deployment.created` | API server | Notification service | Emitted when a deployment is created |
| `deployment.promoted` | API server | Notification service | Emitted when a deployment is promoted |
| `deployment.rolled_back` | API server | Notification service | Emitted when a deployment is rolled back |

### Webhook push notifications

For services that cannot maintain persistent connections, DeploySentry supports outbound webhooks. Configure a webhook endpoint in the UI to receive HTTP POST callbacks on flag changes:

```json
{
  "id": "1740100000000000000",
  "event_type": "flag.toggled",
  "timestamp": "2026-02-21T12:00:00Z",
  "org_id": "<uuid>",
  "project_id": "<uuid>",
  "data": {
    "flag_key": "enable-dark-mode",
    "enabled": "true"
  }
}
```

Webhook payloads are signed with HMAC-SHA256. Verify the `X-DeploySentry-Signature` header against your webhook secret:

```go
import "github.com/deploysentry/deploysentry/internal/notifications"

valid := notifications.VerifySignature(requestBody, signatureHeader, webhookSecret)
```

## Authentication

All API requests require authentication via one of two methods:

### JWT Bearer Token (for users / UI)

Obtained through the OAuth flow (GitHub or Google). Include in requests as:

```
Authorization: Bearer <jwt-token>
```

### API Key (for services / SDKs)

Generated in the dashboard with scoped permissions. Include in requests as:

```
Authorization: ApiKey <api-key>
```

**Available API key scopes:**

| Scope | Description |
|---|---|
| `flags:read` | Read feature flag configurations |
| `flags:write` | Create and update feature flags |
| `deploys:read` | Read deployment information |
| `deploys:write` | Create and manage deployments |
| `releases:read` | Read release information |
| `releases:write` | Create and manage releases |
| `admin` | Full administrative access (implies all scopes) |

For SDK integration, create an API key with at minimum the `flags:read` scope. If the SDK reports evaluation telemetry, it also needs `flags:write`.

## Evaluation Context and Targeting

When evaluating a flag, the SDK sends an `EvaluationContext` that targeting rules match against:

```json
{
  "user_id": "user-123",
  "org_id": "org-456",
  "attributes": {
    "plan": "enterprise",
    "country": "US",
    "app_version": "2.4.1"
  }
}
```

### Supported rule types

| Rule Type | Description | Key Fields |
|---|---|---|
| `percentage` | Hash-based percentage rollout (deterministic per user+flag) | `percentage` (0-100) |
| `user_target` | Match specific user IDs | `target_values` (list of user IDs) |
| `attribute` | Match context attributes with operators | `attribute`, `operator`, `value` |
| `segment` | Match pre-defined user segments | `segment_id` |
| `schedule` | Time-window activation | `start_time`, `end_time` |

### Attribute operators

| Operator | Description |
|---|---|
| `eq` | Exact string equality |
| `neq` | String inequality |
| `contains` | Substring match |
| `starts_with` | Prefix match |
| `ends_with` | Suffix match |
| `in` | Membership in comma-separated list |
| `gt`, `gte`, `lt`, `lte` | Numeric comparison |

Rules are evaluated in priority order (lower number = higher precedence). The first matching rule determines the returned value. If no rules match, the flag's default value is returned.

## Architecture Overview

```
              +-----------------+                  +------------------+
              |   Dashboard UI  |                  |   Mobile App     |
              |   (React SPA)   |                  |   (Flutter)      |
              +--------+--------+                  +--------+---------+
                       |                                    |
                  HTTPS/REST                           HTTPS/REST
                       |                                    |
                       +----------------+-------------------+
                                        |
                               +--------v--------+
                               | DeploySentry API|
                               |   (Go / Gin)    |
                               +--+---------+--+-+
                                  |         |  |
                         +--------+    +----+  +--------+
                         |             |                 |
                  +------v---+  +-----v-----+  +-------v-------+
                  |PostgreSQL |  |   Redis   |  |     NATS      |
                  | (flags,   |  | (eval     |  |  JetStream    |
                  |  deploys, |  |  cache)   |  | (event bus)   |
                  |  users)   |  +-----------+  +-------+-------+
                  +----------+                          |
                                                 +------v------+
                                                 |  Sentinel   |
                                                 |  Server     |
                                                 +--+--+--+--+-+
                                                    |  |  |  |
                                              SSE/gRPC streams
                                                 |  |  |  |  |
                                       +---------+  |  |  |  +--------+
                                       |            |  |  |            |
                                  +----v---+  +----v--+-+  +----v----+
                                  | Go SDK |  |Node SDK |  |React SDK|
                                  |  svc   |  |  svc    |  |  app    |
                                  +--------+  +---------+  +---------+
                                                     |
                                               +-----v-------+
                                               |Flutter SDK  |
                                               | mobile lib  |
                                               +-------------+
```

The **Mobile App** (Flutter) connects directly to the DeploySentry REST API for admin operations (login, managing deployments, flags, and releases). It is separate from the **Flutter SDK**, which is a library for third-party apps to evaluate feature flags via the Sentinel streaming infrastructure.

**Data flow for a flag evaluation:**

1. SDK checks local in-memory cache (< 1ms).
2. On cache miss, SDK calls `POST /api/v1/flags/evaluate`.
3. API server checks Redis cache. On Redis miss, queries PostgreSQL.
4. Result is cached in Redis (30s TTL) and returned to the SDK.
5. SDK caches the result locally.

**Data flow for a flag update (push):**

1. UI, mobile app, or API call updates the flag in PostgreSQL.
2. Redis cache is invalidated for the changed flag.
3. A `flag.changed` event is published to NATS JetStream.
4. Sentinel server receives the event and broadcasts to connected SDKs.
5. Each SDK invalidates its local cache and fetches the fresh value.
