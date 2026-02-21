# DeploySentry React SDK

Official React SDK for integrating with the DeploySentry platform.

## Installation

```bash
npm install @deploysentry/react
```

## Quick Start

```tsx
import { DeploySentryProvider, useFlag } from '@deploysentry/react';

function App() {
  return (
    <DeploySentryProvider apiKey="your-api-key">
      <MyComponent />
    </DeploySentryProvider>
  );
}

function MyComponent() {
  const { enabled, loading } = useFlag('my-feature');

  if (loading) return <div>Loading...</div>;
  if (!enabled) return <div>Feature not available</div>;

  return <div>New feature!</div>;
}
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/react](https://docs.deploysentry.io/sdk/react).

## License

Apache-2.0
