# DeploySentry Node.js SDK

Official Node.js/TypeScript SDK for integrating with the DeploySentry platform. Provides feature flag evaluation with rich metadata including categories, ownership, expiration tracking, and real-time updates via SSE.

## Installation

```bash
npm install @deploysentry/sdk
```

## Quick Start

```typescript
import { DeploySentryClient } from '@deploysentry/sdk';

const client = new DeploySentryClient({
  apiKey: 'ds_live_xxx',
  environment: 'production',
  project: 'my-app',
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

## Offline Mode

For testing or environments without network access, enable offline mode. The client will return default values without contacting the API:

```typescript
const client = new DeploySentryClient({
  apiKey: 'not-used',
  environment: 'test',
  project: 'my-app',
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
| `baseURL`      | string   | No       | `https://api.deploysentry.io`  | API base URL                           |
| `cacheTimeout` | number   | No       | `60000`                        | Cache TTL in milliseconds              |
| `offlineMode`  | boolean  | No       | `false`                        | Return defaults without API calls      |

## API Endpoints

The SDK communicates with these DeploySentry API endpoints:

- `POST /api/v1/flags/evaluate` - Evaluate a single flag
- `POST /api/v1/flags/batch-evaluate` - Evaluate multiple flags
- `GET /api/v1/flags/stream?project_id=X&environment_id=Y` - SSE stream for real-time updates
- `GET /api/v1/flags?project_id=X` - List all flags for a project

Authentication uses the `Authorization: ApiKey <key>` header.

## License

Apache-2.0
