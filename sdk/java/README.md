# DeploySentry Java SDK

Official Java SDK for the DeploySentry feature-flag and deployment platform. Requires Java 11+.

## Installation

### Maven

```xml
<dependency>
    <groupId>io.deploysentry</groupId>
    <artifactId>deploysentry-java</artifactId>
    <version>1.0.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'io.deploysentry:deploysentry-java:1.0.0'
```

## Quick Start

```java
import io.deploysentry.*;

ClientOptions options = ClientOptions.builder()
        .apiKey("ds_live_abc123")
        .environment("production")
        .project("my-app")
        .build();

try (DeploySentryClient client = new DeploySentryClient(options)) {
    client.initialize();

    EvaluationContext ctx = EvaluationContext.builder()
            .userId("user-42")
            .orgId("org-7")
            .attribute("plan", "enterprise")
            .build();

    // Boolean evaluation
    boolean darkMode = client.boolValue("dark-mode", false, ctx);

    // String evaluation
    String banner = client.stringValue("banner-text", "Welcome!", ctx);

    // Integer evaluation
    int maxItems = client.intValue("max-items", 10, ctx);

    // JSON evaluation
    String config = client.jsonValue("ui-config", "{}", ctx);
}
```

## Detailed Evaluation

Use `detail()` to retrieve a full `EvaluationResult` that includes the resolved value, variant, reason, and flag metadata:

```java
EvaluationResult<Boolean> result = client.detail("dark-mode", ctx);

result.getValue();      // resolved value
result.getVariant();    // matched variant name
result.getReason();     // e.g. "TARGETING_MATCH", "DISABLED", "FLAG_NOT_FOUND"
result.isDefaulted();   // true when the flag was not found
result.getMetadata();   // FlagMetadata with category, owners, etc.
```

## Flag Metadata

Every flag carries rich metadata describing its purpose and lifecycle:

```java
FlagMetadata meta = result.getMetadata();

meta.getCategory();   // FlagCategory enum: RELEASE, FEATURE, EXPERIMENT, OPS, PERMISSION
meta.getPurpose();    // human-readable description
meta.getOwners();     // list of owners (teams or individuals)
meta.isPermanent();   // whether the flag is meant to live indefinitely
meta.getExpiresAt();  // optional expiration timestamp
meta.getTags();       // arbitrary key-value tags
meta.isExpired();     // convenience check against the current time
```

## Querying Flags

```java
// All flags in a category
List<Flag> experiments = client.flagsByCategory(FlagCategory.EXPERIMENT);

// Flags past their expiration date
List<Flag> stale = client.expiredFlags();

// Owners for a specific flag
List<String> owners = client.flagOwners("checkout-v2");
```

## Configuration Options

| Option | Default | Description |
|---|---|---|
| `apiKey` | *required* | API key for authentication |
| `baseURL` | `https://api.dr-sentry.com` | API base URL |
| `environment` | `null` | Target environment (e.g. `production`) |
| `project` | `null` | Project identifier |
| `cacheTimeout` | 5 minutes | TTL for cached flag data |
| `connectTimeout` | 10 seconds | HTTP connection timeout |
| `enableSSE` | `true` | Subscribe to real-time flag updates |

```java
ClientOptions options = ClientOptions.builder()
        .apiKey("ds_live_abc123")
        .baseURL("https://custom.deploysentry.io")
        .environment("staging")
        .project("payments")
        .cacheTimeout(Duration.ofMinutes(2))
        .connectTimeout(Duration.ofSeconds(5))
        .enableSSE(true)
        .build();
```

## Real-Time Updates

When SSE is enabled (the default), the client opens a persistent streaming connection to the DeploySentry API. Flag changes are applied to the local cache immediately, so subsequent evaluations reflect the latest state without polling.

SSE reconnects automatically with exponential back-off on transient failures.

## Thread Safety

The SDK is safe to share across threads. The internal flag cache is backed by `ConcurrentHashMap`, and the SSE listener runs on a dedicated daemon thread.

## Authentication

All requests use an API key passed in the `Authorization` header:

```
Authorization: ApiKey <your-api-key>
```

Pass the key via the client constructor. The SDK sets the header automatically.

### Session Consistency

Bind evaluations to a session so the server caches results for a consistent user experience:

```java
ClientOptions options = ClientOptions.builder()
        .apiKey("ds_key_xxxxxxxxxxxx")
        .environment("production")
        .project("my-app")
        .sessionId("user:" + userId)
        .build();
client.refreshSession();
```

- The session ID is sent as an `X-DeploySentry-Session` header on every request.
- The server caches evaluation results per session for 30 minutes (sliding TTL).
- Omit the session ID to always get fresh evaluations on each request.

## License

Apache-2.0
