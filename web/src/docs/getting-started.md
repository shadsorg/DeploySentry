# Getting Started

DeploySentry decouples deployment from release. This guide walks you from zero to your first feature flag in production.

## Choose your platform

### Option A: Use the hosted beta (easiest)

Sign up at [dr-sentry.com](https://dr-sentry.com) — no infrastructure to manage.

1. Create an account at [dr-sentry.com](https://dr-sentry.com)
2. Create your organization
3. Create a project and an application
4. Generate an API key from the **API Keys** page

You're ready to bootstrap your repo (see below).

### Option B: Self-host

Run DeploySentry on your own infrastructure with Docker Compose:

```bash
git clone https://github.com/shadsorg/DeploySentry.git
cd DeploySentry
make dev-up
make migrate-up
make run-api
```

The API listens on `:8080` and the dashboard on `:3001`.

1. Open http://localhost:3001
2. Sign up for an account
3. Create your organization, project, and application
4. Generate an API key from the **API Keys** page

## Bootstrap your repo

Once you have an API key, run the setup script from your project directory. It detects your language, installs the SDK, and writes an LLM integration prompt into `CLAUDE.md`:

```bash
curl -sSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/setup-sdk.sh | sh -s -- --api-key <your-api-key>
```

For self-hosted instances, pass your base URL:

```bash
curl -sSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/setup-sdk.sh | sh -s -- \
  --api-key <your-api-key> \
  --base-url http://localhost:8080
```

The script will:
- Detect your project language (Node, React, Go, Python, Java, Ruby, or Flutter)
- Install the appropriate SDK package
- Write a `CLAUDE.md` with the register/dispatch pattern and flag management instructions
- Print next steps

### What gets added to CLAUDE.md

The script writes a language-specific prompt that teaches LLMs how to use DeploySentry in your project. Here's what it looks like (Node.js example — other languages follow the same structure):

```markdown
## DeploySentry Feature Flags

### Connection

| Setting     | Value |
|-------------|-------|
| API Key     | `DS_API_KEY` env var |
| Environment | production |
| Project     | my-project |
| Application | my-web-app |
| Base URL    | https://dr-sentry.com |

### Initialization

  import { DeploySentryClient } from '@dr-sentry/sdk';
  const ds = new DeploySentryClient({
    apiKey: process.env.DS_API_KEY!,
    environment: 'production',
    project: 'my-project',
    application: 'my-web-app',
  });
  await ds.initialize();

### Register / Dispatch Pattern (REQUIRED)

NEVER evaluate a flag as a plain boolean to branch between old and new code:

  if (await ds.boolValue('my-flag', false, ctx)) { newFn(); } else { oldFn(); }

ALWAYS use register + dispatch so the flag engine selects the right implementation:

  ds.register('createCart', createCartWithMembership, 'membership-lookup');
  ds.register('createCart', createCart); // default — always last

  const result = ds.dispatch('createCart', { user_id: user.id })(cart, user);

**All register calls MUST live in a single file** (e.g. `flags.ts`, `flags.go`, `flags.py`).
This file is the single source of truth for every flag-gated operation in the application.
Never scatter register calls across modules. Dispatch calls go at each call site, but
registration is centralized so any developer or LLM can read one file to see every flag,
every operation, and every code path the flag controls.

### Flag Categories

| Category   | Lifecycle            | Notes                                     |
|------------|----------------------|-------------------------------------------|
| release    | Temporary            | Remove after rollout is complete           |
| feature    | Can be permanent     | Permanent if it controls a toggle-able UX  |
| experiment | Temporary            | Remove after experiment concludes          |
| ops        | Can be permanent     | Permanent for operational controls         |
| permission | Typically permanent  | Gates access by role/plan/attribute        |

### Creating Flags

  # Permanent flag
  deploysentry flags create --project my-project --key my-feature --category feature --is-permanent

  # Temporary flag
  deploysentry flags create --project my-project --key my-release --category release --expires-at 2026-12-31

### Context Requirements

Every dispatch call must supply context so targeting rules can be evaluated:
- user_id — unique user identifier (required)
- session_id — current session identifier (recommended)
- Custom attributes — any key/value pairs used in targeting rules (e.g. plan, country)

### Retiring a Flag (temporary flags only)

Permanent flags (is_permanent = true) are never retired.

For temporary flags, once the rollout or experiment is complete:
1. Remove all register calls for the flag key
2. Remove all dispatch call sites — replace with a direct call to the winning implementation
3. Delete or archive the losing implementation
4. Delete the flag via CLI
5. Remove any targeting rules or segments associated with the flag
```

The snippets are tailored to your detected language — Go, Python, Java, Ruby, React, and Flutter each get idiomatic examples.

## Create your first flag

From the project's Flags page, click **New Flag**. Pick a category (release, feature, experiment, ops, or permission) and define a key. Or use the CLI:

```bash
deploysentry flags create my-flag --type boolean --default false --description "My first flag"
```

For detailed guidance on flag creation, targeting rules, permanent vs temporary flags, and the full lifecycle, see [Flag Management](/docs/flag-management).

## Wire up an SDK

Pick the SDK that matches your stack (see [SDKs](/docs/sdks)) and follow the register/dispatch pattern. The `CLAUDE.md` the setup script wrote has language-specific examples ready to go.
