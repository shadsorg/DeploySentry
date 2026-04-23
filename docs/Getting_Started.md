# Getting Started with DeploySentry

A ten-minute path from "I have an account" to "my MCP-assisted agent can drive DeploySentry." If you're setting up a new machine for a teammate, start here.

## Prerequisites

- A DeploySentry account and at least one organization (create one via the dashboard if you haven't).
- Any modern shell (`bash` / `zsh` / PowerShell).
- An LLM tool with MCP support (Claude Code, Cursor, Continue, etc.) — optional but strongly recommended.

---

## 1. Install the CLI

The CLI is the foundation for everything else — including the MCP server, which is a CLI subcommand.

```bash
# macOS / Linux
curl -fsSL https://api.dr-sentry.com/install.sh | sh

# or from source (requires Go 1.22+)
go install github.com/deploysentry/deploysentry/cmd/cli@latest
mv "$(go env GOPATH)/bin/cli" "$(go env GOPATH)/bin/deploysentry"
```

Verify:
```bash
deploysentry --version
```

## 2. Authenticate the CLI

Create an API key in the dashboard and hand it to the CLI once per machine.

**Step A — create the key.** Sign in to the dashboard, open **Org → API Keys**, click **Create key**. Pick scopes that match what you'll do from this machine: `deploys:read`, `deploys:write`, `flags:read`, `flags:write`, `status:write`, `apikey:manage` are common for a developer laptop. Save the key somewhere safe — the dashboard only shows it once.

**Step B — hand it to the CLI.** Three options — **pick one with the flag or env var**; don't run `deploysentry auth login` bare unless you specifically want the interactive stdin prompt (see the troubleshooting note below).

```bash
# Non-interactive (recommended — works in every shell, including LLM agents)
deploysentry auth login --token ds_live_xxxxxxxxxxxx

# From environment (useful in CI, dev containers, and Claude Code sessions)
export DEPLOYSENTRY_API_KEY=ds_live_xxxxxxxxxxxx
deploysentry auth login

# Interactive — blocks waiting for paste on stdin; only use from a real terminal
deploysentry auth login
```

The key is validated against the server before it's saved, so a typo fails fast. On success the CLI writes `~/.config/deploysentry/credentials.json` (mode 0600) and every subsequent CLI command — plus the MCP server — reads from that file.

Verify:
```bash
deploysentry auth status
deploysentry orgs list           # should return your orgs
```

> **Troubleshooting — "`auth login` opened my browser to a 404 page".** You have a CLI binary from before 2026-04-23, when the flow was still OAuth. Two fixes:
>
> 1. **Rebuild the CLI** — `go install github.com/deploysentry/deploysentry/cmd/cli@main` (or re-run `curl -fsSL https://api.dr-sentry.com/install.sh | sh` once the latest release has been cut). Then run `deploysentry auth login --token ds_live_…`.
> 2. **Skip `auth login` on this binary.** Set `export DEPLOYSENTRY_API_KEY=ds_live_…` in your shell. Every CLI command (and the MCP server) falls back to the env var when no credentials file exists, so you can use the tool without persisting a key. Only the `auth login` / `auth status` commands will be broken — the rest work fine.
>
> DeploySentry does not currently offer a browser-based OAuth flow; API keys are the supported credential.

## 3. Install the MCP server (recommended)

The MCP server lets Claude Code / Cursor / any MCP client drive DeploySentry directly — create apps, mint keys, wire webhooks, check health, all through natural language.

```bash
# Add the DeploySentry MCP server to Claude Code
claude mcp add deploysentry -- deploysentry mcp serve
```

For other MCP clients, point them at the same `deploysentry mcp serve` process — it speaks standard MCP over stdio. The MCP server reads the same credentials file that Step 2 wrote; no separate auth flow.

Verify by asking your agent: **"Using the DeploySentry MCP, list my orgs."** It should call the `ds_list_orgs` tool and return what you saw in Step 2.

From here, most setup work is one conversation with your agent:

> "Set up DeploySentry to track deploys for my Railway service `acme-api`. Use the production env. Add a monitoring link to our Datadog APM page."

The agent will call `ds_reporting_setup_deploy_integration`, `ds_reporting_check_deploy_integration`, and `ds_reporting_verify` end-to-end.

---

## 4. (Optional) Bring up a local stack

If you're a contributor or testing against a local copy, follow [DEVELOPMENT.md](./DEVELOPMENT.md) for Postgres/Redis/NATS + API + web dashboard. For users pointing at `api.dr-sentry.com`, skip this.

## 5. Model your org

Whether you click through the dashboard or paste into your MCP-connected agent, the structural steps are the same:

1. **Organization** — top-level; you already have one.
2. **Environments** — org-scoped (`production`, `staging`, `preview`). Every app inherits them.
3. **Projects** — group related apps for access control and UI grouping.
4. **Applications** — the unit flags and deployments target.
5. **Members + roles** — add teammates with org/project roles.

See [README → Authentication](../README.md#authentication) and [README → Applications](../README.md#applications) for concrete examples.

## 6. Start tracking deployments

Two paths depending on where your app runs:

- **On a PaaS (Railway, Render, Fly, Heroku, Vercel, Netlify, GitHub Actions):** configure a webhook integration. The MCP's `ds_reporting_setup_deploy_integration` tool does the whole setup in one call.
- **Self-hosted / CI-driven:** call `POST /api/v1/deployments` from your pipeline, or use the SDK-embedded status reporter.

Full guide: [Deploy Integration Guide](./Deploy_Integration_Guide.md).

## 7. Ship a feature flag

Install one of the first-party SDKs:

```bash
npm install @dr-sentry/sdk                             # Node / TypeScript
go   get    github.com/deploysentry/deploysentry-go    # Go
pip  install deploysentry                              # Python
# Java, Ruby via their usual package managers
```

SDK walkthroughs: [sdk-onboarding.md](./sdk-onboarding.md). Pick a flag category (`release`, `feature`, `experiment`, `ops`, `permission`); release flags need an `expires_at` or `is_permanent=true`.

## 8. See what's happening

- **Status page** (`/orgs/:slug/status`) — compact project-grouped heatmap of every app × env with health, current version, and per-app links to whichever monitoring tool you use. Auto-refreshes every 15 s.
- **Deploy History** (`/orgs/:slug/deployments`) — chronological list, filterable by project/app/env/status/mode/date range.
- **Webhooks + Slack + PagerDuty** — configure in org/project settings.

## Bootstrap an existing codebase with an LLM

Already have an app you want to wire up fast? Paste the [Bootstrap_My_App.md](./Bootstrap_My_App.md) prompt into your LLM tool. It auto-detects SDKs, sets up env vars, creates the flag-registration file, and wires everything in one conversation — assuming the MCP server is installed (Step 3 above).

## Going to production

Before running DeploySentry in production, review [PRODUCTION.md](./PRODUCTION.md) for the hardening checklist: confirmation dialogs, rollback/history requirements, managed secrets, NATS auth + TLS.

## Where to go next

- [Bootstrap_My_App.md](./Bootstrap_My_App.md) — one-shot LLM prompt to integrate DeploySentry into an existing codebase.
- [Deploy_Integration_Guide.md](./Deploy_Integration_Guide.md) — full PaaS webhook + CI reference, every supported provider.
- [DEVELOPMENT.md](./DEVELOPMENT.md) — local dev loop, testing, schema conventions.
- [sdk-onboarding.md](./sdk-onboarding.md) — per-language SDK walkthroughs including the agentless status reporter.
- [Current_Initiatives.md](./Current_Initiatives.md) — what's in flight.
