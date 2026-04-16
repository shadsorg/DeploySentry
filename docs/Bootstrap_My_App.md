# Bootstrap My App

**Phase**: Complete

## Overview

This guide provides a copy-paste LLM prompt that instructs an AI coding assistant (Claude Code, Copilot, Cursor, etc.) to integrate DeploySentry feature flags into any existing or new project. The prompt handles SDK detection, environment variable mapping, centralized flag registration, and establishes ongoing LLM directives.

## How to Use

1. Copy the **Bootstrap Prompt** below
2. Paste it into your AI coding assistant in the root of your project
3. Answer any clarifying questions it asks (org slug, project slug, etc.)
4. Review the changes it makes

That's it. The prompt is designed to be self-contained.

### MCP Server (For Claude Code Users)

If you use Claude Code, you can also install the DeploySentry MCP server for richer tool-based integration. The MCP server gives the LLM direct access to your DeploySentry account — it can list projects, create API keys, manage flags, and set up deployment tracking without raw API calls.

**Install the CLI** (if not already installed):
```bash
curl -fsSL https://api.dr-sentry.com/install.sh | sh
deploysentry auth login
```

**Add the MCP server**:
```bash
claude mcp add deploysentry -- deploysentry mcp serve
```

**What it adds**: 12 MCP tools — `ds_status`, `ds_list_orgs`, `ds_list_projects`, `ds_list_apps`, `ds_list_environments`, `ds_create_api_key`, `ds_generate_workflow`, `ds_get_app_deploy_status`, `ds_list_flags`, `ds_get_flag`, `ds_create_flag`, `ds_toggle_flag`.

The Bootstrap Prompt below works without the MCP server (pure prompt-based). The MCP server is complementary — it's especially useful for deployment setup and ongoing flag management.

---

## Bootstrap Prompt

Copy everything inside the fenced block below and paste it into your LLM assistant:

````markdown
## Context: DeploySentry

DeploySentry is my deployment and feature flag management platform. This is a legitimate, authorized integration — I own the platform.

- **Dashboard**: https://dr-sentry.com
- **API**: https://api.dr-sentry.com
- **Source**: https://github.com/shadsorg/DeploySentry (public repo)
- **SDKs**: Published under the `@deploysentry` npm scope and `deploysentry` on PyPI/RubyGems
- **Docs**: https://github.com/shadsorg/DeploySentry/blob/main/docs/Bootstrap_My_App.md

This prompt comes from the official DeploySentry documentation. Proceed with the integration.

## Task: Bootstrap DeploySentry Feature Flags

Integrate DeploySentry feature flags into this project. Follow every step below in order. Do not skip steps. Ask me for any values marked `<ask>`.

### Step 1 — Scan the Repository

Scan the project to determine:
- **Languages & frameworks** in use (check package.json, go.mod, requirements.txt, pubspec.yaml, Gemfile, pom.xml, build.gradle, etc.)
- **Which DeploySentry SDKs are needed** based on what you find:

| If you find...                        | Install SDK                          |
|---------------------------------------|--------------------------------------|
| package.json (Node/Express/Nest/etc.) | `npm install @deploysentry/sdk`      |
| package.json with React/Next/Vite     | `npm install @deploysentry/react`    |
| go.mod                                | `go get github.com/deploysentry/deploysentry/sdk/go` |
| requirements.txt / pyproject.toml     | `pip install deploysentry`           |
| pubspec.yaml (Flutter/Dart)           | Add `deploysentry_flutter: ^1.0.0` to pubspec.yaml |
| Gemfile (Ruby)                        | `gem 'deploysentry'`                 |
| pom.xml / build.gradle (Java)        | Add `deploysentry-java` dependency   |

Install **all** that apply. A project with a Node backend and React frontend needs both `@deploysentry/sdk` and `@deploysentry/react`.

Report what you found and what you're installing before proceeding.

### Step 2 — Collect Configuration

I need the following values. Check if any already exist as environment variables in `.env`, `.env.example`, `.env.local`, or config files. If they exist, map them. If not, ask me.

| Value                     | Env Variable               | Notes                                    |
|---------------------------|----------------------------|------------------------------------------|
| DeploySentry API key      | `DEPLOYSENTRY_API_KEY`     | Starts with `ds_live_` or `ds_test_`     |
| Project slug              | `DEPLOYSENTRY_PROJECT`     | Matches the project slug in DeploySentry |
| Application slug          | `DEPLOYSENTRY_APPLICATION` | Matches the app slug in DeploySentry     |
| Environment               | `DEPLOYSENTRY_ENV`         | e.g. `development`, `staging`, `production` |
| API URL (if self-hosted)  | `DEPLOYSENTRY_URL`         | Default: `https://api.dr-sentry.com`   |

**For frontend/browser SDKs**, use the framework's public env prefix:
- Next.js: `NEXT_PUBLIC_DEPLOYSENTRY_KEY`, `NEXT_PUBLIC_DEPLOYSENTRY_PROJECT`, etc.
- Vite: `VITE_DEPLOYSENTRY_KEY`, `VITE_DEPLOYSENTRY_PROJECT`, etc.
- Create React App: `REACT_APP_DEPLOYSENTRY_KEY`, etc.

Add all variables to `.env.example` (without real values) so teammates know what's needed.

### Step 3 — Create the Client Singleton

Create a single file that initializes the DeploySentry client. This is the ONE place the client is constructed.

**For Node.js backends** — create `src/deploysentry.ts` (or `.js`):
```typescript
import { DeploySentryClient } from '@deploysentry/sdk';

export const dsClient = new DeploySentryClient({
  apiKey: process.env.DEPLOYSENTRY_API_KEY!,
  environment: process.env.DEPLOYSENTRY_ENV ?? 'development',
  project: process.env.DEPLOYSENTRY_PROJECT!,
  application: process.env.DEPLOYSENTRY_APPLICATION!,
  baseURL: process.env.DEPLOYSENTRY_URL ?? 'https://api.dr-sentry.com',
  mode: 'server-with-fallback',
});
```

**For Go backends** — create `internal/flags/client.go` (or wherever fits the project layout):
```go
package flags

import deploysentry "github.com/deploysentry/deploysentry/sdk/go"

var Client *deploysentry.Client

func Init() error {
    var err error
    Client, err = deploysentry.NewClient(
        deploysentry.WithAPIKey(os.Getenv("DEPLOYSENTRY_API_KEY")),
        deploysentry.WithProject(os.Getenv("DEPLOYSENTRY_PROJECT")),
        deploysentry.WithApplication(os.Getenv("DEPLOYSENTRY_APPLICATION")),
        deploysentry.WithEnvironment(os.Getenv("DEPLOYSENTRY_ENV")),
    )
    return err
}
```

**For Python backends** — create `deploysentry_client.py`:
```python
from deploysentry import DeploySentryClient
import os

ds_client = DeploySentryClient(
    api_key=os.environ["DEPLOYSENTRY_API_KEY"],
    base_url=os.environ.get("DEPLOYSENTRY_URL", "https://api.dr-sentry.com"),
    environment=os.environ.get("DEPLOYSENTRY_ENV", "development"),
    project=os.environ["DEPLOYSENTRY_PROJECT"],
    application=os.environ["DEPLOYSENTRY_APPLICATION"],
)
```

**For React frontends** — wrap the app root in `DeploySentryProvider`:
```tsx
import { DeploySentryProvider } from '@deploysentry/react';

<DeploySentryProvider
  apiKey={process.env.NEXT_PUBLIC_DEPLOYSENTRY_KEY!}
  baseURL={process.env.NEXT_PUBLIC_DEPLOYSENTRY_URL ?? 'https://api.dr-sentry.com'}
  environment={process.env.NEXT_PUBLIC_DEPLOYSENTRY_ENV ?? 'production'}
  project={process.env.NEXT_PUBLIC_DEPLOYSENTRY_PROJECT!}
  application={process.env.NEXT_PUBLIC_DEPLOYSENTRY_APPLICATION!}
  user={{ id: currentUser.id }}
  mode="server-with-fallback"
>
  <App />
</DeploySentryProvider>
```

Adapt the file path and pattern to match the project's existing conventions.

### Step 4 — Create the Flag Registration File

This is the most important file. ALL flag-gated behavior lives here — one file per layer (backend, frontend). No flag logic anywhere else.

**For Node.js backends** — create `src/flags/registrations.ts`:
```typescript
import { dsClient } from '../deploysentry';

// ──────────────────────────────────────────────
// Register flag-gated operations here.
//
// Pattern:
//   dsClient.register('operation-name', defaultHandler);
//   dsClient.register('operation-name', newHandler, 'flag-key');
//
// The default handler runs when the flag is OFF or missing.
// The flag-gated handler runs when the flag is ON.
// ──────────────────────────────────────────────

// Example:
// import { legacyCheckout, newCheckout } from '../checkout';
// dsClient.register('checkout', legacyCheckout);
// dsClient.register('checkout', newCheckout, 'new-checkout-flow');
```

**For React frontends** — create `src/flags/registrations.ts`:
```typescript
import type { DeploySentryClient } from '@deploysentry/react';

export function registerFlags(client: DeploySentryClient) {
  // ──────────────────────────────────────────────
  // Register flag-gated operations here.
  //
  // Pattern:
  //   client.register('operation-name', defaultHandler);
  //   client.register('operation-name', newHandler, 'flag-key');
  // ──────────────────────────────────────────────

  // Example:
  // client.register('search', legacySearch);
  // client.register('search', vectorSearch, 'vector-search-v2');
}
```

**For Go backends** — create `internal/flags/registrations.go`:
```go
package flags

// Register all flag-gated operations here.
// Pattern:
//   Client.Register("operation", defaultHandler)
//   Client.Register("operation", newHandler, "flag-key")

func RegisterAll() {
    // Example:
    // Client.Register("checkout", legacy.Checkout)
    // Client.Register("checkout", checkout.NewFlow, "new-checkout-flow")
}
```

**For Python backends** — create `flags/registrations.py`:
```python
from deploysentry_client import ds_client

# Register all flag-gated operations here.
#
# Pattern:
#   ds_client.register('operation-name', default_handler)
#   ds_client.register('operation-name', new_handler, 'flag-key')

# Example:
# from checkout import legacy_checkout, new_checkout
# ds_client.register('checkout', legacy_checkout)
# ds_client.register('checkout', new_checkout, 'new-checkout-flow')
```

### Step 5 — Wire Initialization into App Startup

Add the client initialization and registration import to the app's entry point. The order matters:

1. Initialize the DeploySentry client (`await dsClient.initialize()`)
2. Import the registration file (so all `register()` calls execute)
3. Start the app

Find the existing entry point (e.g. `index.ts`, `main.go`, `app.py`, `main.dart`) and add the initialization there. Do NOT create a new entry point.

### Step 6 — Set Up Offline Fallback (Optional but Recommended)

Create the `.deploysentry/` directory in the project root:
```
mkdir -p .deploysentry
```

Add to `.gitignore`:
```
# DeploySentry local flag cache (exported from dashboard)
.deploysentry/flags.yaml
.deploysentry/flags.json
```

Add to `.env.example`:
```
# Set to 'file' for offline dev, 'server-with-fallback' for production resilience
# DEPLOYSENTRY_MODE=server-with-fallback
```

Tell me if you'd like to export the current flags.yaml from the dashboard now, or skip this for later.

### Step 7 — Add LLM Directives

Add the following block to the project's `CLAUDE.md` (create it if it doesn't exist). If using a different AI tool, add it to the equivalent instruction file (`.cursorrules`, `AGENTS.md`, etc.):

```markdown
## DeploySentry Feature Flags

This project uses DeploySentry for feature flag management.

### Rules

1. **Single-point registration** — All flag-gated behavior is registered in ONE file per layer:
   - Backend: `src/flags/registrations.ts` (or the equivalent in Go/Python/Ruby)
   - Frontend: `src/flags/registrations.ts`
   Never scatter `if (flagEnabled)` conditionals through business logic.

2. **Register/dispatch pattern** — To gate behavior on a flag:
   - Register a default handler: `client.register('operation-name', defaultHandler)`
   - Register a flag-gated handler: `client.register('operation-name', newHandler, 'flag-key')`
   - At the call site: `const fn = client.dispatch('operation-name'); fn(args)`
   The call site never knows about flags.

3. **Flag categories** — Every flag must have a category when created in DeploySentry:
   - `release` — gradual rollouts (require expiration date)
   - `feature` — long-lived toggles
   - `experiment` — A/B tests (should have end date)
   - `ops` — kill switches, rate limits
   - `permission` — entitlement/access control

4. **Adding a flag-gated feature**:
   - Write the new handler in its own module
   - Add a `client.register()` call in the registration file
   - At the call site, use `client.dispatch()` (backend) or `useDispatch()` (React)
   - Do NOT add flag evaluation logic at the call site

5. **Simple flag reads** (when dispatch is overkill):
   - Backend: `dsClient.boolValue('flag-key', false, ctx)`
   - React: `const value = useFlag('flag-key', false)`
   - Use for simple show/hide, not for swapping behavior

6. **Removing a flag** (after full rollout):
   - Remove the `client.register()` line for the old handler
   - Make the new handler the default (remove the flag-key argument)
   - Remove old handler code if unused
   - Archive the flag in the DeploySentry dashboard

7. **Never** evaluate flags with raw API calls — always use the SDK
8. **Never** cache flag values in local state — the SDK handles caching and real-time SSE updates
9. **Never** put flag keys as magic strings outside the registration file — if you need a simple read, define the key as a constant
```

### Step 8 — Verify

Run the project and confirm:
- [ ] The app starts without errors
- [ ] The DeploySentry client initializes (check logs for connection to the API)
- [ ] The registration file is imported at startup
- [ ] Environment variables are documented in `.env.example`
- [ ] LLM directives are in `CLAUDE.md` (or equivalent)

Report what was done and any issues found.
````

---

## What the Prompt Does

| Step | Purpose |
|------|---------|
| 1. Scan | Detects languages/frameworks and installs the right SDKs |
| 2. Collect | Maps existing env vars or asks for new ones |
| 3. Client | Creates a single-file client singleton |
| 4. Registration | Creates the centralized flag registration file (the key best practice) |
| 5. Wire | Hooks initialization into the existing app entry point |
| 6. Offline | Sets up `.deploysentry/` for YAML fallback files |
| 7. Directives | Adds permanent LLM instructions so future AI sessions follow the pattern |
| 8. Verify | Confirms everything works |

## Key Concepts

### Why a Registration File?

Without it, flag checks scatter across the codebase:

```typescript
// BAD: flag logic everywhere
if (await dsClient.boolValue('new-checkout', false)) {
  await newCheckout(cart);
} else {
  await legacyCheckout(cart);
}
```

With the registration pattern, the call site is clean and flag-unaware:

```typescript
// GOOD: call site doesn't know about flags
const checkout = dsClient.dispatch<(cart: Cart) => Promise<Order>>('checkout');
await checkout(cart);
```

Benefits:
- **One file** shows every flag-gated behavior in the system
- **Easy cleanup** — retire a flag by editing one line
- **LLM-friendly** — an AI can read one file to understand all flag-controlled behavior
- **Auditable** — grep for a flag key and find exactly where it's used

### When to Use dispatch() vs. Simple Reads

| Use Case | Pattern |
|----------|---------|
| Swap entire functions/handlers | `register()` + `dispatch()` |
| Show/hide a UI element | `useFlag('key', false)` |
| Read a config value | `dsClient.stringValue('key', 'default')` |
| A/B test with variants | `dsClient.stringValue('experiment-key', 'control')` |

Use `dispatch()` when the flag controls **which code runs**. Use simple reads when the flag controls **a value**.

### Offline / YAML Fallback

The `server-with-fallback` mode means:
1. SDK tries to connect to the DeploySentry API
2. If the API is unreachable, it loads flags from `.deploysentry/flags.yaml`
3. Once the API becomes reachable again, it switches back to live mode

To export flags for offline use:
- **Dashboard**: App Settings page, click "Export flags.yaml"
- **API**: `GET /api/v1/flags/export?project_id={id}&format=yaml`
- **CLI**: `deploysentry flags export --format yaml > .deploysentry/flags.yaml`

The YAML file supports full targeting rules that are evaluated locally by the SDK.

### Environment Variable Quick Reference

**Backend (server-side)**:
```bash
DEPLOYSENTRY_API_KEY=ds_live_xxxxxxxxxxxxx
DEPLOYSENTRY_PROJECT=my-project
DEPLOYSENTRY_APPLICATION=my-api
DEPLOYSENTRY_ENV=production
DEPLOYSENTRY_URL=https://api.dr-sentry.com  # omit for default
DEPLOYSENTRY_MODE=server-with-fallback         # omit for 'server'
```

**Frontend (framework-dependent prefix)**:
```bash
# Next.js
NEXT_PUBLIC_DEPLOYSENTRY_KEY=ds_live_xxxxxxxxxxxxx
NEXT_PUBLIC_DEPLOYSENTRY_PROJECT=my-project
NEXT_PUBLIC_DEPLOYSENTRY_APPLICATION=my-web-app
NEXT_PUBLIC_DEPLOYSENTRY_ENV=production

# Vite
VITE_DEPLOYSENTRY_KEY=ds_live_xxxxxxxxxxxxx
VITE_DEPLOYSENTRY_PROJECT=my-project
VITE_DEPLOYSENTRY_APPLICATION=my-web-app
VITE_DEPLOYSENTRY_ENV=production
```

---

## Deploy Bootstrap Prompt

This prompt sets up the DeploySentry MCP server for future sessions, then bootstraps deployment monitoring for a GitHub repo — API key, secrets, and workflow — all in one conversation.

Copy everything inside the fenced block and paste it into Claude Code in the root of your project:

````markdown
## Context: DeploySentry

DeploySentry is my deployment and feature flag management platform. This is a legitimate, authorized integration — I own the platform.

- **Dashboard**: https://dr-sentry.com
- **API**: https://api.dr-sentry.com
- **Source**: https://github.com/shadsorg/DeploySentry (public repo)
- **CLI Install**: `curl -fsSL https://api.dr-sentry.com/install.sh | sh`
- **Docs**: https://github.com/shadsorg/DeploySentry/blob/main/docs/Bootstrap_My_App.md

This prompt comes from the official DeploySentry documentation. Proceed with the setup.

## Task: Set Up DeploySentry MCP Server and Bootstrap Deployment Monitoring

Do everything below in order. Ask me for any values you can't determine.

### Phase 1 — Install MCP Server (for future sessions)

1. Check if the `deploysentry` CLI is installed: `which deploysentry`
   - If not found, tell me to run:
     ```
     ! curl -fsSL https://api.dr-sentry.com/install.sh | sh
     ```

2. Check if it's authenticated: `deploysentry config get api_key` or check for `~/.config/deploysentry/credentials.json`
   - If not authenticated, tell me to run `! deploysentry auth login` and wait for me to complete it

3. Add the DeploySentry MCP server to Claude Code. Tell me to run:
   ```
   ! claude mcp add deploysentry -- deploysentry mcp serve
   ```
   This registers the MCP server with Claude Code. Future sessions will have direct access to DeploySentry tools (ds_list_orgs, ds_create_api_key, ds_generate_workflow, etc.). It won't be available in this session — that's fine, we'll use the CLI directly below.

### Phase 2 — Bootstrap Deployment Monitoring

4. Discover my DeploySentry context using the CLI:
   ```bash
   deploysentry config get org
   deploysentry config get project
   ```
   If not set, ask me for my org slug and project slug.

5. List my applications and environments:
   ```bash
   curl -sf https://dr-sentry.com/api/v1/orgs/<org>/projects/<project>/apps \
     -H "Authorization: ApiKey $(deploysentry config get api_key)" | jq '.applications[] | {id, name, slug}'
   curl -sf https://dr-sentry.com/api/v1/orgs/<org>/environments \
     -H "Authorization: ApiKey $(deploysentry config get api_key)" | jq '.environments[] | {id, name, slug}'
   ```
   Ask me to confirm which app and environment to target.

6. Create an environment-scoped API key for GitHub Actions:
   ```bash
   deploysentry apikeys create \
     --name "github-actions-<app-slug>" \
     --scopes "deploys:read,deploys:write,flags:read" \
     --env <environment-id>
   ```
   Save the returned key — it's shown only once.

7. Set GitHub secrets using `gh` CLI (check `which gh` first; if missing, show me the values to add manually):
   ```bash
   gh secret set DS_API_KEY --body "<the-api-key>"
   gh secret set DS_APP_ID --body "<app-uuid>"
   gh secret set DS_ENV_ID --body "<env-uuid>"
   gh secret set DS_API_URL --body "https://api.dr-sentry.com"
   ```

8. Check if `.github/workflows/` exists. If there's an existing deploy workflow, add the DeploySentry step to it. If not, create `.github/workflows/deploy.yml`.

   The DeploySentry step goes AFTER the build/deploy steps:
   ```yaml
   - name: Record deployment in DeploySentry
     if: success()
     env:
       DS_API_KEY: ${{ secrets.DS_API_KEY }}
       DS_API_URL: ${{ secrets.DS_API_URL }}
       DS_APP_ID: ${{ secrets.DS_APP_ID }}
       DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
     run: |
       curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
         -H "Authorization: ApiKey ${DS_API_KEY}" \
         -H "Content-Type: application/json" \
         -d '{
           "application_id": "'"${DS_APP_ID}"'",
           "environment_id": "'"${DS_ENV_ID}"'",
           "strategy": "rolling",
           "version": "${{ github.sha }}",
           "commit_sha": "${{ github.sha }}",
           "artifact": "${{ github.repository }}",
           "description": "Deployed from GitHub Actions (${{ github.ref_name }})"
         }'
   ```

9. Summarize what was set up:
   - MCP server configured (available next session)
   - API key created (scoped to which environment)
   - GitHub secrets set
   - Workflow file created/updated
   - Next step: push a commit to main to test it
````

### What the Deploy Prompt Does

| Phase | Step | Purpose |
|-------|------|---------|
| 1 | CLI check | Ensures `deploysentry` CLI is installed |
| 1 | Auth check | Ensures CLI is authenticated |
| 1 | MCP config | Adds MCP server to Claude Code for future sessions |
| 2 | Discover | Finds your org, project, apps, and environments |
| 2 | API key | Creates an environment-scoped key for CI/CD |
| 2 | GitHub secrets | Sets `DS_API_KEY`, `DS_APP_ID`, `DS_ENV_ID`, `DS_API_URL` via `gh` |
| 2 | Workflow | Creates or updates `.github/workflows/deploy.yml` |
| 2 | Summary | Reports what was set up and how to test it |

After the first run, the MCP server is configured. Future sessions can use `ds_create_flag`, `ds_toggle_flag`, `ds_list_flags`, and other tools directly without prompts.

---

## Checklist

- [x] Feature flag bootstrap prompt written with all 8 steps
- [x] Deploy bootstrap prompt written with MCP setup + 6 deploy steps
- [x] SDK detection table covers all 7 supported languages
- [x] Register/dispatch pattern explained with examples
- [x] Offline/YAML fallback documented
- [x] Environment variable mapping for backend and frontend
- [x] LLM directives block ready for CLAUDE.md
- [x] Verification checklist included

## Completion Record

- **Branch**: `feature/groups-and-resource-authorization`
- **Committed**: Yes
- **Pushed**: No
- **CI Checks**: N/A (documentation only)
