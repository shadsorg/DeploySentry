# Deployment Docs & MCP Traffic Tools

**Date:** 2026-04-17
**Status:** Design

## Overview

Two deliverables that complete the sidecar traffic management feature:

1. **Traffic Management Guide** (`docs/Traffic_Management_Guide.md`) — A dedicated guide covering the agent/Envoy sidecar system, local Docker setup, flag canaries, dashboard observability, and PaaS patterns. Separate from the existing Deploy Integration Guide (which covers CI/CD triggering, not runtime traffic control).

2. **MCP tools for deployment lifecycle and traffic management** — 8 new tools plus a `resolveApp` helper that enable LLM-assisted deployment operations and traffic observability. Tools use slug-first parameters with auto-resolution to UUIDs.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Doc location | Separate guide (`Traffic_Management_Guide.md`) | Existing Deploy Integration Guide is CI/CD-focused. Traffic management is a different concern — runtime control, not deployment triggering. |
| MCP parameter style | Slug-first with auto-resolution | LLMs work better with human-readable names. Resolution logic already exists in `context.go`. Add `resolveApp` for the app slug → UUID step. |
| Tool scope | Both setup and operational | Setup tools help onboarding, operational tools are what make the MCP useful day-to-day. Each tool is small and focused. |

## MCP Tools

### Resolution Helper

Add `resolveApp(org, project, app string) (string, error)` to `internal/mcp/context.go`. Resolves org + project + app slugs to the app UUID by calling `GET /api/v1/orgs/:org/projects/:project/apps/:app`. Falls back to Viper config for org/project if not provided.

### Deployment Lifecycle Tools (`internal/mcp/tools_deploy.go`)

**`ds_create_deployment`** — Create a new deployment.
- Parameters: `org` (optional), `project` (optional), `app` (required), `env` (required), `version` (required), `strategy` (optional, default "rolling", enum: rolling/canary/blue-green), `flag_test_key` (optional — links deployment to a flag for canary testing)
- Resolves org/project/app/env slugs to UUIDs
- POSTs to `POST /api/v1/deployments`
- Returns: deployment object with ID, status, strategy

**`ds_promote_deployment`** — Promote a deployment to 100% traffic.
- Parameters: `deployment_id` (required)
- POSTs to `POST /api/v1/deployments/:id/promote`
- Returns: updated deployment

**`ds_rollback_deployment`** — Trigger rollback.
- Parameters: `deployment_id` (required), `reason` (optional)
- POSTs to `POST /api/v1/deployments/:id/rollback`
- Returns: updated deployment

**`ds_advance_deployment`** — Advance to the next canary phase (manual gate).
- Parameters: `deployment_id` (required)
- POSTs to `POST /api/v1/deployments/:id/advance`
- Returns: updated deployment with new phase info

### Agent & Traffic Tools (`internal/mcp/tools_traffic.go`)

**`ds_list_agents`** — List registered agents for an application.
- Parameters: `org` (optional), `project` (optional), `app` (required)
- Resolves app slug to UUID
- GETs `GET /api/v1/applications/:app_id/agents`
- Returns: agents list with status, version, last seen, upstream config

**`ds_get_traffic_state`** — Get combined traffic state: desired vs. actual split, per-version metrics, agent health, active routing rules.
- Parameters: `org` (optional), `project` (optional), `app` (required)
- Resolves app slug to UUID
- Fetches agents via `GET /api/v1/applications/:app_id/agents`, then heartbeats for the first connected agent via `GET /api/v1/agents/:id/heartbeats`
- Returns: combined JSON with agent status, actual traffic distribution, per-upstream metrics (RPS, error rate, P99, P50), active rules (weights, header overrides, sticky sessions), envoy health, config version
- This is the most LLM-useful tool — an LLM can look at the response and recommend promote/rollback/wait

**`ds_setup_local_deploy`** — Print setup instructions for the local multi-instance Docker environment.
- No parameters
- Returns: step-by-step text instructions for running `make dev-deploy`, configuring env vars, verifying agent connection, and creating a first deployment
- Pure text, no API calls — this is a helper for onboarding

**`ds_deployment_phases`** — List phases for a deployment with their status.
- Parameters: `deployment_id` (required)
- GETs `GET /api/v1/deployments/:id/phases`
- Returns: phases list with name, status, traffic percent, duration, auto-promote flag

## Documentation

### New: `docs/Traffic_Management_Guide.md`

Sections:

1. **Overview** — What the agent does, architecture summary (agent + Envoy + xDS), when to use traffic splitting vs. feature flag percentage rollout.

2. **Local Setup** — Step-by-step:
   - Prerequisites (Docker, DeploySentry API running)
   - `make dev-deploy` to start Envoy + agent + blue/green
   - Configuring `DS_APP_ID`, `DS_API_KEY`, `DS_UPSTREAMS`
   - Verifying agent connection
   - Creating first canary deployment and watching traffic shift

3. **Agent Configuration Reference** — Table of all env vars with defaults and examples:
   - `DS_API_URL` (default: `http://localhost:8080`)
   - `DS_API_KEY` (required)
   - `DS_APP_ID` (required, UUID)
   - `DS_ENVIRONMENT` (default: `production`)
   - `DS_UPSTREAMS` (format: `blue:host:port,green:host:port`)
   - `DS_ENVOY_XDS_PORT` (default: `18000`)
   - `DS_ENVOY_LISTEN_PORT` (default: `8080`)
   - `DS_HEARTBEAT_INTERVAL` (default: `5s`)

4. **Traffic Splitting** — How Envoy weights work, header overrides for developer testing (`X-Version: canary`), sticky sessions.

5. **Flag Canaries** — The `SERVICE_COLOR` pattern: same-version deployment, SDK auto-detection, creating a targeting rule, `--flag-test` mode, reading the "Flags Under Test" dashboard panel.

6. **Dashboard Observability** — What the traffic panel shows: desired vs. actual bars, per-version metrics cards, agent status indicators, traffic rules summary, how to read it during a rollout.

7. **PaaS Deployment** — Pattern guidance for Render/Railway/Fly.io: agent as entrypoint, co-located blue/green processes, `PORT` env var handling. Guidance only, not step-by-step templates.

8. **Troubleshooting** — Common issues table covering: agent not connecting, Envoy not picking up xDS config, traffic not shifting, heartbeat gaps, dashboard not showing traffic panel.

### Modified: `docs/Deploy_Integration_Guide.md`

Add a one-line cross-reference near the top:

> For traffic splitting with Envoy and the DeploySentry agent sidecar, see the [Traffic Management Guide](./Traffic_Management_Guide.md).

### Modified: `README.md`

Add a bullet to the Deployments section:

> **Traffic Management** — Control traffic splitting with the DeploySentry agent sidecar and Envoy proxy. See the [Traffic Management Guide](docs/Traffic_Management_Guide.md).

## Scope Boundaries

**In scope:**
- 8 new MCP tools + `resolveApp` helper
- New `tools_traffic.go` file for agent/traffic tools
- Additions to existing `tools_deploy.go` for deployment lifecycle tools
- New `docs/Traffic_Management_Guide.md`
- Cross-reference updates to Deploy Integration Guide and README

**Out of scope:**
- MCP tools for targeting rule management (create/update/delete rules)
- MCP tools for SDK configuration
- PaaS-specific deployment templates in documentation
- Agent auto-update documentation
