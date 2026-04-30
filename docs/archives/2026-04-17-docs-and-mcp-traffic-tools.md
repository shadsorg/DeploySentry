# Deployment Docs & MCP Traffic Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 8 MCP tools for deployment lifecycle and traffic management, plus a Traffic Management Guide and cross-reference updates.

**Architecture:** New MCP tools follow the existing pattern in `internal/mcp/` — tool definition vars, handler functions, slug-first parameters with `resolveOrg`/`resolveProject`/`resolveApp` auto-resolution. A new `tools_traffic.go` file holds agent/traffic tools. Deployment lifecycle tools extend `tools_deploy.go`. Documentation is a new standalone guide with cross-references from existing docs.

**Tech Stack:** Go 1.25, `mark3labs/mcp-go`, Markdown

---

## File Structure

### New Files

```
internal/mcp/tools_traffic.go          — Agent & traffic MCP tools (4 tools)
docs/Traffic_Management_Guide.md       — Full sidecar/Envoy/agent documentation
```

### Modified Files

```
internal/mcp/context.go                — Add resolveApp helper
internal/mcp/tools_deploy.go           — Add 4 deployment lifecycle tools
internal/mcp/server.go                 — Register all new tools
docs/Deploy_Integration_Guide.md       — Add cross-reference to traffic guide
README.md                              — Add traffic management bullet
```

---

### Task 1: Add resolveApp Helper to context.go

**Files:**
- Modify: `internal/mcp/context.go`

- [ ] **Step 1: Add resolveApp function**

Add after the existing `resolveProject` function (~line 116):

```go
// resolveApp resolves an application slug to its UUID by calling the API.
// Requires org and project to be already resolved.
func resolveApp(c *apiClient, org, project, app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("app is required: pass it as a parameter")
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, project, app))
	if err != nil {
		return "", fmt.Errorf("failed to resolve app '%s': %w", app, err)
	}
	id, ok := data["id"].(string)
	if !ok {
		return "", fmt.Errorf("app '%s' not found or missing id in response", app)
	}
	return id, nil
}

// resolveEnv resolves an environment slug to its UUID by calling the API.
func resolveEnv(c *apiClient, org, env string) (string, error) {
	if env == "" {
		return "", fmt.Errorf("env is required: pass it as a parameter")
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/environments", org))
	if err != nil {
		return "", fmt.Errorf("failed to list environments: %w", err)
	}
	envs, ok := data["environments"].([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected environments response format")
	}
	for _, e := range envs {
		em, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if em["slug"] == env || em["name"] == env {
			if id, ok := em["id"].(string); ok {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("environment '%s' not found", env)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/context.go
git commit -m "feat(mcp): add resolveApp and resolveEnv helpers for slug-to-UUID resolution"
```

---

### Task 2: Deployment Lifecycle MCP Tools

**Files:**
- Modify: `internal/mcp/tools_deploy.go`

- [ ] **Step 1: Add ds_create_deployment tool**

Append to `internal/mcp/tools_deploy.go`:

```go
// ---------------------------------------------------------------------------
// ds_create_deployment
// ---------------------------------------------------------------------------

var createDeploymentTool = mcp.NewTool("ds_create_deployment",
	mcp.WithDescription("Create a new deployment. Supports canary, blue-green, and rolling strategies. Optionally link to a feature flag for flag-canary testing."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
	mcp.WithString("app", mcp.Required(), mcp.Description("Application slug")),
	mcp.WithString("env", mcp.Required(), mcp.Description("Environment slug (e.g. 'production', 'staging')")),
	mcp.WithString("version", mcp.Required(), mcp.Description("Version to deploy (e.g. 'v2.1.0')")),
	mcp.WithString("strategy", mcp.Description("Deployment strategy: rolling, canary, blue-green (default: rolling)"),
		mcp.Enum("rolling", "canary", "blue-green")),
	mcp.WithString("flag_test_key", mcp.Description("Feature flag key to canary-test with this deployment (optional)")),
)

func handleCreateDeployment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	org, err := resolveOrg(req.GetString("org", ""))
	if err != nil {
		return errResult(err), nil
	}
	project, err := resolveProject(req.GetString("project", ""))
	if err != nil {
		return errResult(err), nil
	}
	appSlug, err := req.RequireString("app")
	if err != nil {
		return errResult(err), nil
	}
	envSlug, err := req.RequireString("env")
	if err != nil {
		return errResult(err), nil
	}
	version, err := req.RequireString("version")
	if err != nil {
		return errResult(err), nil
	}

	appID, err := resolveApp(c, org, project, appSlug)
	if err != nil {
		return errResult(err), nil
	}
	envID, err := resolveEnv(c, org, envSlug)
	if err != nil {
		return errResult(err), nil
	}

	body := map[string]interface{}{
		"app_id":         appID,
		"environment_id": envID,
		"version":        version,
		"strategy":       req.GetString("strategy", "rolling"),
	}
	if ftk := req.GetString("flag_test_key", ""); ftk != "" {
		body["flag_test_key"] = ftk
	}

	data, err := c.post("/api/v1/deployments", body)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 2: Add ds_promote_deployment tool**

```go
// ---------------------------------------------------------------------------
// ds_promote_deployment
// ---------------------------------------------------------------------------

var promoteDeploymentTool = mcp.NewTool("ds_promote_deployment",
	mcp.WithDescription("Promote a deployment to 100% traffic immediately."),
	mcp.WithString("deployment_id", mcp.Required(), mcp.Description("Deployment ID (UUID)")),
)

func handlePromoteDeployment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	id, err := req.RequireString("deployment_id")
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.post(fmt.Sprintf("/api/v1/deployments/%s/promote", id), nil)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 3: Add ds_rollback_deployment tool**

```go
// ---------------------------------------------------------------------------
// ds_rollback_deployment
// ---------------------------------------------------------------------------

var rollbackDeploymentTool = mcp.NewTool("ds_rollback_deployment",
	mcp.WithDescription("Rollback a deployment. Returns traffic to the previous version."),
	mcp.WithString("deployment_id", mcp.Required(), mcp.Description("Deployment ID (UUID)")),
	mcp.WithString("reason", mcp.Description("Reason for the rollback (optional)")),
)

func handleRollbackDeployment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	id, err := req.RequireString("deployment_id")
	if err != nil {
		return errResult(err), nil
	}
	var body map[string]interface{}
	if reason := req.GetString("reason", ""); reason != "" {
		body = map[string]interface{}{"reason": reason}
	}
	data, err := c.post(fmt.Sprintf("/api/v1/deployments/%s/rollback", id), body)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 4: Add ds_advance_deployment tool**

```go
// ---------------------------------------------------------------------------
// ds_advance_deployment
// ---------------------------------------------------------------------------

var advanceDeploymentTool = mcp.NewTool("ds_advance_deployment",
	mcp.WithDescription("Advance a canary deployment to its next phase (manual gate). Only works when a deployment is paused at a phase gate."),
	mcp.WithString("deployment_id", mcp.Required(), mcp.Description("Deployment ID (UUID)")),
)

func handleAdvanceDeployment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	id, err := req.RequireString("deployment_id")
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.post(fmt.Sprintf("/api/v1/deployments/%s/advance", id), nil)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 5: Add ds_deployment_phases tool**

```go
// ---------------------------------------------------------------------------
// ds_deployment_phases
// ---------------------------------------------------------------------------

var deploymentPhasesTool = mcp.NewTool("ds_deployment_phases",
	mcp.WithDescription("List all phases of a deployment with their status, traffic percentage, and duration."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("deployment_id", mcp.Required(), mcp.Description("Deployment ID (UUID)")),
)

func handleDeploymentPhases(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	id, err := req.RequireString("deployment_id")
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.get(fmt.Sprintf("/api/v1/deployments/%s/phases", id))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 6: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tools_deploy.go
git commit -m "feat(mcp): add deployment lifecycle tools (create, promote, rollback, advance, phases)"
```

---

### Task 3: Agent & Traffic MCP Tools

**Files:**
- Create: `internal/mcp/tools_traffic.go`

- [ ] **Step 1: Create tools_traffic.go with ds_list_agents**

```go
package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// ds_list_agents
// ---------------------------------------------------------------------------

var listAgentsTool = mcp.NewTool("ds_list_agents",
	mcp.WithDescription("List registered DeploySentry agents for an application. Shows agent status, version, upstream config, and last heartbeat time."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
	mcp.WithString("app", mcp.Required(), mcp.Description("Application slug")),
)

func handleListAgents(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	org, err := resolveOrg(req.GetString("org", ""))
	if err != nil {
		return errResult(err), nil
	}
	project, err := resolveProject(req.GetString("project", ""))
	if err != nil {
		return errResult(err), nil
	}
	appSlug, err := req.RequireString("app")
	if err != nil {
		return errResult(err), nil
	}

	appID, err := resolveApp(c, org, project, appSlug)
	if err != nil {
		return errResult(err), nil
	}

	data, err := c.get(fmt.Sprintf("/api/v1/applications/%s/agents", appID))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
```

- [ ] **Step 2: Add ds_get_traffic_state tool**

Append to `tools_traffic.go`:

```go
// ---------------------------------------------------------------------------
// ds_get_traffic_state
// ---------------------------------------------------------------------------

var getTrafficStateTool = mcp.NewTool("ds_get_traffic_state",
	mcp.WithDescription("Get real-time traffic state for an application: desired vs actual traffic split, per-version metrics (RPS, error rate, latency), agent health, and active routing rules. Use this to assess whether a canary deployment is healthy."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
	mcp.WithString("app", mcp.Required(), mcp.Description("Application slug")),
)

func handleGetTrafficState(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	org, err := resolveOrg(req.GetString("org", ""))
	if err != nil {
		return errResult(err), nil
	}
	project, err := resolveProject(req.GetString("project", ""))
	if err != nil {
		return errResult(err), nil
	}
	appSlug, err := req.RequireString("app")
	if err != nil {
		return errResult(err), nil
	}

	appID, err := resolveApp(c, org, project, appSlug)
	if err != nil {
		return errResult(err), nil
	}

	// Fetch agents for this app.
	agentsData, err := c.get(fmt.Sprintf("/api/v1/applications/%s/agents", appID))
	if err != nil {
		return errResult(fmt.Errorf("failed to fetch agents: %w", err)), nil
	}

	agents, ok := agentsData["agents"].([]interface{})
	if !ok || len(agents) == 0 {
		return jsonResult(map[string]interface{}{
			"status":  "no_agents",
			"message": "No agents registered for this application. Run 'make dev-deploy' to start the agent sidecar.",
		})
	}

	// Find first connected agent.
	var connectedAgent map[string]interface{}
	for _, a := range agents {
		am, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		if am["status"] == "connected" {
			connectedAgent = am
			break
		}
	}
	if connectedAgent == nil {
		return jsonResult(map[string]interface{}{
			"status": "no_connected_agents",
			"agents": agents,
		})
	}

	agentID, _ := connectedAgent["id"].(string)

	// Fetch latest heartbeats.
	heartbeats, err := c.get(fmt.Sprintf("/api/v1/agents/%s/heartbeats", agentID))
	if err != nil {
		return errResult(fmt.Errorf("failed to fetch heartbeats: %w", err)), nil
	}

	hbList, ok := heartbeats["heartbeats"].([]interface{})
	if !ok || len(hbList) == 0 {
		return jsonResult(map[string]interface{}{
			"status": "no_heartbeats",
			"agent":  connectedAgent,
		})
	}

	// Extract the latest heartbeat payload.
	latest, ok := hbList[0].(map[string]interface{})
	if !ok {
		return jsonResult(map[string]interface{}{
			"status": "heartbeat_parse_error",
			"agent":  connectedAgent,
		})
	}

	payload, _ := latest["payload"].(map[string]interface{})

	result := map[string]interface{}{
		"status":         "ok",
		"agent_id":       agentID,
		"agent_status":   connectedAgent["status"],
		"last_seen_at":   connectedAgent["last_seen_at"],
		"actual_traffic": payload["actual_traffic"],
		"upstreams":      payload["upstreams"],
		"active_rules":   payload["active_rules"],
		"envoy_healthy":  payload["envoy_healthy"],
		"config_version": payload["config_version"],
	}

	return jsonResult(result)
}
```

- [ ] **Step 3: Add ds_setup_local_deploy tool**

Append to `tools_traffic.go`:

```go
// ---------------------------------------------------------------------------
// ds_setup_local_deploy
// ---------------------------------------------------------------------------

var setupLocalDeployTool = mcp.NewTool("ds_setup_local_deploy",
	mcp.WithDescription("Get step-by-step instructions for setting up the local multi-instance Docker environment with Envoy traffic splitting."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func handleSetupLocalDeploy(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instructions := `# Local Traffic Management Setup

## Prerequisites
- Docker and Docker Compose installed
- DeploySentry API running (make run-api)
- An application and environment created in DeploySentry

## Steps

1. Start backing services (if not already running):
   make dev-up

2. Set your app ID and API key:
   export DS_APP_ID=<your-app-uuid>
   export DS_API_KEY=<your-api-key>

3. Start the multi-instance deploy environment:
   make dev-deploy

   This starts:
   - app-blue (stable, port 8081) with SERVICE_COLOR=blue
   - app-green (canary, port 8082) with SERVICE_COLOR=green
   - Envoy proxy (port 8080) — routes traffic based on xDS weights
   - deploysentry-agent (port 18000) — xDS control plane

4. Verify the agent is connected:
   curl -s http://localhost:8080/api/v1/applications/$DS_APP_ID/agents | jq

5. Create a canary deployment:
   deploysentry deploy create --strategy canary --version v2.0 --env production

6. Watch traffic shift on the dashboard (http://localhost:3001)
   or check via API:
   curl -s http://localhost:8080/api/v1/agents/<agent-id>/heartbeats | jq '.[0].payload.actual_traffic'

## Environment Variables

| Variable              | Default              | Description                    |
|-----------------------|----------------------|--------------------------------|
| DS_API_URL            | http://localhost:8080 | DeploySentry API URL           |
| DS_API_KEY            |                      | API key for agent auth         |
| DS_APP_ID             |                      | Application UUID (required)    |
| DS_ENVIRONMENT        | production           | Environment name               |
| DS_UPSTREAMS          | blue:localhost:8081,green:localhost:8082 | Upstream mapping |
| DS_ENVOY_XDS_PORT     | 18000                | xDS gRPC listen port           |
| DS_ENVOY_LISTEN_PORT  | 8080                 | Envoy listener port            |
| DS_HEARTBEAT_INTERVAL | 5s                   | Heartbeat frequency            |
| DS_APP_IMAGE          | deploysentry/demo-app:latest | Docker image for blue/green |

## Flag Canary Testing

To canary-test a feature flag (same code, different flag state):
1. Deploy the same version to both blue and green
2. Create a targeting rule: service_color eq green → flag enabled
3. Create deployment with --flag-test:
   deploysentry deploy create --strategy canary --version v1.0.0 --flag-test my-feature --env production
4. 5% of traffic (green) sees the flag ON, 95% (blue) sees it OFF
`
	return mcp.NewToolResultText(instructions), nil
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools_traffic.go
git commit -m "feat(mcp): add agent and traffic management tools (list-agents, traffic-state, setup-local)"
```

---

### Task 4: Register New Tools in server.go

**Files:**
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Add tool registrations**

In `internal/mcp/server.go`, add two new sections after the existing "Flags" block:

```go
	// Deploy lifecycle
	s.AddTool(createDeploymentTool, handleCreateDeployment)
	s.AddTool(promoteDeploymentTool, handlePromoteDeployment)
	s.AddTool(rollbackDeploymentTool, handleRollbackDeployment)
	s.AddTool(advanceDeploymentTool, handleAdvanceDeployment)
	s.AddTool(deploymentPhasesTool, handleDeploymentPhases)

	// Traffic & agents
	s.AddTool(listAgentsTool, handleListAgents)
	s.AddTool(getTrafficStateTool, handleGetTrafficState)
	s.AddTool(setupLocalDeployTool, handleSetupLocalDeploy)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/server.go
git commit -m "feat(mcp): register deployment lifecycle and traffic tools in server"
```

---

### Task 5: Traffic Management Guide

**Files:**
- Create: `docs/Traffic_Management_Guide.md`

- [ ] **Step 1: Write the guide**

Create `docs/Traffic_Management_Guide.md` with the following content:

```markdown
# Traffic Management Guide

Control traffic splitting between application versions using the DeploySentry agent sidecar and Envoy proxy.

## Overview

The DeploySentry agent is a Go binary that runs alongside your application. It acts as an Envoy xDS control plane — receiving desired traffic state from the DeploySentry API via SSE, pushing weight configurations to Envoy in real-time, and reporting actual traffic metrics back via heartbeats.

```
┌─────────────────────────────────────────┐
│            DeploySentry API             │
│  (deployment engine, desired state)     │
└──────────┬──────────────▲───────────────┘
           │ SSE          │ Heartbeats
           ▼              │
┌──────────────────────────────────────────┐
│          deploysentry-agent              │
│  • Receives desired traffic %            │
│  • Pushes xDS config to Envoy            │
│  • Reports actual traffic + metrics      │
└──────────┬──────────────────────────────┘
           │ xDS (gRPC)
           ▼
┌──────────────────────────────────────────┐
│              Envoy Proxy                 │
│  • Routes traffic by weight              │
│  • Header overrides for testing          │
│  upstream: app-blue  weight=95           │
│  upstream: app-green weight=5            │
└──────┬────────────────────┬─────────────┘
       ▼                    ▼
  ┌──────────┐        ┌──────────┐
  │ app-blue │        │ app-green│
  │ (stable) │        │ (canary) │
  └──────────┘        └──────────┘
```

**When to use traffic splitting vs. flag percentage rollout:**

| Approach | Use when | How it works |
|----------|----------|-------------|
| Traffic splitting (this guide) | Different code versions or testing infrastructure changes | Envoy routes N% of all requests to the new version |
| Flag percentage rollout | Same code, different behavior | SDK hashes user ID to deterministically enable for N% of users |

Traffic splitting gives you per-version metrics (error rate, latency) that flag rollouts don't — you can see exactly how v2.0 is performing vs. v1.0.

## Local Setup

### Prerequisites

- Docker and Docker Compose installed
- DeploySentry API running (`make run-api`)
- An application and environment created in DeploySentry

### Step 1: Start Backing Services

```bash
make dev-up    # Starts PostgreSQL, Redis, NATS
make run-api   # Starts API server on :8080
```

### Step 2: Configure the Agent

Set your application ID and API key:

```bash
export DS_APP_ID=<your-app-uuid>
export DS_API_KEY=<your-api-key>
```

Find your app ID:
```bash
deploysentry apps list
# or via API:
curl -s http://localhost:8080/api/v1/orgs/<org>/projects/<project>/apps | jq
```

### Step 3: Start the Multi-Instance Environment

```bash
make dev-deploy
```

This starts four containers on the `deploysentry-net` Docker network:

| Container | Port | Role |
|-----------|------|------|
| `deploysentry-app-blue` | 8081 | Stable version (`SERVICE_COLOR=blue`) |
| `deploysentry-app-green` | 8082 | Canary version (`SERVICE_COLOR=green`) |
| `deploysentry-envoy` | 8080 | Traffic router (Envoy proxy) |
| `deploysentry-agent` | 18000 | xDS control plane |

### Step 4: Verify Agent Connection

```bash
curl -s http://localhost:8080/api/v1/applications/$DS_APP_ID/agents \
  -H "Authorization: ApiKey $DS_API_KEY" | jq '.agents[0].status'
# Expected: "connected"
```

Check the dashboard at http://localhost:3001 — navigate to a deployment detail page to see the traffic panel.

### Step 5: Create a Canary Deployment

```bash
deploysentry deploy create \
  --strategy canary \
  --version v2.0.0 \
  --env production
```

Watch traffic shift through the five canary phases (1% → 5% → 25% → 50% → 100%) on the dashboard or via:

```bash
# Check current traffic state
curl -s http://localhost:8080/api/v1/agents/<agent-id>/heartbeats \
  -H "Authorization: ApiKey $DS_API_KEY" | jq '.[0].payload.actual_traffic'
```

## Agent Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `DS_API_URL` | `http://localhost:8080` | DeploySentry API base URL |
| `DS_API_KEY` | — | API key for agent authentication (required) |
| `DS_APP_ID` | — | Application UUID to manage (required) |
| `DS_ENVIRONMENT` | `production` | Environment name |
| `DS_UPSTREAMS` | `blue:localhost:8081,green:localhost:8082` | Upstream mapping (`name:host:port,...`) |
| `DS_ENVOY_XDS_PORT` | `18000` | Port for the xDS gRPC server |
| `DS_ENVOY_LISTEN_PORT` | `8080` | Port Envoy listens on for incoming traffic |
| `DS_HEARTBEAT_INTERVAL` | `5s` | How often the agent reports metrics back |

## Traffic Splitting

### Weighted Routing

The default behavior. Envoy distributes traffic by weight — e.g., 95% to blue, 5% to green. Weights are updated dynamically via xDS when the deployment engine advances canary phases.

### Header Overrides

During a canary rollout, you can bypass the weight-based routing to test the canary directly:

```bash
# Always route to the canary (green) version
curl -H "X-Version: canary" http://localhost:8080/your-endpoint
```

Header overrides take precedence over weight-based routing. Configure them when creating the deployment or via the deployment detail dashboard.

### Sticky Sessions

When enabled, a user is pinned to one version for the duration of their session. Prevents users from bouncing between blue and green mid-workflow.

Configure via the deployment creation API:
```json
{
  "strategy": "canary",
  "version": "v2.0.0",
  "sticky_sessions": {
    "enabled": true,
    "strategy": "cookie",
    "ttl": "30m"
  }
}
```

## Flag Canaries

Test a feature flag change through traffic splitting — same code version on both blue and green, but the flag is only enabled for green traffic.

### How It Works

1. **Both instances run the same code.** `app-blue` has `SERVICE_COLOR=blue`, `app-green` has `SERVICE_COLOR=green`.
2. **The SDK auto-detects `SERVICE_COLOR`.** Every flag evaluation automatically includes `service_color` in the context — no code changes needed.
3. **Create a targeting rule:** `service_color eq green` → flag enabled.
4. **Create a flag-test deployment:**

```bash
deploysentry deploy create \
  --strategy canary \
  --version v1.0.0 \
  --flag-test my-new-feature \
  --env production
```

5. **Result:** 5% of traffic hits green (flag ON), 95% hits blue (flag OFF). The dashboard shows the "Flags Under Test" panel with per-color evaluation results.

### Why Use This Instead of Flag Percentage Rollout?

Flag percentage rollout (via the SDK's hash-based bucketing) works great for most cases. Use flag canaries when you need:

- **Per-version metrics** — See the exact error rate and latency difference between flag-on and flag-off traffic
- **Instant kill switch** — Rollback the deployment to route 100% away from the flag-on traffic, faster than disabling the flag via the SDK cache
- **Infrastructure validation** — Test that the flag doesn't break something at the infrastructure level (memory, CPU, connection pools) that wouldn't show up in SDK-level evaluation

## Dashboard Observability

When an agent is connected, the deployment detail page shows a real-time traffic panel:

**Traffic Distribution** — Horizontal bars showing desired vs. actual traffic percentage for each upstream (blue/green). Sourced from agent heartbeats every 5 seconds.

**Per-Version Metrics** — Side-by-side cards comparing blue and green on:
- RPS (requests per second)
- Error rate (green < 1% = healthy, red >= 1% = investigate)
- P99 and P50 latency

**Agent Status** — Connection indicator (green/yellow/red), last-seen time, Envoy health, config version.

**Traffic Rules** — Active routing rules summary: weights, header overrides, sticky session config.

**Flags Under Test** — (Flag canary deployments only) Shows the flag key being tested, the `service_color eq green` targeting rule, and per-color flag state.

## PaaS Deployment

On platforms like Render, Railway, and Fly.io, you don't control the load balancer. The agent handles this by running as the service entrypoint.

### Architecture

The platform sees one service. Internally, the agent + Envoy handle routing:

```
Platform Edge LB → Your Service [:443]
                    ├── deploysentry-agent (entrypoint, receives all traffic)
                    │   └── Envoy → app-blue  :8081 (SERVICE_COLOR=blue)
                    │            → app-green :8082 (SERVICE_COLOR=green)
                    └── Agent → SSE → DeploySentry API (external)
```

### Key Considerations

- **PORT env var:** The agent must listen on the platform-assigned `PORT` (set `DS_ENVOY_LISTEN_PORT=$PORT`).
- **Process management:** Use the platform's multi-process support (Render Procfile, Railway nixpacks, Fly.io processes) or run blue/green as background processes.
- **Outbound connectivity:** The agent needs outbound HTTPS to the DeploySentry API for SSE and heartbeats.

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| Agent not connecting | API URL or API key misconfigured | Check `DS_API_URL` and `DS_API_KEY`. Verify with `curl $DS_API_URL/health` |
| Envoy not picking up config | Agent xDS port mismatch | Ensure `DS_ENVOY_XDS_PORT` matches the port in `envoy-bootstrap.yaml` |
| Traffic not shifting | No active deployment | Create a deployment: `deploysentry deploy create --strategy canary` |
| Dashboard not showing traffic panel | No agent heartbeats | Check agent logs: `docker logs deploysentry-agent`. Verify agent status via API |
| Heartbeat gaps | Network issues or agent crash | Check `docker ps` for agent container. Restart with `make dev-deploy` |
| SERVICE_COLOR not in flag context | SDK version too old | Update SDK. The `SERVICE_COLOR` auto-detection was added in the latest release |
| Blue/green both serving same content | Same image, no flag targeting | This is expected for flag canaries — the differentiation happens through flag evaluation, not code |
```

- [ ] **Step 2: Commit**

```bash
git add docs/Traffic_Management_Guide.md
git commit -m "docs: add Traffic Management Guide for sidecar, Envoy, and flag canaries"
```

---

### Task 6: Cross-Reference Updates

**Files:**
- Modify: `docs/Deploy_Integration_Guide.md`
- Modify: `README.md`

- [ ] **Step 1: Add cross-reference to Deploy Integration Guide**

Read `docs/Deploy_Integration_Guide.md` and add near the top (after the first heading or introduction paragraph):

```markdown
> **Traffic splitting:** For controlling traffic between versions with Envoy and the DeploySentry agent sidecar, see the [Traffic Management Guide](./Traffic_Management_Guide.md).
```

- [ ] **Step 2: Add traffic management bullet to README**

In `README.md`, find the Deployments section (around line 991) and add after the strategy table:

```markdown
See the [Traffic Management Guide](docs/Traffic_Management_Guide.md) for controlling traffic splitting with the DeploySentry agent sidecar and Envoy proxy, including flag canary testing.
```

- [ ] **Step 3: Commit**

```bash
git add docs/Deploy_Integration_Guide.md README.md
git commit -m "docs: add cross-references to Traffic Management Guide"
```

---

## Task Dependency Summary

```
Task 1 (resolveApp/resolveEnv) ──▶ Task 2 (deploy lifecycle tools) ──┐
                                                                       ├──▶ Task 4 (register in server.go)
Task 1 (resolveApp/resolveEnv) ──▶ Task 3 (traffic tools)  ──────────┘
Task 5 (Traffic Management Guide) — independent
Task 6 (cross-references) — independent, do last
```

**Parallel tracks:**
- Track A: Tasks 1→2→4 (MCP deploy lifecycle)
- Track B: Tasks 1→3→4 (MCP traffic tools — shares Task 1 dep with Track A)
- Track C: Task 5 (documentation)
- Track D: Task 6 (cross-references — do last)
