package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// ds_create_api_key
// ---------------------------------------------------------------------------

var createAPIKeyTool = mcp.NewTool("ds_create_api_key",
	mcp.WithDescription("Create a new API key. Returns the plaintext key (shown only once)."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable name for the API key")),
	mcp.WithString("scopes", mcp.Required(), mcp.Description("Comma-separated scopes, e.g. 'deploy:write,flags:read'")),
	mcp.WithString("environment_ids", mcp.Description("Comma-separated environment IDs to restrict this key to (optional)")),
)

func handleCreateAPIKey(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}

	name, err := req.RequireString("name")
	if err != nil {
		return errResult(err), nil
	}
	scopesStr, err := req.RequireString("scopes")
	if err != nil {
		return errResult(err), nil
	}

	body := map[string]interface{}{
		"name":   name,
		"scopes": splitTrim(scopesStr),
	}

	envIDs := req.GetString("environment_ids", "")
	if envIDs != "" {
		body["environment_ids"] = splitTrim(envIDs)
	}

	data, err := c.post("/api/v1/api-keys", body)
	if err != nil {
		return errResult(err), nil
	}

	// Wrap with a warning so the LLM surfaces it to the user.
	result := map[string]interface{}{
		"warning": "Store this API key securely. It will not be shown again.",
		"data":    data,
	}
	return jsonResult(result)
}

// ---------------------------------------------------------------------------
// ds_get_app_deploy_status
// ---------------------------------------------------------------------------

var appDeployStatusTool = mcp.NewTool("ds_get_app_deploy_status",
	mcp.WithDescription("Get application details and recent deployments."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
	mcp.WithString("app", mcp.Required(), mcp.Description("Application slug")),
)

func handleAppDeployStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	app, err := req.RequireString("app")
	if err != nil {
		return errResult(err), nil
	}

	appData, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, project, app))
	if err != nil {
		return errResult(err), nil
	}

	deploys, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s/deployments", org, project, app))
	if err != nil {
		// Non-fatal: app info still useful even if deployments fail.
		deploys = map[string]interface{}{"error": err.Error()}
	}

	result := map[string]interface{}{
		"app":         appData,
		"deployments": deploys,
	}
	return jsonResult(result)
}

// ---------------------------------------------------------------------------
// ds_generate_workflow
// ---------------------------------------------------------------------------

var generateWorkflowTool = mcp.NewTool("ds_generate_workflow",
	mcp.WithDescription("Generate a GitHub Actions YAML step for recording deployments via the DeploySentry API."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("app_id", mcp.Required(), mcp.Description("Application ID (UUID)")),
	mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID (UUID)")),
	mcp.WithString("strategy", mcp.Description("Deployment strategy: rolling, canary, blue-green (default: rolling)")),
)

func handleGenerateWorkflow(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appID, err := req.RequireString("app_id")
	if err != nil {
		return errResult(err), nil
	}
	envID, err := req.RequireString("env_id")
	if err != nil {
		return errResult(err), nil
	}
	strategy := req.GetString("strategy", "rolling")

	yaml := fmt.Sprintf(`# DeploySentry deployment recording step
# Required GitHub Secrets:
#   DEPLOYSENTRY_API_KEY — an API key with deploy:write scope
#   DEPLOYSENTRY_URL     — (optional) API base URL, defaults to http://localhost:8080
- name: Record deployment in DeploySentry
  run: |
    curl -sf -X POST \
      "${DEPLOYSENTRY_URL:-http://localhost:8080}/api/v1/deployments" \
      -H "Authorization: ApiKey ${{ secrets.DEPLOYSENTRY_API_KEY }}" \
      -H "Content-Type: application/json" \
      -d '{
        "app_id": "%s",
        "environment_id": "%s",
        "strategy": "%s",
        "release": "${{ github.sha }}",
        "source": "github-actions",
        "metadata": {
          "repo": "${{ github.repository }}",
          "run_id": "${{ github.run_id }}",
          "actor": "${{ github.actor }}"
        }
      }'`, appID, envID, strategy)

	return mcp.NewToolResultText(yaml), nil
}

// splitTrim splits a comma-separated string and trims whitespace from each element.
func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
