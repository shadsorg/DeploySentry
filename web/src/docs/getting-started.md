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

## Create your first flag

From the project's Flags page, click **New Flag**. Pick a category (release, feature, experiment, ops, or permission) and define a key. Or use the CLI:

```bash
deploysentry flags create my-flag --type boolean --default false --description "My first flag"
```

For detailed guidance on flag creation, targeting rules, permanent vs temporary flags, and the full lifecycle, see [Flag Management](/docs/flag-management).

## Wire up an SDK

Pick the SDK that matches your stack (see [SDKs](/docs/sdks)) and follow the register/dispatch pattern. The `CLAUDE.md` the setup script wrote has language-specific examples ready to go.
