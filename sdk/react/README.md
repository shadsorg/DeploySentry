# @deploysentry/react

Official React SDK for the DeploySentry feature flag platform. Provides React hooks and a context provider for evaluating feature flags with real-time SSE updates.

## Installation

```bash
npm install @deploysentry/react
```

React 18 or later is required as a peer dependency.

## Quick Start

Wrap your application with `DeploySentryProvider` and use hooks anywhere in the tree:

```tsx
import { DeploySentryProvider, useFlag } from '@deploysentry/react';

function App() {
  return (
    <DeploySentryProvider
      apiKey="ds_live_abc123"
      baseURL="https://api.deploysentry.io"
      environment="production"
      project="my-app"
      user={{ id: 'user-42' }}
    >
      <MyComponent />
    </DeploySentryProvider>
  );
}

function MyComponent() {
  const showBanner = useFlag('show-banner', false);

  if (!showBanner) return null;
  return <div>New feature banner!</div>;
}
```

## Provider Props

| Prop          | Type         | Required | Description                                      |
|---------------|--------------|----------|--------------------------------------------------|
| `apiKey`      | `string`     | Yes      | API key for authenticating with DeploySentry.     |
| `baseURL`     | `string`     | Yes      | Base URL of the DeploySentry API.                 |
| `environment` | `string`     | Yes      | Environment identifier (e.g. `"production"`).     |
| `project`     | `string`     | Yes      | Project identifier.                               |
| `user`        | `UserContext` | No      | User context for targeting rules.                 |
| `children`    | `ReactNode`  | Yes      | React children.                                   |

## Hooks

### `useFlag(key, defaultValue)`

Returns the resolved flag value. Falls back to `defaultValue` while loading or if the flag does not exist.

```tsx
const isEnabled = useFlag('dark-mode', false);
const variant = useFlag('checkout-flow', 'control');
```

### `useFlagDetail(key)`

Returns detailed evaluation information including metadata.

```tsx
const { value, enabled, metadata, loading } = useFlagDetail('new-checkout');

// metadata contains: category, purpose, owners, isPermanent, expiresAt, tags
```

### `useFlagsByCategory(category)`

Returns all flags matching the given category.

```tsx
const experiments = useFlagsByCategory('experiment');
const releases = useFlagsByCategory('release');
```

Categories: `'release'` | `'feature'` | `'experiment'` | `'ops'` | `'permission'`

### `useExpiredFlags()`

Returns all non-permanent flags whose `expiresAt` date is in the past. Useful for admin dashboards.

```tsx
const expired = useExpiredFlags();
```

### `useDispatch(operation)`

Returns the resolved handler for a registered operation based on current flag state. See [Register / Dispatch Pattern](#register--dispatch-pattern) below.

```tsx
const checkout = useDispatch<() => Promise<void>>('checkout');
await checkout();
```

### `useDeploySentry()`

Returns the raw `DeploySentryClient` instance for advanced use-cases.

```tsx
const client = useDeploySentry();
```

## Register / Dispatch Pattern

The register/dispatch pattern centralizes all flag-gated behavior in one place instead of scattering `if/else` checks across components.

### Setup (single-point registration)

Create one registration file that runs at app initialization:

```typescript
// src/flags/registrations.ts
import type { DeploySentryClient } from '@deploysentry/react';
import { legacySearch, vectorSearch } from '../search';
import { classicCheckout, newCheckout } from '../checkout';

export function registerFlags(client: DeploySentryClient) {
  // Search — default handler, then flag-gated override
  client.register('search', legacySearch);
  client.register('search', vectorSearch, 'vector-search-v2');

  // Checkout
  client.register('checkout', classicCheckout);
  client.register('checkout', newCheckout, 'new-checkout-flow');
}
```

Call `registerFlags(client)` after the client initializes (e.g. in the provider setup or an app-level effect).

### Usage in components

```tsx
import { useDispatch } from '@deploysentry/react';

function SearchBar() {
  const search = useDispatch<(query: string) => Promise<Results>>('search');

  return (
    <input onChange={(e) => search(e.target.value).then(setResults)} />
  );
}
```

### How it works

1. **`client.register(operation, handler, flagKey?)`** — Register a handler for a named operation. With `flagKey`, it's only selected when that flag is enabled. Without `flagKey`, it's the default fallback.
2. **`useDispatch(operation)`** — Returns the first registered handler whose flag is enabled, or the default handler. The component never knows about flags.

### Why single-point registration

- One file shows every flag-gated behavior in the app
- Easy auditing — search for a flag key and find every behavior it controls
- Simple cleanup when a flag is retired (remove the registration, keep the handler)
- LLMs and code review tools can read one file to understand all flag-gated behavior

## Real-Time Updates

The provider automatically opens an SSE connection to receive flag updates in real time. When a flag changes on the server, all components consuming that flag re-render immediately. The connection reconnects automatically with exponential backoff if it drops.

## SSR Compatibility

Hooks return default/empty values during server-side rendering. Flags are fetched client-side after the provider mounts.

## Types

All types are exported from the package:

```tsx
import type {
  Flag,
  FlagCategory,
  FlagDetail,
  FlagMetadata,
  ProviderProps,
  UserContext,
} from '@deploysentry/react';
```

## Authentication

All requests use an API key passed in the `Authorization` header:

```
Authorization: ApiKey <your-api-key>
```

Pass the key via the provider's `apiKey` prop. The SDK sets the header automatically.

### Session Consistency

Bind evaluations to a session so the server caches results for a consistent user experience:

```tsx
<DeploySentryProvider
  apiKey="ds_key_xxxxxxxxxxxx"
  baseURL="https://deploysentry.example.com"
  environment="production"
  project="my-app"
  sessionId={`user:${userId}`}
>
  <App />
</DeploySentryProvider>
```

```tsx
const client = useDeploySentry();
await client.refreshSession();
```

- The session ID is sent as an `X-DeploySentry-Session` header on every request.
- The server caches evaluation results per session for 30 minutes (sliding TTL).
- Omit the session ID to always get fresh evaluations on each request.

## License

Apache-2.0
