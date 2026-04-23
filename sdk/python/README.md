# DeploySentry Python SDK

Official Python SDK for integrating with the DeploySentry feature-flag platform. Supports synchronous and asynchronous usage, real-time flag updates via SSE, in-memory caching, and rich flag metadata.

## Installation

```bash
pip install deploysentry
```

## Quick Start

### Synchronous

```python
from deploysentry import DeploySentryClient, EvaluationContext

client = DeploySentryClient(
    api_key="ds_your_api_key",
    base_url="https://api.dr-sentry.com",
    environment="production",
    project="my-project",
)
client.initialize()

# Boolean evaluation
if client.bool_value("dark-mode", default=False):
    enable_dark_mode()

# Evaluation with user context
ctx = EvaluationContext(user_id="user-123", org_id="acme", attributes={"plan": "pro"})
limit = client.int_value("rate-limit", default=100, context=ctx)

# Full detail including metadata
result = client.detail("new-checkout")
print(result.enabled, result.reason, result.metadata.category)

# Cleanup
client.close()
```

### Context Manager

```python
with DeploySentryClient(api_key="ds_key", project="my-project") as client:
    val = client.string_value("banner-text", default="Welcome!")
```

### Asynchronous

```python
import asyncio
from deploysentry import AsyncDeploySentryClient, EvaluationContext

async def main():
    async with AsyncDeploySentryClient(
        api_key="ds_key",
        project="my-project",
    ) as client:
        enabled = await client.bool_value("new-feature", default=False)
        config = await client.json_value("ui-config", default={})
        print(enabled, config)

asyncio.run(main())
```

## Status reporting (optional)

Enable `report_status` to have the SDK push version + health to DeploySentry automatically. No separate startup code needed — the SDK posts `POST /applications/:id/status` on `initialize()` and on an interval.

```python
from deploysentry import DeploySentryClient, HealthReport

client = DeploySentryClient(
    api_key=os.environ["DS_API_KEY"],
    base_url="https://api.dr-sentry.com",
    environment="production",
    project="my-project",

    # Agentless status reporting
    application_id=os.environ["DS_APPLICATION_ID"],   # UUID
    report_status=True,
    report_status_interval=30.0,                       # seconds; 0 = startup-only
    report_status_version=os.environ.get("APP_VERSION"),
    report_status_commit_sha=os.environ.get("GIT_SHA"),
    report_status_deploy_slot=os.environ.get("DS_DEPLOY_SLOT"),
    report_status_tags={"region": os.environ.get("REGION", "unknown")},
    report_status_health_provider=lambda: HealthReport(
        state="healthy" if db_up() else "degraded",
        score=0.99,
    ),
)
client.initialize()
```

The API key must carry the `status:write` scope and be scoped to a single application + environment. Without a `report_status_health_provider`, the SDK sends `state="healthy"` on every tick (the "process alive" floor). Version auto-detection checks `APP_VERSION`, `GIT_SHA`, `GIT_COMMIT`, `SOURCE_COMMIT`, `RAILWAY_GIT_COMMIT_SHA`, `RENDER_GIT_COMMIT`, `VERCEL_GIT_COMMIT_SHA`, `HEROKU_SLUG_COMMIT` in order and falls back to the literal `"unknown"`.

## Evaluation Methods

| Method | Return type | Description |
|--------|-------------|-------------|
| `bool_value(key, default, context)` | `bool` | Boolean flag evaluation |
| `string_value(key, default, context)` | `str` | String flag evaluation |
| `int_value(key, default, context)` | `int` | Integer flag evaluation |
| `json_value(key, default, context)` | `Any` | JSON/dict/list flag evaluation |
| `detail(key, context)` | `EvaluationResult` | Full result with metadata |

## Flag Metadata

Every flag carries rich metadata:

```python
from deploysentry import FlagCategory

# Filter flags by category
release_flags = client.flags_by_category(FlagCategory.RELEASE)
experiment_flags = client.flags_by_category(FlagCategory.EXPERIMENT)

# Find flags past their expiration date
stale = client.expired_flags()

# Get owners for a specific flag
owners = client.flag_owners("checkout-v2")
```

### Flag Categories

- `RELEASE` -- gradual rollout gates
- `FEATURE` -- product feature toggles
- `EXPERIMENT` -- A/B tests and experiments
- `OPS` -- operational/infrastructure flags
- `PERMISSION` -- access-control flags

### FlagMetadata Fields

| Field | Type | Description |
|-------|------|-------------|
| `category` | `FlagCategory` | Categorisation of the flag |
| `purpose` | `str` | Human-readable purpose |
| `owners` | `list[str]` | Team or individual owners |
| `is_permanent` | `bool` | Whether the flag is permanent |
| `expires_at` | `datetime | None` | Optional expiry timestamp |
| `tags` | `list[str]` | Arbitrary tags |

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `api_key` | *required* | API key (`ApiKey` auth header) |
| `base_url` | `https://api.dr-sentry.com` | API base URL |
| `environment` | `"production"` | Target environment |
| `project` | `""` | Project identifier |
| `cache_timeout` | `30` | Cache TTL in seconds (0 to disable) |
| `offline_mode` | `False` | Skip all network requests |

## Offline Mode

Set `offline_mode=True` to disable all network calls. Every evaluation returns the provided default value. Useful for local development and testing.

```python
client = DeploySentryClient(api_key="unused", offline_mode=True)
client.initialize()
assert client.bool_value("any-flag", default=True) is True
```

## Real-Time Updates

The SDK automatically opens an SSE connection to `/api/v1/flags/stream` after `initialize()`. Flag changes are applied immediately and the evaluation cache is invalidated for updated keys. The SSE client reconnects automatically with exponential backoff on connection loss.

## Authentication

All requests use an API key passed in the `Authorization` header:

```
Authorization: ApiKey <your-api-key>
```

Pass the key via the client constructor. The SDK sets the header automatically.

### Session Consistency

Bind evaluations to a session so the server caches results for a consistent user experience:

```python
client = DeploySentryClient(
    api_key="ds_key_xxxxxxxxxxxx",
    environment="production",
    project="my-project",
    session_id=f"user:{user_id}",
)
client.refresh_session()
```

- The session ID is sent as an `X-DeploySentry-Session` header on every request.
- The server caches evaluation results per session for 30 minutes (sliding TTL).
- Omit the session ID to always get fresh evaluations on each request.

## License

Apache-2.0
