# Documentation Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update README.md with complete API schemas, SSE protocol, and session consistency docs. Fix SDK READMEs.

**Architecture:** Add new sections to main README. Update 7 SDK READMEs with session examples and auth header corrections.

**Tech Stack:** Markdown

**Spec:** `docs/superpowers/specs/2026-03-27-deploysentry-production-readiness-design.md` (Section 9)

---

## Key Facts (verified from source)

- Evaluate endpoint struct: `evaluateRequest` — fields: `project_id`, `environment_id`, `flag_key`, `context` (`user_id`, `org_id`, `attributes`). `session_id` is to be added per spec.
- Batch evaluate struct: `batchEvaluateRequest` — fields: `project_id`, `environment_id`, `flag_keys`, `context`. `session_id` is to be added per spec.
- Batch response key: `"results"` (not `"evaluations"`) — confirmed from `handler.go` line 339.
- SSE event name: `flag_change` — confirmed from `handler.go` line 510: `c.SSEvent("flag_change", msg)`.
- SSE data payload shape: `{"event":"flag.toggled","flag_id":"...","enabled":true}` — from `handler.go` line 258. `flag_key` field is to be added.
- `FlagEvaluationResult` model fields: `flag_key`, `enabled`, `value`, `reason`, `rule_id`, `variation_id`, `metadata` (no `flag_type`).
- `FlagMetadata` model fields: `category`, `purpose`, `owners`, `is_permanent`, `expires_at`, `tags`.
- Rule management routes confirmed in `handler.go` lines 49–52: POST/PUT/DELETE on `/:id/rules` and `/:id/rules/:ruleId`.
- Auth header bug: Java uses `"Bearer "`, React uses `Bearer`, Flutter uses `'Bearer $apiKey'` — all must become `ApiKey`. Go, Node, Python, Ruby already correct.

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `README.md` | Modify | Add evaluation API schemas, SSE protocol, rule management, session consistency, rollout algorithm |
| `sdk/java/README.md` | Modify | Fix auth header doc (`Bearer` → `ApiKey`), add session consistency example |
| `sdk/react/README.md` | Modify | Fix auth header doc (`Bearer` → `ApiKey`), add session consistency example |
| `sdk/flutter/README.md` | Modify | Fix auth header doc (`Bearer` → `ApiKey`), add session consistency example |
| `sdk/go/README.md` | Modify | Add session consistency example |
| `sdk/node/README.md` | Modify | Add session consistency example |
| `sdk/python/README.md` | Modify | Add session consistency example |
| `sdk/ruby/README.md` | Modify | Add session consistency example |

---

## Task 1: Add Evaluation API Schemas to README

**File:** `README.md`

**Insertion point:** After the existing "API Endpoints for Monitoring" table (around line 506), before the "Observability Integration" section. Add a new section `### Evaluation API`.

- [ ] **Step 1: Add `### Evaluation API` section to README**

Insert the following section between "API Endpoints for Monitoring" and "Observability Integration":

```markdown
### Evaluation API

#### `POST /api/v1/flags/evaluate`

Evaluate a single flag for a given user context. Requires `flags:read` scope.

**Request:**
```json
{
  "flag_key": "new-checkout-flow",
  "project_id": "<project-uuid>",
  "environment_id": "<environment-uuid>",
  "context": {
    "user_id": "user-123",
    "org_id": "org-456",
    "attributes": {
      "plan": "enterprise",
      "region": "us-east-1"
    }
  },
  "session_id": "user:user-123"
}
```

`session_id` is optional. When provided, the server returns a cached result for the duration of the session (see [Session Consistency](#session-consistency)).

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
    "tags": ["payments", "checkout"],
    "expires_at": "2026-04-21T00:00:00Z"
  }
}
```

#### `POST /api/v1/flags/batch-evaluate`

Evaluate multiple flags in a single request. Requires `flags:read` scope.

**Request:**
```json
{
  "flag_keys": ["new-checkout-flow", "dark-mode", "max-upload-size"],
  "project_id": "<project-uuid>",
  "environment_id": "<environment-uuid>",
  "context": {
    "user_id": "user-123",
    "org_id": "org-456",
    "attributes": { "plan": "enterprise" }
  },
  "session_id": "user:user-123"
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
      "metadata": { "category": "release", "owners": ["team-payments"], "expires_at": "2026-04-21T00:00:00Z" }
    },
    {
      "flag_key": "dark-mode",
      "value": "false",
      "enabled": false,
      "reason": "DEFAULT_VALUE",
      "metadata": { "category": "feature", "owners": ["team-frontend"] }
    }
  ]
}
```

#### Evaluation Reason Values

| Reason | Description |
|--------|-------------|
| `TARGETING_MATCH` | A targeting rule matched (user_target, attribute, segment, or schedule rule) |
| `PERCENTAGE_ROLLOUT` | A percentage rollout rule matched for this user |
| `DEFAULT_VALUE` | No rules matched; the flag's default value was returned |
| `FLAG_DISABLED` | The flag exists but is toggled off |
| `NOT_FOUND` | No flag with the given key exists in this project/environment |
| `ERROR` | An error occurred during evaluation; default value was returned |
| `SESSION_CACHED` | The result was served from session cache (see Session Consistency) |
```

---

## Task 2: Add SSE Protocol Docs to README

**File:** `README.md`

**Insertion point:** After the new "Evaluation API" section added in Task 1, before "Observability Integration". Add a new section `### SSE Streaming Protocol`.

- [ ] **Step 2: Add `### SSE Streaming Protocol` section to README**

```markdown
### SSE Streaming Protocol

SDK clients receive real-time flag updates via Server-Sent Events (SSE).

**Endpoint:** `GET /api/v1/flags/stream?project_id=<uuid>&token=<jwt>`

The `token` query parameter accepts a JWT (dashboard/mobile) or API key (`ApiKey <key>` value).

**Event format:**

```
event: flag_change
data: {"event": "flag.toggled", "flag_id": "abc123", "flag_key": "new-checkout-flow", "enabled": true}

event: flag_change
data: {"event": "flag.updated", "flag_id": "abc123", "flag_key": "new-checkout-flow", "enabled": true}
```

**Event types in the data payload:**

| `event` field | Trigger |
|---------------|---------|
| `flag.toggled` | Flag was enabled or disabled |
| `flag.created` | A new flag was created |
| `flag.updated` | Flag configuration was changed |
| `flag.deleted` | Flag was deleted or archived |

**Heartbeat:** The server sends `: heartbeat` every 15 seconds to keep the connection alive through proxies and load balancers.

**Reconnection:** SDKs reconnect with exponential backoff on disconnect:
- Initial delay: 1 second
- Maximum delay: 30 seconds
- Backoff factor: 2x per retry
- Jitter: +/- 20% to prevent thundering herd
```

---

## Task 3: Add Rule Management Endpoints to README

**File:** `README.md`

**Insertion point:** In the existing "API Endpoints for Monitoring" table, or immediately after it as a new subsection `### Rule Management Endpoints`. Either extend the table or add a sibling section. Prefer a sibling section to keep the monitoring table focused.

- [ ] **Step 3: Add `### Rule Management Endpoints` section to README**

Add after the "API Endpoints for Monitoring" table:

```markdown
### Rule Management Endpoints

| Method | Endpoint | Description | Scope |
|--------|----------|-------------|-------|
| `POST` | `/api/v1/flags/:id/rules` | Add a targeting rule to a flag | `flags:write` |
| `PUT` | `/api/v1/flags/:id/rules/:ruleId` | Update an existing targeting rule | `flags:write` |
| `DELETE` | `/api/v1/flags/:id/rules/:ruleId` | Delete a targeting rule | `flags:write` |

**Add rule request body (`POST /api/v1/flags/:id/rules`):**
```json
{
  "rule_type": "percentage",
  "priority": 1,
  "percentage": 25,
  "value": "true"
}
```

Rule types and their required fields:

| `rule_type` | Required fields | Optional fields |
|-------------|----------------|-----------------|
| `percentage` | `percentage` (0–100), `value` | `priority` |
| `user_target` | `target_values` (list of user IDs), `value` | `priority` |
| `attribute` | `attribute`, `operator`, `target_values`, `value` | `priority` |
| `segment` | `segment_id`, `value` | `priority` |
| `schedule` | `start_time`, `end_time`, `value` | `priority` |

**Attribute operators:** `eq`, `neq`, `contains`, `starts_with`, `ends_with`, `in`, `gt`, `gte`, `lt`, `lte`

Rules are evaluated in `priority` order — lower number wins. First matching rule determines the evaluated value.
```

---

## Task 4: Add Session Consistency Docs to README

**File:** `README.md`

**Insertion point:** After "SDK Behavior" section (around line 454), before the "---" separator. Add a new section `### Session Consistency`.

- [ ] **Step 4: Add `### Session Consistency` section to README**

```markdown
### Session Consistency

By default, flag evaluations are always fresh. Session consistency lets you pin flag values for the duration of a user session, so a user does not see a flag change mid-session.

**How it works:**

1. SDK sends a `session_id` with each evaluation request (via `X-DeploySentry-Session` header and/or request body)
2. Server checks Redis for a cached result under `flag_session:{session_id}:{flag_key}`
3. Cache hit — returns the cached result, even if the underlying flag has changed (reason: `SESSION_CACHED`)
4. Cache miss — evaluates fresh, caches the result with a sliding 30-minute TTL
5. SSE updates are still received by the SDK but take effect on the next session refresh

**Session TTL:** 30 minutes (sliding, reset on each evaluation). Configurable via `DS_SESSION_TTL`.

**Opting in (all SDKs):** Session consistency is opt-in. If no session ID is configured, the SDK does not send `X-DeploySentry-Session` and fresh evaluation is always used.

**Session ID composition examples:**

| Use case | Session ID |
|----------|-----------|
| Per-user (default) | `"user:" + userID` |
| Per-user per app version | `appVersion + ":" + userID` |
| Per-user per region | `userID + ":" + region` |
| Custom composite | `userID + ":" + appVersion + ":" + region` |

**Refreshing a session:** Call `refreshSession()` on the client to clear both the local cache and the server-side session cache, forcing fresh evaluations on the next call.

**Go example:**
```go
// Opt-in to session consistency
client := deploysentry.NewClient(
    deploysentry.WithAPIKey("ds_key_xxxxxxxxxxxx"),
    deploysentry.WithEnvironment("production"),
    deploysentry.WithProject("my-project"),
    deploysentry.WithSessionID("user:" + userID),
)

// Later, refresh the session (e.g. after login or major user action)
client.RefreshSession(ctx)
```

**Node.js / TypeScript example:**
```typescript
const client = new DeploySentryClient({
  apiKey: 'ds_key_xxxxxxxxxxxx',
  environment: 'production',
  project: 'my-project',
  sessionId: `user:${userId}`,
});

// Refresh after login
await client.refreshSession();
```
```

---

## Task 5: Fix SDK READMEs (Auth Headers + Session Consistency Examples)

**Files:** All 7 SDK READMEs

The Java, React, and Flutter READMEs incorrectly document the auth header as `Bearer` instead of `ApiKey`. All 7 SDK READMEs also need a session consistency example added.

### 5a: Java SDK README

**File:** `sdk/java/README.md`

- [ ] **Step 5a-1: Fix auth header documentation in Java README**

The Java README does not currently show the auth header explicitly in prose, but the SDK implementation uses `"Bearer "`. Add a new "Authentication" section after "Configuration Options" that documents the correct header:

```markdown
## Authentication

All requests use an API key passed in the `Authorization` header:

```
Authorization: ApiKey <your-api-key>
```

Pass the key via `ClientOptions.builder().apiKey("ds_key_xxxxxxxxxxxx")`. The SDK sets the header automatically.
```

- [ ] **Step 5a-2: Add session consistency example to Java README**

Add after the "Authentication" section:

```markdown
## Session Consistency

To receive stable flag values for the duration of a user session, configure a session ID:

```java
ClientOptions options = ClientOptions.builder()
        .apiKey("ds_live_abc123")
        .environment("production")
        .project("my-app")
        .sessionId("user:" + userId)
        .build();

try (var client = new DeploySentryClient(options)) {
    client.initialize();
    // All evaluations are pinned for this session
    boolean enabled = client.boolValue("new-checkout-flow", false, ctx);

    // Force fresh evaluations (e.g. after login)
    client.refreshSession();
}
```

The session ID is sent as `X-DeploySentry-Session: user:<userId>`. The server caches results for 30 minutes (sliding TTL). Omit `sessionId` to always get fresh evaluations.
```

### 5b: React SDK README

**File:** `sdk/react/README.md`

- [ ] **Step 5b-1: Fix auth header documentation in React README**

Add an "Authentication" section after the "Provider Props" table:

```markdown
## Authentication

All API requests use the `Authorization: ApiKey <key>` header. Pass your key via the `apiKey` prop on `DeploySentryProvider` — the SDK sets the header automatically.

```tsx
<DeploySentryProvider
  apiKey="ds_key_xxxxxxxxxxxx"  // sent as: Authorization: ApiKey ds_key_xxxxxxxxxxxx
  ...
>
```
```

- [ ] **Step 5b-2: Add session consistency example to React README**

Add after the existing "Real-Time Updates" section:

```markdown
## Session Consistency

To pin flag values for the duration of a user session, pass `sessionId` to the provider:

```tsx
<DeploySentryProvider
  apiKey="ds_key_xxxxxxxxxxxx"
  baseURL="https://deploysentry.example.com"
  environment="production"
  project="my-app"
  user={{ userId: 'user-123', attributes: { plan: 'enterprise' } }}
  sessionId={`user:${userId}`}
>
  <App />
</DeploySentryProvider>
```

The session ID is sent as `X-DeploySentry-Session` with each evaluation. The server caches results for 30 minutes (sliding TTL). To force fresh evaluations (e.g. on logout/login), call:

```tsx
const client = useDeploySentry();
await client.refreshSession();
```

Omit `sessionId` to always receive fresh flag evaluations.
```

### 5c: Flutter SDK README

**File:** `sdk/flutter/README.md`

- [ ] **Step 5c-1: Fix auth header documentation in Flutter README**

Add an "Authentication" section after the "API Endpoints" table:

```markdown
## Authentication

All requests use the `Authorization: ApiKey <key>` header. Pass your key via the `apiKey` constructor parameter — the SDK sets the header automatically.

```dart
final client = DeploySentryClient(
  apiKey: 'ds_key_xxxxxxxxxxxx',  // sent as: Authorization: ApiKey ds_key_xxxxxxxxxxxx
  baseUrl: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
);
```
```

- [ ] **Step 5c-2: Add session consistency example to Flutter README**

Add after the new "Authentication" section:

```markdown
## Session Consistency

To pin flag values for the duration of a user session, pass a `sessionId`:

```dart
final client = DeploySentryClient(
  apiKey: 'ds_key_xxxxxxxxxxxx',
  baseUrl: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
  sessionId: 'user:$userId',
);
await client.initialize();

// All evaluations are stable for this session
final enabled = await client.boolValue('new-checkout-flow', defaultValue: false);

// Force fresh evaluations (e.g. after login)
await client.refreshSession();
```

The session ID is sent as `X-DeploySentry-Session`. The server caches results for 30 minutes (sliding TTL). Omit `sessionId` to always get fresh evaluations.
```

### 5d: Go SDK README

**File:** `sdk/go/README.md`

- [ ] **Step 5d: Add session consistency example to Go README**

Find the existing "Configuration" or options section and add a "Session Consistency" subsection. Add after the section describing `WithAPIKey` / initialization options:

```markdown
### Session Consistency

To receive stable flag values for the duration of a user session, set a session ID at client creation:

```go
client := ds.NewClient(
    ds.WithAPIKey("ds_key_xxxxxxxxxxxx"),
    ds.WithEnvironment("production"),
    ds.WithProject("my-project"),
    ds.WithSessionID("user:" + userID),
)
defer client.Close()

client.Initialize()

// All evaluations are pinned for this session
enabled := client.BoolValue(ctx, "new-checkout-flow", false, evalCtx)

// Force fresh evaluations (e.g. after login or major user action)
client.RefreshSession(ctx)
```

The session ID is sent as `X-DeploySentry-Session`. The server caches results for 30 minutes (sliding TTL). Omit `WithSessionID` to always get fresh evaluations.
```

### 5e: Node.js SDK README

**File:** `sdk/node/README.md`

- [ ] **Step 5e: Add session consistency example to Node README**

Add a "Session Consistency" section after the existing "Real-time Updates" or "Configuration" section:

```markdown
### Session Consistency

To receive stable flag values for the duration of a user session, set a session ID:

```typescript
const client = new DeploySentryClient({
  apiKey: 'ds_key_xxxxxxxxxxxx',
  baseURL: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
  sessionId: `user:${userId}`,
});
await client.initialize();

// All evaluations are pinned for this session
const enabled = await client.boolValue('new-checkout-flow', false, { userId });

// Force fresh evaluations (e.g. after login)
await client.refreshSession();
```

The session ID is sent as `X-DeploySentry-Session`. The server caches results for 30 minutes (sliding TTL). Omit `sessionId` to always get fresh evaluations.
```

### 5f: Python SDK README

**File:** `sdk/python/README.md`

- [ ] **Step 5f: Add session consistency example to Python README**

Add a "Session Consistency" section:

```markdown
### Session Consistency

To receive stable flag values for the duration of a user session, pass a `session_id`:

```python
client = DeploySentryClient(
    api_key="ds_key_xxxxxxxxxxxx",
    base_url="https://deploysentry.example.com",
    environment="production",
    project="my-project",
    session_id=f"user:{user_id}",
)
client.initialize()

# All evaluations are pinned for this session
enabled = client.bool_value("new-checkout-flow", default=False, context=ctx)

# Force fresh evaluations (e.g. after login)
client.refresh_session()
```

The session ID is sent as `X-DeploySentry-Session`. The server caches results for 30 minutes (sliding TTL). Omit `session_id` to always get fresh evaluations.
```

### 5g: Ruby SDK README

**File:** `sdk/ruby/README.md`

- [ ] **Step 5g: Add session consistency example to Ruby README**

Add a "Session Consistency" section:

```markdown
### Session Consistency

To receive stable flag values for the duration of a user session, pass a `session_id`:

```ruby
client = DeploySentry::Client.new(
  api_key: 'ds_key_xxxxxxxxxxxx',
  base_url: 'https://deploysentry.example.com',
  environment: 'production',
  project: 'my-project',
  session_id: "user:#{user_id}",
)
client.initialize!

# All evaluations are pinned for this session
enabled = client.bool_value('new-checkout-flow', default: false, context: ctx)

# Force fresh evaluations (e.g. after login)
client.refresh_session
```

The session ID is sent as `X-DeploySentry-Session`. The server caches results for 30 minutes (sliding TTL). Omit `session_id` to always get fresh evaluations.
```

---

## Task 6: Add Percentage Rollout Algorithm Docs to README

**File:** `README.md`

**Insertion point:** After the existing "Targeting Rules" table and operators list (around line 168), as a new subsection `#### Percentage Rollout Algorithm`.

- [ ] **Step 6: Add percentage rollout algorithm docs to README**

Add immediately after the "Attribute operators" line in the Targeting Rules section:

```markdown
#### Percentage Rollout Algorithm

Percentage rollouts are deterministic — the same user always lands in the same bucket for the same flag, regardless of when evaluation happens.

**Algorithm:**

1. Compute `SHA256("{flag_key}:{user_id}")` — e.g. `SHA256("new-checkout-flow:user-123")`
2. Take the first 8 bytes of the hash and convert to a uint64
3. Map to 0–100 bucket: `bucket = uint64Value % 101`
4. If `bucket <= percentage`, the rule matches and the flag's `value` is returned

This ensures:
- **Consistency:** The same user always sees the same result for a given flag
- **Uniform distribution:** Users are spread evenly across buckets
- **Independence:** Rolling out flag A does not affect which bucket a user lands in for flag B (different hash input)

**Example:** `new-checkout-flow` at 25% — user `user-123` either always sees the flag or never does, for the lifetime of the flag configuration.
```
