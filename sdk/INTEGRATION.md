# DeploySentry Integration Guide

This guide covers how to integrate DeploySentry feature flags into a project, including LLM-assisted development directives for teams using AI coding tools (Claude Code, Copilot, etc.).

## Architecture Overview

DeploySentry provides:
- **Feature flags** with rich metadata (categories, owners, expiration, tags)
- **Real-time updates** via SSE — flag changes propagate instantly
- **Register/dispatch pattern** — centralize flag-gated behavior instead of scattering conditionals
- **Flag categories** that enforce lifecycle policies (release, feature, experiment, ops, permission)

## SDK Packages

| Package              | Use Case                          |
| -------------------- | --------------------------------- |
| `@deploysentry/sdk`  | Node.js / server-side backends    |
| `@deploysentry/react`| React frontends                   |

## Integration Checklist

### Backend (Node.js / Nest.js / Express)

1. Install the SDK:
   ```bash
   npm install @deploysentry/sdk
   ```

2. Create a singleton client (e.g. `src/deploysentry.ts`):
   ```typescript
   import { DeploySentryClient } from '@deploysentry/sdk';

   export const dsClient = new DeploySentryClient({
     apiKey: process.env.DEPLOYSENTRY_API_KEY!,
     environment: process.env.DEPLOYSENTRY_ENV ?? 'development',
     project: process.env.DEPLOYSENTRY_PROJECT!,
     application: process.env.DEPLOYSENTRY_APPLICATION!,
     baseURL: process.env.DEPLOYSENTRY_URL ?? 'https://api.deploysentry.io',
   });
   ```

3. Initialize at startup:
   ```typescript
   await dsClient.initialize();
   ```

4. Create a single-point registration file (e.g. `src/flags/registrations.ts`):
   ```typescript
   import { dsClient } from '../deploysentry';
   import { legacyCheckout, newCheckout } from '../checkout';

   // Default handler (fallback)
   dsClient.register('checkout', legacyCheckout);
   // Flag-gated handler
   dsClient.register('checkout', newCheckout, 'new-checkout-flow');
   ```

5. Import the registration file at startup (after `initialize()`).

6. At call sites, dispatch by operation name:
   ```typescript
   const checkout = dsClient.dispatch<(cart: Cart) => Promise<Order>>('checkout');
   await checkout(cart);
   ```

### Frontend (React)

1. Install the SDK:
   ```bash
   npm install @deploysentry/react
   ```

2. Wrap the app in the provider:
   ```tsx
   import { DeploySentryProvider } from '@deploysentry/react';

   <DeploySentryProvider
     apiKey={process.env.NEXT_PUBLIC_DEPLOYSENTRY_KEY!}
     baseURL="https://api.deploysentry.io"
     environment="production"
     project="my-app"
     application="my-web-app"
     user={{ id: currentUser.id }}
   >
     <App />
   </DeploySentryProvider>
   ```

3. Create a single-point registration file (e.g. `src/flags/registrations.ts`):
   ```typescript
   import type { DeploySentryClient } from '@deploysentry/react';

   export function registerFlags(client: DeploySentryClient) {
     client.register('search', legacySearch);
     client.register('search', vectorSearch, 'vector-search-v2');
   }
   ```

4. Use hooks in components:
   ```tsx
   // Simple boolean check
   const showBanner = useFlag('promo-banner', false);

   // Full metadata
   const { value, enabled, metadata } = useFlagDetail('new-checkout');

   // Dispatch registered operation
   const search = useDispatch<(q: string) => Promise<Results>>('search');
   ```

## Single-Point Registration Principle

All flag-to-behavior mappings should be defined in ONE place per layer:

- **Backend**: A single registration module that calls `client.register()` for every flag-gated operation
- **Frontend**: A single registration file that sets up all dispatch registrations at app init

**Why this matters:**
- One place to see every flag-gated behavior
- Easy auditing — `grep` for the flag key and you find every behavior it controls
- Simple cleanup — when a flag is retired, remove the registration line, keep the handler
- LLMs can read one file to understand all flag-gated behavior instead of scanning the entire codebase

## Flag Categories

Every flag must have a category. These drive lifecycle enforcement:

| Category     | Purpose                                           | Lifecycle Rule                         |
| ------------ | ------------------------------------------------- | -------------------------------------- |
| `release`    | Gradual rollout gates for new releases            | Requires `expiresAt` or `isPermanent`  |
| `feature`    | Long-lived feature toggles                        | No expiration required                 |
| `experiment` | A/B tests and experiments                         | Should have defined end date           |
| `ops`        | Kill switches, rate limits, circuit breakers      | Typically permanent                    |
| `permission` | Entitlement and access-control flags              | Tied to billing/plan lifecycle         |

## Environment Variables

| Variable                    | Required | Description                          |
| --------------------------- | -------- | ------------------------------------ |
| `DEPLOYSENTRY_API_KEY`      | Yes      | Server-side API key                  |
| `DEPLOYSENTRY_PROJECT`      | Yes      | Project identifier                   |
| `DEPLOYSENTRY_APPLICATION`  | Yes      | Application identifier                 |
| `DEPLOYSENTRY_ENV`          | No       | Environment (default: `development`) |
| `DEPLOYSENTRY_URL`          | No       | API base URL (default: `https://api.deploysentry.io`) |
| `DEPLOYSENTRY_MODE`         | No       | SDK mode: `server`, `file`, or `server-with-fallback` |

For React/browser apps, prefix with your framework's public env convention (e.g. `NEXT_PUBLIC_`, `VITE_`).

## Offline / File Mode

Both SDKs support loading flags from a local config file for offline development, CI, or as a server fallback.

### Setup

1. Export the flag config from the dashboard: App Settings → Export flags.yaml
2. Place the file at `.deploysentry/flags.yaml` in your project root
3. Configure the SDK:

**Backend:**
```typescript
const dsClient = new DeploySentryClient({
  apiKey: process.env.DEPLOYSENTRY_API_KEY!,
  environment: process.env.DEPLOYSENTRY_ENV ?? 'development',
  project: process.env.DEPLOYSENTRY_PROJECT!,
  application: process.env.DEPLOYSENTRY_APPLICATION!,
  mode: 'server-with-fallback', // falls back to .deploysentry/flags.yaml
});
```

**Frontend (React):**
```tsx
import flagConfig from './.deploysentry/flags.json';

<DeploySentryProvider
  {...props}
  mode="server-with-fallback"
  flagData={flagConfig}
>
```

---

## LLM Directives for CLAUDE.md

Copy the block below into your project's `CLAUDE.md` (or equivalent AI instruction file) so that Claude Code and similar tools follow DeploySentry conventions automatically.

```markdown
## DeploySentry Feature Flags

This project uses DeploySentry for feature flag management.

### Key Principles

1. **Single-point registration** — All flag-gated behavior is registered in ONE file per layer:
   - Backend: `src/flags/registrations.ts`
   - Frontend: `src/flags/registrations.ts`
   Never add `if (flagEnabled)` conditionals in business logic. Use the register/dispatch pattern instead.

2. **Register/dispatch pattern** — To gate behavior on a flag:
   - Register a default handler: `client.register('operation-name', defaultHandler)`
   - Register a flag-gated handler: `client.register('operation-name', newHandler, 'flag-key')`
   - At the call site: `const fn = client.dispatch('operation-name'); fn(args)`
   The call site never knows about flags. The registration file is the single source of truth.

3. **Flag categories** — Every flag must have a category:
   - `release` — gradual rollouts (require expiration)
   - `feature` — long-lived toggles
   - `experiment` — A/B tests (require end date)
   - `ops` — kill switches, rate limits
   - `permission` — entitlement/access control

4. **Adding a new flag-gated feature**:
   - Write the new handler function in its own module
   - Add a `client.register()` call in the registration file
   - At the call site, use `client.dispatch()` (backend) or `useDispatch()` (React)
   - Do NOT add flag evaluation logic at the call site

5. **Removing a flag** (after full rollout):
   - Remove the `client.register()` line for the old handler from the registration file
   - Make the new handler the default: change `client.register('op', newHandler, 'flag-key')` to `client.register('op', newHandler)`
   - Remove the old handler code if no longer needed
   - Delete the flag in the DeploySentry dashboard

### SDK Usage

- **Backend**: Import `dsClient` from `src/deploysentry.ts`. Use `dsClient.boolValue()`, `dsClient.stringValue()`, etc. for simple reads. Use `dsClient.dispatch()` for behavior switching.
- **Frontend**: Use `useFlag()` for simple checks, `useFlagDetail()` for metadata, `useDispatch()` for behavior switching, `useFlagsByCategory()` for grouping.
- **Required SDK parameters**: `apiKey`, `environment`, `project`, and `application` are all required when constructing a client or provider.
- **Never** evaluate flags with raw API calls. Always use the SDK.
- **Never** cache flag values in local state. The SDK handles caching and real-time updates via SSE.
```
