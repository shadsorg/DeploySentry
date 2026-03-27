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

### `useDeploySentry()`

Returns the raw `DeploySentryClient` instance for advanced use-cases.

```tsx
const client = useDeploySentry();
```

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

## License

Apache-2.0
