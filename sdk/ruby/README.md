# DeploySentry Ruby SDK

Ruby SDK for DeploySentry feature flag management with rich metadata, categories, ownership tracking, and real-time updates via SSE streaming.

## Requirements

- Ruby 3.0+
- No external dependencies (stdlib only)

## Installation

Add to your Gemfile:

```ruby
gem "deploysentry"
```

Or install directly:

```sh
gem install deploysentry
```

## Quick Start

```ruby
require "deploysentry"

client = DeploySentry::Client.new(
  api_key:     "ds_live_abc123",
  base_url:    "https://api.dr-sentry.com",
  environment: "production",
  project:     "my-project"
)

client.initialize!

# Simple boolean check
if client.enabled?("dark_mode")
  enable_dark_mode
end

# Typed value access with defaults
theme  = client.string_value("ui_theme", default: "light")
limit  = client.int_value("rate_limit", default: 100)
config = client.json_value("feature_config", default: {})

# Contextual evaluation
context = DeploySentry::EvaluationContext.new(
  user_id: "user-42",
  org_id:  "org-7",
  attributes: { plan: "enterprise", region: "us-east" }
)

enabled = client.bool_value("beta_feature", default: false, context: context)

# Full evaluation detail
result = client.detail("beta_feature", context: context)
puts result.value    # => true
puts result.reason   # => "EVALUATED"
puts result.metadata # => FlagMetadata struct

# Cleanup
client.close
```

## Flag Metadata

Every flag can carry rich metadata:

```ruby
result = client.detail("checkout_v2")
meta = result.metadata

meta.category     # => "release"
meta.purpose      # => "New checkout flow rollout"
meta.owners       # => ["team-payments", "alice@example.com"]
meta.is_permanent # => false
meta.expires_at   # => 2026-06-01 00:00:00 +0000
meta.tags         # => ["payments", "frontend"]
meta.expired?     # => false
```

## Flag Categories

Use `DeploySentry::FlagCategory` constants to filter flags:

```ruby
DeploySentry::FlagCategory::RELEASE    # => "release"
DeploySentry::FlagCategory::FEATURE    # => "feature"
DeploySentry::FlagCategory::EXPERIMENT # => "experiment"
DeploySentry::FlagCategory::OPS        # => "ops"
DeploySentry::FlagCategory::PERMISSION # => "permission"

# Query flags by category
release_flags = client.flags_by_category(DeploySentry::FlagCategory::RELEASE)

# Find expired flags for cleanup
stale = client.expired_flags

# Get flag owners
owners = client.flag_owners("checkout_v2")
```

## Offline Mode

For testing or environments without API access:

```ruby
client = DeploySentry::Client.new(
  api_key:      "test",
  base_url:     "http://localhost:8080",
  environment:  "test",
  project:      "test",
  offline_mode: true
)
```

In offline mode, the client evaluates flags against locally cached state and does not open an SSE connection.

## Caching

The SDK caches evaluation results in a thread-safe in-memory store. The default TTL is 30 seconds, configurable via `cache_timeout:`.

```ruby
client = DeploySentry::Client.new(
  # ...
  cache_timeout: 60  # seconds
)
```

Real-time SSE updates automatically invalidate the cache for changed flags.

## Thread Safety

All public methods on `DeploySentry::Client` are thread-safe. The flag store and cache are protected by `Mutex`. The SSE streaming connection runs on a dedicated background thread with automatic reconnection.

## API Reference

### Client Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `initialize!` | `self` | Fetch flags and start SSE streaming |
| `close` | `nil` | Stop streaming and clear cache |
| `bool_value(key, default:, context:)` | `Boolean` | Evaluate as boolean |
| `string_value(key, default:, context:)` | `String` | Evaluate as string |
| `int_value(key, default:, context:)` | `Integer` | Evaluate as integer |
| `json_value(key, default:, context:)` | `Hash/Array` | Evaluate as parsed JSON |
| `enabled?(key, context:)` | `Boolean` | Shorthand for `bool_value(key, default: false)` |
| `detail(key, context:)` | `EvaluationResult` | Full evaluation with metadata |
| `flags_by_category(category)` | `Array<Flag>` | Filter flags by category |
| `expired_flags` | `Array<Flag>` | Flags past their expiration |
| `flag_owners(key)` | `Array<String>` | Owners of a flag |

## Authentication

All requests use an API key passed in the `Authorization` header:

```
Authorization: ApiKey <your-api-key>
```

Pass the key via the client constructor. The SDK sets the header automatically.

### Session Consistency

Bind evaluations to a session so the server caches results for a consistent user experience:

```ruby
client = DeploySentry::Client.new(
  api_key: 'ds_key_xxxxxxxxxxxx',
  base_url: 'https://api.dr-sentry.com',
  environment: 'production',
  project: 'my-project',
  session_id: "user:#{user_id}",
)
client.refresh_session
```

- The session ID is sent as an `X-DeploySentry-Session` header on every request.
- The server caches evaluation results per session for 30 minutes (sliding TTL).
- Omit the session ID to always get fresh evaluations on each request.

## License

MIT
