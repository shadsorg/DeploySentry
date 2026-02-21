# DeploySentry Node.js SDK

Official Node.js SDK for integrating with the DeploySentry platform.

## Installation

```bash
npm install @deploysentry/sdk
```

## Quick Start

```typescript
import { DeploySentry } from '@deploysentry/sdk';

const client = new DeploySentry({ apiKey: 'your-api-key' });

// Evaluate a feature flag
const enabled = await client.flags.isEnabled('my-feature', {
  userId: 'user-123',
});

if (enabled) {
  // New feature code path
}
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/node](https://docs.deploysentry.io/sdk/node).

## License

Apache-2.0
