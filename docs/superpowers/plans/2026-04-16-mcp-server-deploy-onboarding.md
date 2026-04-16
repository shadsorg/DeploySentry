# MCP Server & Deployment Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `deploysentry mcp serve` subcommand that exposes DeploySentry operations as MCP tools, enabling an LLM to set up deployment tracking for a GitHub repo in one conversation.

**Architecture:** The MCP server is a Cobra subcommand that speaks JSON-RPC 2.0 over stdio using the `mcp-go` library. It reuses the CLI's `apiClient`, config (Viper), and auth (OAuth/API key) infrastructure. Tools are organized by domain: entities (list orgs/projects/apps/envs), deploy (create API key, generate workflow), flags (CRUD), and status (readiness check). Each tool validates auth before executing and returns structured JSON with actionable errors.

**Tech Stack:** Go, Cobra, Viper, `github.com/mark3labs/mcp-go` (MCP SDK), existing `cmd/cli` HTTP client

**Spec:** `docs/superpowers/specs/2026-04-16-mcp-server-deploy-onboarding-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `cmd/cli/mcp.go` | Cobra subcommand: `deploysentry mcp serve` |
| `internal/mcp/server.go` | MCP server setup: create server, register all tools, run stdio transport |
| `internal/mcp/context.go` | Shared context: build apiClient from CLI config, auth validation helper |
| `internal/mcp/tools_status.go` | `ds_status` tool — readiness check |
| `internal/mcp/tools_entities.go` | `ds_list_orgs`, `ds_list_projects`, `ds_list_apps`, `ds_list_environments` |
| `internal/mcp/tools_deploy.go` | `ds_create_api_key`, `ds_get_app_deploy_status`, `ds_generate_workflow` |
| `internal/mcp/tools_flags.go` | `ds_list_flags`, `ds_get_flag`, `ds_create_flag`, `ds_toggle_flag` |

### Modified Files

| File | Change |
|------|--------|
| `go.mod` | Add `github.com/mark3labs/mcp-go` dependency |

---

## Task 1: Add mcp-go Dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the mcp-go library**

Run: `go get github.com/mark3labs/mcp-go@latest`

- [ ] **Step 2: Verify it resolves**

Run: `go mod tidy && go build ./...`
Expected: No errors, mcp-go in go.sum

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add mcp-go library for MCP server support"
```

---

## Task 2: MCP Context — Shared Client Builder

**Files:**
- Create: `internal/mcp/context.go`

- [ ] **Step 1: Create the shared context module**

This module builds an API client from CLI config (Viper + credentials) and provides an auth check helper that every tool calls before doing work.

```go
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// apiClient is a lightweight HTTP client for the DeploySentry API.
type apiClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string
}

// newClientFromConfig builds an apiClient from Viper config and stored credentials.
func newClientFromConfig() (*apiClient, error) {
	apiURL := viper.GetString("api_url")
	if apiURL == "" {
		apiURL = os.Getenv("DEPLOYSENTRY_URL")
	}
	if apiURL == "" {
		apiURL = "https://dr-sentry.com"
	}

	client := &apiClient{
		baseURL:    strings.TrimRight(apiURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Try API key first (env var or config)
	apiKey := os.Getenv("DEPLOYSENTRY_API_KEY")
	if apiKey == "" {
		apiKey = viper.GetString("api_key")
	}
	if apiKey != "" {
		client.apiKey = apiKey
		return client, nil
	}

	// Try OAuth token from credentials file
	token, err := loadAccessToken()
	if err == nil && token != "" {
		client.token = token
		return client, nil
	}

	return nil, fmt.Errorf("not authenticated")
}

// loadAccessToken reads the stored OAuth access token.
func loadAccessToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".config", "deploysentry", "credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var creds struct {
		AccessToken string    `json:"access_token"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", err
	}
	if time.Now().After(creds.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return creds.AccessToken, nil
}

// checkReady verifies the client can authenticate. Returns nil or an error
// with an actionable message for the LLM to relay.
func checkReady() (*apiClient, error) {
	client, err := newClientFromConfig()
	if err != nil {
		return nil, fmt.Errorf("not authenticated — run `deploysentry auth login` or set DEPLOYSENTRY_API_KEY")
	}
	return client, nil
}

// getOrg returns the configured org slug, or empty string.
func getOrg() string {
	if org := os.Getenv("DEPLOYSENTRY_ORG"); org != "" {
		return org
	}
	return viper.GetString("org")
}

// getProject returns the configured project slug, or empty string.
func getProject() string {
	if proj := os.Getenv("DEPLOYSENTRY_PROJECT"); proj != "" {
		return proj
	}
	return viper.GetString("project")
}

// resolveOrg returns the org slug from the parameter or config. Returns error if neither set.
func resolveOrg(param string) (string, error) {
	if param != "" {
		return param, nil
	}
	org := getOrg()
	if org == "" {
		return "", fmt.Errorf("no organization configured — pass org_slug parameter or run `deploysentry config set org <slug>`")
	}
	return org, nil
}

// resolveProject returns the project slug from the parameter or config. Returns error if neither set.
func resolveProject(param string) (string, error) {
	if param != "" {
		return param, nil
	}
	proj := getProject()
	if proj == "" {
		return "", fmt.Errorf("no project configured — pass project_slug parameter or run `deploysentry config set project <slug>`")
	}
	return proj, nil
}

// --- HTTP helpers ---

func (c *apiClient) doRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	url := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]interface{}
		_ = json.Unmarshal(data, &errBody)
		msg := fmt.Sprintf("API error %d", resp.StatusCode)
		if errMsg, ok := errBody["error"].(string); ok {
			msg = errMsg
		}
		return nil, fmt.Errorf("%s", msg)
	}

	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

func (c *apiClient) get(path string) (map[string]interface{}, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

func (c *apiClient) post(path string, body interface{}) (map[string]interface{}, error) {
	return c.doRequest(http.MethodPost, path, body)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/context.go
git commit -m "feat: add MCP shared context — client builder, auth check, HTTP helpers"
```

---

## Task 3: MCP Server Core + Status Tool

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools_status.go`

- [ ] **Step 1: Create the MCP server setup**

```go
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the MCP server with all tools registered.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"DeploySentry",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Status
	registerStatusTools(s)

	// Entity discovery
	registerEntityTools(s)

	// Deployment onboarding
	registerDeployTools(s)

	// Flag management
	registerFlagTools(s)

	return s
}
```

- [ ] **Step 2: Create the ds_status tool**

```go
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerStatusTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("ds_status",
			mcp.WithDescription("Check if the DeploySentry CLI is authenticated and configured. Returns current org, project, API URL, and any missing config with instructions to fix."),
		),
		handleStatus,
	)
}

func handleStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result := map[string]interface{}{
		"authenticated": false,
		"org":           "",
		"project":       "",
		"api_url":       "",
		"issues":        []string{},
	}

	issues := []string{}

	// Check auth
	client, err := newClientFromConfig()
	if err != nil {
		issues = append(issues, "Not authenticated. Run `deploysentry auth login` or set DEPLOYSENTRY_API_KEY environment variable.")
	} else {
		result["authenticated"] = true
		result["api_url"] = client.baseURL
	}

	// Check org
	org := getOrg()
	if org == "" {
		issues = append(issues, "No organization configured. Run `deploysentry config set org <slug>` or set DEPLOYSENTRY_ORG.")
	} else {
		result["org"] = org
	}

	// Check project
	project := getProject()
	if project == "" {
		issues = append(issues, "No project configured. Run `deploysentry config set project <slug>` or set DEPLOYSENTRY_PROJECT.")
	} else {
		result["project"] = project
	}

	result["issues"] = issues

	if len(issues) > 0 {
		return mcp.NewToolResultText(fmt.Sprintf("DeploySentry CLI has configuration issues:\n%s\n\nCurrent state: org=%q, project=%q", formatIssues(issues), org, project)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("DeploySentry CLI is ready. Org: %s, Project: %s, API: %s", org, project, client.baseURL)), nil
}

func formatIssues(issues []string) string {
	out := ""
	for i, issue := range issues {
		out += fmt.Sprintf("  %d. %s\n", i+1, issue)
	}
	return out
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools_status.go
git commit -m "feat: add MCP server core and ds_status tool"
```

---

## Task 4: Entity Discovery Tools

**Files:**
- Create: `internal/mcp/tools_entities.go`

- [ ] **Step 1: Implement the four entity listing tools**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEntityTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("ds_list_orgs",
			mcp.WithDescription("List organizations the current user belongs to."),
		),
		handleListOrgs,
	)

	s.AddTool(
		mcp.NewTool("ds_list_projects",
			mcp.WithDescription("List projects in an organization."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
		),
		handleListProjects,
	)

	s.AddTool(
		mcp.NewTool("ds_list_apps",
			mcp.WithDescription("List applications in a project."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
			mcp.WithString("project_slug", mcp.Description("Project slug. Uses CLI default if omitted.")),
		),
		handleListApps,
	)

	s.AddTool(
		mcp.NewTool("ds_list_environments",
			mcp.WithDescription("List environments in an organization."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
		),
		handleListEnvironments,
	)
}

func handleListOrgs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resp, err := client.get("/api/v1/orgs")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list organizations: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects", org))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleListApps(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	projectSlug, _ := request.Params.Arguments["project_slug"].(string)

	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := resolveProject(projectSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps", org, project))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list applications: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleListEnvironments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/orgs/%s/environments", org))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %s", err)), nil
	}

	return jsonResult(resp)
}

// jsonResult marshals a map to indented JSON and returns it as a text result.
func jsonResult(data interface{}) (*mcp.CallToolResult, error) {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format result: %s", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/tools_entities.go
git commit -m "feat: add MCP entity discovery tools — list orgs, projects, apps, environments"
```

---

## Task 5: Deployment Onboarding Tools

**Files:**
- Create: `internal/mcp/tools_deploy.go`

- [ ] **Step 1: Implement the three deployment tools**

```go
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDeployTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("ds_create_api_key",
			mcp.WithDescription("Create a DeploySentry API key, optionally scoped to specific environments. Returns the plaintext key (shown only once)."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable name for the key (e.g., 'github-actions-deploy')")),
			mcp.WithString("scopes", mcp.Required(), mcp.Description("Comma-separated permission scopes (e.g., 'deploys:read,deploys:write,flags:read')")),
			mcp.WithString("environment_ids", mcp.Description("Comma-separated environment UUIDs to restrict key to. Omit for unrestricted.")),
		),
		handleCreateAPIKey,
	)

	s.AddTool(
		mcp.NewTool("ds_get_app_deploy_status",
			mcp.WithDescription("Check if an application has any deployments, active webhooks, or configured deployment tracking."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
			mcp.WithString("project_slug", mcp.Description("Project slug. Uses CLI default if omitted.")),
			mcp.WithString("app_slug", mcp.Required(), mcp.Description("Application slug.")),
		),
		handleGetAppDeployStatus,
	)

	s.AddTool(
		mcp.NewTool("ds_generate_workflow",
			mcp.WithDescription("Generate a GitHub Actions workflow YAML step for recording deployments in DeploySentry. Returns YAML that should be added to an existing workflow after the build/deploy step."),
			mcp.WithString("app_id", mcp.Required(), mcp.Description("Application UUID.")),
			mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment UUID.")),
			mcp.WithString("strategy", mcp.Description("Deployment strategy: rolling (default), canary, or blue_green.")),
		),
		handleGenerateWorkflow,
	)
}

func handleCreateAPIKey(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	name, _ := request.Params.Arguments["name"].(string)
	scopesRaw, _ := request.Params.Arguments["scopes"].(string)
	envIDsRaw, _ := request.Params.Arguments["environment_ids"].(string)

	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if scopesRaw == "" {
		return mcp.NewToolResultError("scopes is required (e.g., 'deploys:read,deploys:write,flags:read')"), nil
	}

	scopes := splitTrim(scopesRaw, ",")

	body := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}

	if envIDsRaw != "" {
		envIDs := splitTrim(envIDsRaw, ",")
		body["environment_ids"] = envIDs
	}

	resp, err := client.post("/api/v1/api-keys", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create API key: %s", err)), nil
	}

	// Extract the plaintext key for display
	token, _ := resp["token"].(string)
	keyData, _ := resp["api_key"].(map[string]interface{})

	resultText := fmt.Sprintf("API key created successfully.\n\nKey: %s\nName: %s\nScopes: %s\n\nIMPORTANT: Store this key securely. It will not be shown again.",
		token, name, scopesRaw)

	if keyData != nil {
		if id, ok := keyData["id"].(string); ok {
			resultText += fmt.Sprintf("\nKey ID: %s", id)
		}
	}

	return mcp.NewToolResultText(resultText), nil
}

func handleGetAppDeployStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	projectSlug, _ := request.Params.Arguments["project_slug"].(string)
	appSlug, _ := request.Params.Arguments["app_slug"].(string)

	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := resolveProject(projectSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if appSlug == "" {
		return mcp.NewToolResultError("app_slug is required"), nil
	}

	// Get app details
	appResp, err := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, project, appSlug))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application: %s", err)), nil
	}

	// Try to get recent deployments
	app, _ := appResp["application"].(map[string]interface{})
	appID := ""
	if app != nil {
		appID, _ = app["id"].(string)
	}

	status := map[string]interface{}{
		"application":     app,
		"has_deployments": false,
		"deployment_count": 0,
	}

	if appID != "" {
		deploysResp, err := client.get(fmt.Sprintf("/api/v1/deployments?app_id=%s&limit=5", appID))
		if err == nil {
			if deploys, ok := deploysResp["deployments"].([]interface{}); ok {
				status["has_deployments"] = len(deploys) > 0
				status["deployment_count"] = len(deploys)
				status["recent_deployments"] = deploys
			}
		}
	}

	return jsonResult(status)
}

func handleGenerateWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appID, _ := request.Params.Arguments["app_id"].(string)
	envID, _ := request.Params.Arguments["env_id"].(string)
	strategy, _ := request.Params.Arguments["strategy"].(string)

	if appID == "" {
		return mcp.NewToolResultError("app_id is required"), nil
	}
	if envID == "" {
		return mcp.NewToolResultError("env_id is required"), nil
	}
	if strategy == "" {
		strategy = "rolling"
	}

	workflow := fmt.Sprintf(`# Add this step to your existing GitHub Actions workflow,
# after your build and deploy steps.
#
# Required GitHub secrets:
#   DS_API_KEY  — DeploySentry API key (deploys:read, deploys:write)
#   DS_APP_ID   — %s
#   DS_ENV_ID   — %s
#   DS_API_URL  — DeploySentry API URL (default: https://dr-sentry.com)

- name: Record deployment in DeploySentry
  if: success()
  env:
    DS_API_KEY: ${{ secrets.DS_API_KEY }}
    DS_API_URL: ${{ secrets.DS_API_URL || 'https://dr-sentry.com' }}
    DS_APP_ID: ${{ secrets.DS_APP_ID }}
    DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
  run: |
    curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
      -H "Authorization: ApiKey ${DS_API_KEY}" \
      -H "Content-Type: application/json" \
      -d '{
        "application_id": "'"${DS_APP_ID}"'",
        "environment_id": "'"${DS_ENV_ID}"'",
        "strategy": "%s",
        "version": "${{ github.sha }}",
        "commit_sha": "${{ github.sha }}",
        "artifact": "${{ github.repository }}",
        "description": "Deployed from GitHub Actions (${{ github.ref_name }})"
      }'`, appID, envID, strategy)

	return mcp.NewToolResultText(workflow), nil
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/tools_deploy.go
git commit -m "feat: add MCP deployment onboarding tools — create API key, deploy status, generate workflow"
```

---

## Task 6: Flag Management Tools

**Files:**
- Create: `internal/mcp/tools_flags.go`

- [ ] **Step 1: Implement the four flag tools**

```go
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFlagTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("ds_list_flags",
			mcp.WithDescription("List feature flags for the current project."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
			mcp.WithString("project_slug", mcp.Description("Project slug. Uses CLI default if omitted.")),
		),
		handleListFlags,
	)

	s.AddTool(
		mcp.NewTool("ds_get_flag",
			mcp.WithDescription("Get details for a specific feature flag by ID."),
			mcp.WithString("flag_id", mcp.Required(), mcp.Description("Flag UUID.")),
		),
		handleGetFlag,
	)

	s.AddTool(
		mcp.NewTool("ds_create_flag",
			mcp.WithDescription("Create a new feature flag."),
			mcp.WithString("org_slug", mcp.Description("Organization slug. Uses CLI default if omitted.")),
			mcp.WithString("project_slug", mcp.Description("Project slug. Uses CLI default if omitted.")),
			mcp.WithString("key", mcp.Required(), mcp.Description("Flag key (e.g., 'enable-dark-mode').")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable flag name.")),
			mcp.WithString("flag_type", mcp.Required(), mcp.Description("Flag type: boolean, string, integer, or json.")),
			mcp.WithString("category", mcp.Required(), mcp.Description("Flag category: release, feature, experiment, ops, or permission.")),
			mcp.WithString("default_value", mcp.Description("Default value (e.g., 'false', '\"hello\"', '42').")),
			mcp.WithString("description", mcp.Description("Flag description.")),
		),
		handleCreateFlag,
	)

	s.AddTool(
		mcp.NewTool("ds_toggle_flag",
			mcp.WithDescription("Toggle a feature flag on or off."),
			mcp.WithString("flag_id", mcp.Required(), mcp.Description("Flag UUID.")),
			mcp.WithBoolean("enabled", mcp.Required(), mcp.Description("Set to true to enable, false to disable.")),
		),
		handleToggleFlag,
	)
}

func handleListFlags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	projectSlug, _ := request.Params.Arguments["project_slug"].(string)

	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := resolveProject(projectSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list flags: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleGetFlag(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	flagID, _ := request.Params.Arguments["flag_id"].(string)
	if flagID == "" {
		return mcp.NewToolResultError("flag_id is required"), nil
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/flags/%s", flagID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get flag: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleCreateFlag(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	orgSlug, _ := request.Params.Arguments["org_slug"].(string)
	projectSlug, _ := request.Params.Arguments["project_slug"].(string)

	org, err := resolveOrg(orgSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := resolveProject(projectSlug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	key, _ := request.Params.Arguments["key"].(string)
	name, _ := request.Params.Arguments["name"].(string)
	flagType, _ := request.Params.Arguments["flag_type"].(string)
	category, _ := request.Params.Arguments["category"].(string)
	defaultValue, _ := request.Params.Arguments["default_value"].(string)
	description, _ := request.Params.Arguments["description"].(string)

	if key == "" || name == "" || flagType == "" || category == "" {
		return mcp.NewToolResultError("key, name, flag_type, and category are all required"), nil
	}

	body := map[string]interface{}{
		"key":       key,
		"name":      name,
		"flag_type": flagType,
		"category":  category,
	}
	if defaultValue != "" {
		body["default_value"] = defaultValue
	}
	if description != "" {
		body["description"] = description
	}

	resp, err := client.post(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project), body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create flag: %s", err)), nil
	}

	return jsonResult(resp)
}

func handleToggleFlag(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := checkReady()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	flagID, _ := request.Params.Arguments["flag_id"].(string)
	enabled, _ := request.Params.Arguments["enabled"].(bool)

	if flagID == "" {
		return mcp.NewToolResultError("flag_id is required"), nil
	}

	body := map[string]interface{}{
		"enabled": enabled,
	}

	resp, err := client.post(fmt.Sprintf("/api/v1/flags/%s/toggle", flagID), body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle flag: %s", err)), nil
	}

	return jsonResult(resp)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/mcp/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/tools_flags.go
git commit -m "feat: add MCP flag management tools — list, get, create, toggle"
```

---

## Task 7: Cobra Subcommand — `deploysentry mcp serve`

**Files:**
- Create: `cmd/cli/mcp.go`

- [ ] **Step 1: Create the mcp subcommand**

```go
package main

import (
	"fmt"
	"os"

	mcpserver "github.com/deploysentry/deploysentry/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for LLM integrations",
	Long:  "Model Context Protocol server that exposes DeploySentry operations as tools for Claude Code and other LLM assistants.",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server (stdio transport)",
	Long: `Start the DeploySentry MCP server using stdio transport.

Add this to your Claude Code MCP configuration:

  {
    "mcpServers": {
      "deploysentry": {
        "command": "deploysentry",
        "args": ["mcp", "serve"]
      }
    }
  }

The server inherits authentication from the CLI. Run 'deploysentry auth login'
or set DEPLOYSENTRY_API_KEY before starting.`,
	RunE: runMCPServe,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	s := mcpserver.NewServer()

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return err
	}

	return nil
}
```

- [ ] **Step 2: Verify full CLI builds**

Run: `go build ./cmd/cli/...`
Expected: No errors

- [ ] **Step 3: Test the subcommand help text**

Run: `go run ./cmd/cli/... mcp serve --help`
Expected: Shows the help text with MCP config example

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/mcp.go
git commit -m "feat: add 'deploysentry mcp serve' subcommand"
```

---

## Task 8: Integration Test — Run the MCP Server

- [ ] **Step 1: Build the CLI binary**

Run: `go build -o /tmp/deploysentry-mcp-test ./cmd/cli/`
Expected: Binary builds successfully

- [ ] **Step 2: Test tools/list via stdio**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | /tmp/deploysentry-mcp-test mcp serve 2>/dev/null
```

Expected: JSON response containing tool definitions for `ds_status`, `ds_list_orgs`, `ds_list_projects`, `ds_list_apps`, `ds_list_environments`, `ds_create_api_key`, `ds_get_app_deploy_status`, `ds_generate_workflow`, `ds_list_flags`, `ds_get_flag`, `ds_create_flag`, `ds_toggle_flag` (12 tools total).

- [ ] **Step 3: Test ds_status tool call**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ds_status","arguments":{}}}' | /tmp/deploysentry-mcp-test mcp serve 2>/dev/null
```

Expected: Response with authentication status and current config.

- [ ] **Step 4: Test ds_generate_workflow tool call**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ds_generate_workflow","arguments":{"app_id":"test-app-id","env_id":"test-env-id","strategy":"canary"}}}' | /tmp/deploysentry-mcp-test mcp serve 2>/dev/null
```

Expected: Response containing a GitHub Actions YAML step with the provided app_id, env_id, and canary strategy.

- [ ] **Step 5: Commit test results (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address issues found during MCP server integration testing"
```

---

## Task 9: Build Binary and Update Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Build the production binary**

Run: `go build -o bin/deploysentry-api ./cmd/api/ && go build -o bin/deploysentry ./cmd/cli/`
Expected: Both binaries build successfully

- [ ] **Step 2: Verify full project**

Run: `go build ./... && cd web && npx tsc --noEmit && echo "ALL BUILDS PASS"`
Expected: Clean build

- [ ] **Step 3: Update current initiatives**

Add to `docs/Current_Initiatives.md`:

```markdown
| MCP Server & Deploy Onboarding | Implementation | [Plan](./superpowers/plans/2026-04-16-mcp-server-deploy-onboarding.md) / [Spec](./superpowers/specs/2026-04-16-mcp-server-deploy-onboarding-design.md) | On `feature/groups-and-resource-authorization` branch. 12 MCP tools, stdio transport, deployment onboarding flow. Pending merge. |
```

- [ ] **Step 4: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add MCP Server & Deploy Onboarding to current initiatives"
```
