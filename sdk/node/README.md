# DeploySentry Node.js SDK

Official Node.js/TypeScript SDK for integrating with the DeploySentry platform. Provides feature flag evaluation with rich metadata including categories, ownership, expiration tracking, and real-time updates via SSE.

## Installation

```bash
npm install @dr-sentry/sdk
```

## Quick Start

```typescript
import { DeploySentryClient } from '@dr-sentry/sdk';

const client = new DeploySentryClient({
  apiKey: 'ds_live_xxx',
  environment: 'production',
  project: 'my-app',
  application: 'my-web-app',
});

await client.initialize();

// Boolean flag
const darkMode = await client.boolValue('dark-mode', false, {
  userId: 'user-42',
});

// String flag
const variant = await client.stringValue('checkout-variant', 'control');

// Integer flag
const maxRetries = await client.intValue('max-retries', 3);

// JSON flag
const config = await client.jsonValue('rate-limits', { rpm: 100 });

// Clean up when done
client.close();
```

## Status reporting (optional)

Enable `reportStatus` to have the SDK push version + health to DeploySentry automatically. No separate startup code needed — the SDK posts `POST /applications/:id/status` on init and on an interval.

```typescript
const client = new DeploySentryClient({
  apiKey: process.env.DS_API_KEY!,
  environment: 'production',
  project: 'my-app',
  application: 'my-web-app',

  // Agentless status reporting
  applicationId: process.env.DS_APPLICATION_ID!, // UUID of the app
  reportStatus: true,
  reportStatusIntervalMs: 30_000,                // default; 0 = startup-only
  reportStatusVersion: process.env.APP_VERSION,   // optional override
  reportStatusCommitSha: process.env.GIT_SHA,
  reportStatusDeploySlot: process.env.DS_DEPLOY_SLOT, // "stable" | "canary"
  reportStatusTags: { region: process.env.REGION ?? 'unknown' },
  reportStatusHealthProvider: async () => ({
    state: (await isDbUp()) ? 'healthy' : 'degraded',
    score: 0.99,
  }),
});

await client.initialize();
```

The API key must carry the `status:write` scope and be scoped to a single application + environment. When `reportStatus` is `true` but `applicationId` is omitted, the reporter logs a warning and is disabled — flag evaluation continues to work.

Without a `reportStatusHealthProvider`, the SDK sends `health: "healthy"` on every tick ("process alive" floor). Supply a provider if your app can detect degradation (stale DB, downstream outages, etc.).

Version auto-detection (when `reportStatusVersion` is omitted) checks these env vars in order: `APP_VERSION`, `GIT_SHA`, `GIT_COMMIT`, `SOURCE_COMMIT`, `RAILWAY_GIT_COMMIT_SHA`, `RENDER_GIT_COMMIT`, `VERCEL_GIT_COMMIT_SHA`, `HEROKU_SLUG_COMMIT`, `npm_package_version` — falling back to the literal string `"unknown"`.

The first `/status` report with a version DeploySentry has not seen auto-creates a `mode=record` deployment with `source="app-push"`, so deploy history populates even without a Railway/Render/etc. webhook configured.

## Evaluation Context

Pass targeting context with every evaluation:

```typescript
const enabled = await client.boolValue('premium-feature', false, {
  userId: 'user-42',
  orgId: 'org-7',
  attributes: {
    plan: 'enterprise',
    region: 'us-east-1',
  },
});
```

## Rich Metadata

Every flag carries structured metadata describing its purpose, ownership, and lifecycle:

```typescript
const result = await client.detail('new-checkout');

console.log(result.metadata.category);   // 'release' | 'feature' | 'experiment' | 'ops' | 'permission'
console.log(result.metadata.purpose);    // "Gradual rollout of new checkout flow"
console.log(result.metadata.owners);     // ["payments-team", "jane@example.com"]
console.log(result.metadata.isPermanent); // false
console.log(result.metadata.expiresAt);  // "2026-06-01T00:00:00Z"
console.log(result.metadata.tags);       // ["checkout", "q2-2026"]
```

## Flag Categories

Flags are classified into categories that drive lifecycle policies:

| Category     | Description                                        |
| ------------ | -------------------------------------------------- |
| `release`    | Gradual rollout gates for new releases              |
| `feature`    | Long-lived feature toggles                          |
| `experiment` | A/B tests and experiments with defined end dates    |
| `ops`        | Operational controls (kill switches, rate limits)   |
| `permission` | Entitlement and access-control flags                |

Query by category:

```typescript
const experiments = client.flagsByCategory('experiment');
const killSwitches = client.flagsByCategory('ops');
```

## Lifecycle Management

Find expired flags that should be cleaned up:

```typescript
const expired = client.expiredFlags();
for (const flag of expired) {
  console.warn(`Flag "${flag.key}" expired at ${flag.metadata.expiresAt}`);
}
```

Look up flag owners:

```typescript
const owners = client.flagOwners('new-checkout');
// ["payments-team", "jane@example.com"]
```

List all cached flags:

```typescript
const flags = client.allFlags();
```

## Register / Dispatch Pattern

The register/dispatch pattern lets you centralize all flag-gated behavior in one place instead of scattering `if/else` checks throughout your codebase. Register named operations with their handlers and optional flag keys, then dispatch by operation name at the call site.

### Basic usage

```typescript
// --- registrations.ts (single-point registration) ---

import { client } from './deploysentry';

// Default handler (no flag key) — always used as fallback
client.register('checkout', () => {
  return processLegacyCheckout();
});

// Flag-gated handler — used when 'new-checkout-flow' is enabled
client.register('checkout', () => {
  return processNewCheckout();
}, 'new-checkout-flow');


// --- somewhere-else.ts (call site) ---

// Dispatch selects the right handler based on flag state.
// No flag logic here — the call site doesn't know or care.
const checkout = client.dispatch<() => Promise<Order>>('checkout');
await checkout();
```

### How it works

1. **`register(operation, handler, flagKey?)`** — Registers a handler for a named operation. If `flagKey` is provided, the handler is only selected when that flag is enabled. Omit `flagKey` to register the default/fallback handler.

2. **`dispatch(operation)`** — Returns the first registered handler whose flag is enabled. If no flagged handler matches, returns the default handler. Throws if no handlers are registered.

### Single-point registration

Keep all registrations in a single file (e.g. `src/flags/registrations.ts`) that runs at startup. This gives you:

- One place to see every flag-gated behavior in your app
- Easy auditing of which flags control which operations
- Simple cleanup when a flag is retired (remove the registration, keep the handler)

```typescript
// src/flags/registrations.ts
import { client } from './deploysentry';
import { legacySearch, vectorSearch } from '../search';
import { standardPricing, tieredPricing } from '../pricing';

// Search
client.register('search', legacySearch);
client.register('search', vectorSearch, 'vector-search-v2');

// Pricing
client.register('calculate-price', standardPricing);
client.register('calculate-price', tieredPricing, 'tiered-pricing-experiment');
```

## Offline Mode

For testing or environments without network access, enable offline mode. The client will return default values without contacting the API:

```typescript
const client = new DeploySentryClient({
  apiKey: 'not-used',
  environment: 'test',
  project: 'my-app',
  application: 'my-web-app',
  offlineMode: true,
});

await client.initialize(); // no-op in offline mode

const value = await client.boolValue('any-flag', true);
// Returns the default value: true
```

## Configuration

| Option         | Type     | Required | Default                        | Description                            |
| -------------- | -------- | -------- | ------------------------------ | -------------------------------------- |
| `apiKey`       | string   | Yes      | -                              | API key for authentication             |
| `environment`  | string   | Yes      | -                              | Target environment                     |
| `project`      | string   | Yes      | -                              | Project identifier                     |
| `application`  | string   | Yes      | -                              | Application identifier                 |
| `baseURL`      | string   | No       | `https://api.dr-sentry.com`  | API base URL                           |
| `cacheTimeout` | number   | No       | `60000`                        | Cache TTL in milliseconds              |
| `offlineMode`  | boolean  | No       | `false`                        | Return defaults without API calls      |
| `mode`         | string   | No       | `server`                       | `server`, `file`, or `server-with-fallback` |
| `flagFilePath` | string   | No       | `.deploysentry/flags.yaml`     | Path to YAML flag config file          |

## Offline / File Mode

The SDK can load flag configurations from a local YAML file instead of (or as fallback to) the server.

### Modes

| Mode | Behavior |
| --- | --- |
| `server` (default) | API calls + SSE streaming |
| `file` | Load from YAML file, evaluate locally. No server contact. |
| `server-with-fallback` | Try server first. If unavailable, fall back to YAML file. |

### Usage

```typescript
// File mode — local development, CI, testing
const client = new DeploySentryClient({
  apiKey: 'not-used',
  environment: 'staging',
  project: 'my-project',
  application: 'my-web-app',
  mode: 'file',
  flagFilePath: '.deploysentry/flags.yaml', // default
});
await client.initialize();

// Fallback mode — production resilience
const client = new DeploySentryClient({
  apiKey: 'ds_live_xxx',
  environment: 'production',
  project: 'my-project',
  application: 'my-web-app',
  mode: 'server-with-fallback',
});
await client.initialize();
```

### Generating the YAML file

Export from the DeploySentry dashboard: App Settings → Export flags.yaml. Place the downloaded file at `.deploysentry/flags.yaml` in your project root.

### Local rule evaluation

In file mode, targeting rules are evaluated locally. The SDK matches context attributes against rule conditions using the same operators as the server: equals, not_equals, in, not_in, contains, starts_with, ends_with, greater_than, less_than.

## API Endpoints

The SDK communicates with these DeploySentry API endpoints:

- `POST /api/v1/flags/evaluate` - Evaluate a single flag
- `POST /api/v1/flags/batch-evaluate` - Evaluate multiple flags
- `GET /api/v1/flags/stream?project_id=X&environment_id=Y` - SSE stream for real-time updates
- `GET /api/v1/flags?project_id=X` - List all flags for a project

Authentication uses the `Authorization: ApiKey <key>` header.

### Session Consistency

Bind evaluations to a session so the server caches results for a consistent user experience:

```typescript
const client = new DeploySentryClient({
  apiKey: 'ds_key_xxxxxxxxxxxx',
  environment: 'production',
  project: 'my-project',
  application: 'my-web-app',
  sessionId: `user:${userId}`,
});
await client.refreshSession();
```

- The session ID is sent as an `X-DeploySentry-Session` header on every request.
- The server caches evaluation results per session for 30 minutes (sliding TTL).
- Omit the session ID to always get fresh evaluations on each request.

## License

Apache-2.0
