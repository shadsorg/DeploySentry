# DeploySentry Go SDK

Official Go SDK for the DeploySentry feature flag platform.
Supports flag evaluation with rich metadata (category, purpose, owners, expiration), real-time streaming updates via SSE, and an in-memory cache with TTL.

## Installation

```bash
go get github.com/deploysentry/deploysentry-go
```

Requires Go 1.22 or later.

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	ds "github.com/deploysentry/deploysentry-go"
)

func main() {
	client := ds.NewClient(
		ds.WithAPIKey("your-api-key"),
		ds.WithProject("project-uuid"),
		ds.WithEnvironment("production"),
		ds.WithCacheTimeout(5*time.Minute),
		ds.WithOfflineMode(true),
	)

	ctx := context.Background()
	if err := client.Initialize(ctx); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}
	defer client.Close()

	// Evaluate a boolean flag
	evalCtx := ds.NewEvaluationContext().
		UserID("user-123").
		OrgID("org-456").
		Set("plan", "enterprise").
		Build()

	enabled, err := client.BoolValue(ctx, "enable-dark-mode", false, evalCtx)
	if err != nil {
		log.Printf("evaluation error: %v", err)
	}
	fmt.Println("dark mode enabled:", enabled)
}
```

## Client Options

| Option               | Description                                         |
|----------------------|-----------------------------------------------------|
| `WithAPIKey`         | API key for authentication (required)               |
| `WithBaseURL`        | Override the default API base URL                   |
| `WithEnvironment`    | Environment identifier (e.g., `"production"`)       |
| `WithProject`        | Project identifier                                  |
| `WithOfflineMode`    | Serve stale cache when the API is unreachable        |
| `WithCacheTimeout`   | TTL for cached flag values (default 5 minutes)      |
| `WithHTTPClient`     | Custom `*http.Client` for API requests              |
| `WithLogger`         | Custom `*log.Logger`                                |

## Evaluation Methods

All evaluation methods accept `(ctx, flagKey, defaultValue, evalContext)`. When the API is unreachable and offline mode is enabled, the client falls back to the last known cached value.

```go
// Boolean
val, err := client.BoolValue(ctx, "my-bool-flag", false, evalCtx)

// String
val, err := client.StringValue(ctx, "my-string-flag", "default", evalCtx)

// Integer
val, err := client.IntValue(ctx, "my-int-flag", 0, evalCtx)

// JSON (unmarshals into target)
var config MyConfig
err := client.JSONValue(ctx, "my-json-flag", &config, evalCtx)

// Full detail with metadata
result, err := client.Detail(ctx, "my-flag", evalCtx)
fmt.Println(result.Reason)                // "TARGETING_MATCH"
fmt.Println(result.Metadata.Category)     // "feature"
fmt.Println(result.Metadata.Purpose)      // "Toggle dark mode UI theme"
fmt.Println(result.Metadata.Owners)       // ["team-frontend"]
```

## Evaluation Context

Build targeting context with the fluent builder:

```go
evalCtx := ds.NewEvaluationContext().
    UserID("user-123").
    OrgID("org-456").
    Set("plan", "enterprise").
    Set("country", "US").
    Build()
```

## Flag Metadata

Every flag carries rich metadata accessible through the `Detail()` method or the cache query helpers:

```go
// Get full evaluation detail
result, _ := client.Detail(ctx, "enable-dark-mode", evalCtx)
fmt.Println(result.Metadata.Category)    // FlagCategory: "feature"
fmt.Println(result.Metadata.Purpose)     // "Toggle dark mode UI theme"
fmt.Println(result.Metadata.Owners)      // ["team-frontend", "jane@example.com"]
fmt.Println(result.Metadata.IsPermanent) // false
fmt.Println(result.Metadata.ExpiresAt)   // 2026-06-01 00:00:00 +0000 UTC
fmt.Println(result.Metadata.Tags)        // ["ui", "theme"]
```

## Cache Queries

Query the local cache without making API calls:

```go
// All flags of a given category
featureFlags := client.FlagsByCategory(ds.CategoryFeature)

// All flags past their expiration date
expired := client.ExpiredFlags()

// Owners for a specific flag
owners := client.FlagOwners("enable-dark-mode")

// All cached flags
all := client.AllFlags()
```

## Flag Categories

The SDK defines five flag categories:

| Constant              | Value          | Typical Use                          |
|-----------------------|----------------|--------------------------------------|
| `CategoryRelease`     | `"release"`    | Gradual rollout of new releases      |
| `CategoryFeature`     | `"feature"`    | Feature toggles                      |
| `CategoryExperiment`  | `"experiment"` | A/B tests and experiments            |
| `CategoryOps`         | `"ops"`        | Operational kill switches            |
| `CategoryPermission`  | `"permission"` | Entitlement and permission gates     |

## Real-Time Updates

After calling `Initialize()`, the client automatically connects to the SSE streaming endpoint and updates the cache in real time. If the connection drops, it reconnects with exponential backoff (1s to 60s).

## Offline Mode

When `WithOfflineMode(true)` is set:

- `Initialize()` will not return an error if the initial cache warm fails.
- Evaluation methods return the last known cached value (with reason `"STALE_CACHE"`) when the API is unreachable.

## API Endpoints

The SDK communicates with the following DeploySentry API endpoints:

| Method | Path                          | Purpose                    |
|--------|-------------------------------|----------------------------|
| GET    | `/api/v1/flags`               | List all flags (cache warm)|
| POST   | `/api/v1/flags/evaluate`      | Single flag evaluation     |
| POST   | `/api/v1/flags/batch-evaluate`| Batch flag evaluation      |
| GET    | `/api/v1/flags/stream`        | SSE real-time updates      |

All requests include the header `Authorization: ApiKey <key>`.

## License

Apache-2.0
