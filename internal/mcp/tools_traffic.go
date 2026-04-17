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

	// Step 1: Fetch agents.
	data, err := c.get(fmt.Sprintf("/api/v1/applications/%s/agents", appID))
	if err != nil {
		return errResult(err), nil
	}

	agents, ok := data["agents"].([]interface{})
	if !ok || len(agents) == 0 {
		return jsonResult(map[string]interface{}{
			"status":  "no_agents",
			"message": "No agents registered for this application.",
		})
	}

	// Step 2: Find first connected agent.
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

	agentID, ok := connectedAgent["id"].(string)
	if !ok {
		return errResult(fmt.Errorf("connected agent missing id")), nil
	}

	// Step 3: Fetch heartbeats.
	hbData, err := c.get(fmt.Sprintf("/api/v1/agents/%s/heartbeats", agentID))
	if err != nil {
		return errResult(err), nil
	}

	heartbeats, ok := hbData["heartbeats"].([]interface{})
	if !ok || len(heartbeats) == 0 {
		return jsonResult(map[string]interface{}{
			"status":       "ok",
			"agent_id":     agentID,
			"agent_status": connectedAgent["status"],
			"last_seen_at": connectedAgent["last_seen_at"],
			"message":      "Agent connected but no heartbeats received yet.",
		})
	}

	// Extract payload from latest heartbeat.
	latest, ok := heartbeats[0].(map[string]interface{})
	if !ok {
		return errResult(fmt.Errorf("unexpected heartbeat format")), nil
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

// ---------------------------------------------------------------------------
// ds_setup_local_deploy
// ---------------------------------------------------------------------------

var setupLocalDeployTool = mcp.NewTool("ds_setup_local_deploy",
	mcp.WithDescription("Get step-by-step instructions for setting up the local multi-instance Docker environment with Envoy traffic splitting."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func handleSetupLocalDeploy(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	instructions := `# Local Deploy Environment Setup

## Prerequisites
- Docker and Docker Compose installed
- DeploySentry API running (make run-api)
- An org, project, and application created in DeploySentry

## Step 1: Set Environment Variables

| Variable               | Description                            | Example                        |
|------------------------|----------------------------------------|--------------------------------|
| DEPLOYSENTRY_URL       | API base URL                           | http://localhost:8080          |
| DEPLOYSENTRY_API_KEY   | API key with deploy:write scope        | ds_key_abc123                  |
| DEPLOYSENTRY_ORG       | Organization slug                      | my-org                         |
| DEPLOYSENTRY_PROJECT   | Project slug                           | my-project                     |
| DS_APP_ID              | Application UUID                       | (from ds_list_apps)            |
| DS_ENV_ID              | Environment UUID                       | (from ds_list_environments)    |

Export these in your shell or add them to ~/.deploysentry.yml:
  org: my-org
  project: my-project
  api_url: http://localhost:8080

## Step 2: Start the Infrastructure

  make dev-up          # Start PostgreSQL, Redis, NATS
  make migrate-up      # Run database migrations
  make run-api         # Start the API server

## Step 3: Launch the Deploy Environment

  make dev-deploy      # Start Envoy proxy, app instances, and agent

This starts:
- Envoy proxy on port 9901 (admin) and 10000 (traffic)
- Two application instances (v1 and v2)
- A DeploySentry agent that registers with the API and sends heartbeats

## Step 4: Verify Agent Connection

Use ds_list_agents to confirm the agent registered and shows status "connected".
Use ds_get_traffic_state to see the current traffic split and health metrics.

## Step 5: Create Your First Deployment

Use the CLI or API to create a canary deployment:

  deploysentry deploy create \
    --app <app-slug> \
    --env <env-slug> \
    --strategy canary \
    --release v2.0.0

The agent will pick up the new routing rules and Envoy will begin splitting traffic.

## Canary Testing with Feature Flags

You can also use feature flags to control canary rollout:

1. Create a flag with category "release":
   deploysentry flags create --name canary-v2 --category release --expires-at 2026-05-01

2. Add targeting rules to route a percentage of traffic to v2.

3. Monitor with ds_get_traffic_state to verify the split is applied.

## Troubleshooting

- Agent not connecting? Check DEPLOYSENTRY_URL and API key are correct.
- No heartbeats? Ensure the agent container can reach the API on the Docker network.
- Traffic not splitting? Check Envoy admin at http://localhost:9901/clusters.
`
	return mcp.NewToolResultText(instructions), nil
}
