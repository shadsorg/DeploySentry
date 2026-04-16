# MCP Server & Deployment Onboarding

**Date**: 2026-04-16
**Status**: Approved

## Overview

Add a `deploysentry mcp serve` subcommand that runs an MCP server over stdio, exposing DeploySentry operations as tools to LLMs. The primary use case is zero-friction deployment onboarding: the LLM discovers the user's org/project/app/environments, creates an API key, sets GitHub secrets via `gh` CLI, and generates a workflow file — all in one conversation. Flag management tools are included in the structure for future expansion.

## Problem

Setting up deployment tracking today requires 30+ minutes of manual work: copying UUIDs from the dashboard, creating GitHub secrets one by one, writing workflow YAML with curl commands, and understanding the API shape. For developers using LLM-assisted tools like Claude Code, this should be a single conversational request.

## Goals

1. A `deploysentry mcp serve` subcommand that runs an MCP server over stdio
2. The server reuses the CLI's existing auth and config — no separate credentials
3. Deployment onboarding tools: list orgs/projects/apps/environments, create API keys, configure deployment tracking
4. Flag management tools stubbed into the structure (list, get, create, toggle)
5. Every tool validates prerequisites and returns actionable error messages when config is missing
6. An LLM can set up deployment tracking for a GitHub repo in one conversation using these tools + `gh` CLI

## Architecture

```
Claude Code ──stdio──► deploysentry mcp serve
                              │
                              ├── reads CLI config (~/.deploysentry.yml or env vars)
                              ├── validates auth on each tool call
                              ├── calls DeploySentry API
                              └── returns structured JSON tool results
```

The MCP server is a Go process that speaks MCP (JSON-RPC over stdio). It is a subcommand of the existing `deploysentry` CLI binary, sharing all config, auth, and HTTP client infrastructure. No separate installation or credentials.

The server does NOT handle GitHub operations directly. The LLM uses `gh` CLI (which the developer typically has authenticated) for GitHub-side operations like setting secrets.

### Future State (Out of Scope)

In the future, DeploySentry will orchestrate multi-environment promotion — calling GitHub Actions to deploy to the next environment when health gates pass. This spec does not implement that. The deployment tracking set up here is the foundation: push → build → deploy → record in DeploySentry → monitor.

## MCP Tools

### Readiness & Config

| Tool | Description | Parameters |
|------|-------------|------------|
| `ds_status` | Check if the CLI is authenticated and configured. Returns org, project, API URL, and any missing config with instructions to fix. | none |

**Behavior**: Every other tool calls an internal `checkReady()` function first. If auth is missing, the tool returns:
```json
{
  "error": "not_authenticated",
  "message": "DeploySentry CLI is not authenticated. Run `deploysentry auth login` to sign in, or set DEPLOYSENTRY_API_KEY in your environment."
}
```
If org/project context is missing (for tools that need it):
```json
{
  "error": "no_project_context",
  "message": "No project configured. Run `deploysentry config set-project <org-slug> <project-slug>` or pass org_slug and project_slug parameters."
}
```

### Entity Discovery

| Tool | Description | Parameters |
|------|-------------|------------|
| `ds_list_orgs` | List organizations the user belongs to | none |
| `ds_list_projects` | List projects in an organization | `org_slug` (string, optional — uses CLI default if omitted) |
| `ds_list_apps` | List applications in a project | `org_slug`, `project_slug` (both optional — use CLI defaults) |
| `ds_list_environments` | List environments in an organization | `org_slug` (optional) |

Each returns a JSON array of objects with `id`, `name`, `slug` at minimum.

### Deployment Onboarding

| Tool | Description | Parameters |
|------|-------------|------------|
| `ds_create_api_key` | Create an API key, optionally scoped to environments | `name` (string, required), `scopes` (string[], required), `environment_ids` (string[], optional), `project_id` (string, optional — uses CLI default) |
| `ds_get_app_deploy_status` | Check if an app has any deployments, active webhooks, or configured strategies | `org_slug`, `project_slug`, `app_slug` |
| `ds_generate_workflow` | Generate a GitHub Actions workflow YAML for recording deployments | `app_id` (string), `env_id` (string), `strategy` (string, default "rolling") |

**`ds_create_api_key`** returns:
```json
{
  "api_key": "ds_live_...",
  "key_id": "uuid",
  "name": "github-actions-deploy",
  "scopes": ["deploys:read", "deploys:write", "flags:read"],
  "environment_ids": ["uuid"],
  "warning": "Store this key securely. It will not be shown again."
}
```

**`ds_generate_workflow`** returns a complete YAML string that the LLM can write to `.github/workflows/deploy.yml`. The workflow includes:
- A `Record deployment in DeploySentry` step using curl
- References to secrets: `${{ secrets.DS_API_KEY }}`, `${{ secrets.DS_APP_ID }}`, `${{ secrets.DS_ENV_ID }}`
- The configured strategy
- Commit SHA and repository as version/artifact

The LLM is responsible for integrating this into the user's existing workflow (or creating a new one).

### Flag Management (Structural)

| Tool | Description | Parameters |
|------|-------------|------------|
| `ds_list_flags` | List flags for the current project | `org_slug`, `project_slug` (optional) |
| `ds_get_flag` | Get flag details by key | `flag_key` (string) |
| `ds_create_flag` | Create a new feature flag | `key` (string), `name` (string), `flag_type` (string), `category` (string), `default_value` (string) |
| `ds_toggle_flag` | Toggle a flag on/off | `flag_id` (string), `enabled` (boolean) |

These are simple API pass-throughs. They use the same auth/config validation as all other tools.

## The Onboarding Flow

When a user says "set up deployment tracking for this repo" in Claude Code:

```
1. LLM calls ds_status
   → Confirms CLI is authenticated, shows current org/project
   → If not authenticated: tells user to run `deploysentry auth login`

2. LLM calls ds_list_apps + ds_list_environments
   → Shows user their apps and environments
   → Asks user to confirm which app and environment to track

3. LLM calls ds_create_api_key
   → Creates key with deploys:read, deploys:write, flags:read scopes
   → Scoped to the selected environment

4. LLM runs via Bash tool:
   gh secret set DS_API_KEY --body "<key>"
   gh secret set DS_APP_ID --body "<app-uuid>"
   gh secret set DS_ENV_ID --body "<env-uuid>"
   gh secret set DS_API_URL --body "https://ds-sentry.com"

5. LLM calls ds_generate_workflow
   → Gets a YAML template
   → Writes it to .github/workflows/deploy.yml (or integrates into existing workflow)

6. LLM summarizes what was set up and suggests: "Push a commit to main to test it."
```

If `gh` CLI is not available or not authenticated, the LLM falls back to showing the user the secret values and asking them to add them manually in GitHub settings.

## Server Implementation

### Package Structure

```
cmd/cli/
  mcp.go                    ← "deploysentry mcp serve" cobra subcommand
internal/mcp/
  server.go                 ← MCP server setup: stdio transport, tool registration, JSON-RPC
  context.go                ← config loading, auth validation, API client construction
  tools_entities.go         ← ds_list_orgs, ds_list_projects, ds_list_apps, ds_list_environments
  tools_deploy.go           ← ds_create_api_key, ds_get_app_deploy_status, ds_generate_workflow
  tools_flags.go            ← ds_list_flags, ds_get_flag, ds_create_flag, ds_toggle_flag
  tools_status.go           ← ds_status
```

### MCP Protocol

The server implements the MCP specification over stdio:
- Reads JSON-RPC messages from stdin
- Writes JSON-RPC responses to stdout
- Stderr is available for logging (not part of the protocol)
- Supports `initialize`, `tools/list`, and `tools/call` methods

### Tool Registration Pattern

Each tool is registered with a name, description, JSON Schema for parameters, and a handler function:

```go
type Tool struct {
    Name        string
    Description string
    InputSchema map[string]interface{} // JSON Schema
    Handler     func(ctx context.Context, params map[string]interface{}) (interface{}, error)
}
```

Tools return structured JSON. Errors are returned as JSON objects with `error` and `message` fields, not as protocol-level errors — this gives the LLM actionable information to relay to the user.

### Config Resolution

The MCP server resolves config in this order (same as CLI):
1. Environment variables (`DEPLOYSENTRY_API_KEY`, `DEPLOYSENTRY_URL`, etc.)
2. Config file (`~/.deploysentry.yml`)
3. Tool parameters (org_slug, project_slug can override defaults per-call)

### Authentication

The server uses the CLI's existing auth mechanisms:
- API key from env var or config file
- JWT from `deploysentry auth login` session

No new auth flow is introduced. If the user isn't authenticated, `ds_status` tells them how to fix it.

## User Setup

One-time addition to Claude Code MCP config:

**`~/.claude/claude_code_config.json`** (or via Claude Code settings):
```json
{
  "mcpServers": {
    "deploysentry": {
      "command": "deploysentry",
      "args": ["mcp", "serve"]
    }
  }
}
```

Alternatively, if the CLI isn't in PATH:
```json
{
  "mcpServers": {
    "deploysentry": {
      "command": "/usr/local/bin/deploysentry",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Out of Scope

- GitHub App / OAuth integration (the LLM uses `gh` CLI instead)
- Multi-environment promotion orchestration (future — DeploySentry calling GitHub Actions)
- Branch-to-environment mapping (trunk-based: main flows through all environments)
- Provider-specific deployment templates (separate spec)
- MCP server auto-update or version negotiation
- Streaming / SSE tools (all tools are request-response)
