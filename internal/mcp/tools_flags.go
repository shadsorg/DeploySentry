package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// ds_list_flags
// ---------------------------------------------------------------------------

var listFlagsTool = mcp.NewTool("ds_list_flags",
	mcp.WithDescription("List feature flags in a project."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
)

func handleListFlags(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_get_flag
// ---------------------------------------------------------------------------

var getFlagTool = mcp.NewTool("ds_get_flag",
	mcp.WithDescription("Get details of a single feature flag by ID."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("flag_id", mcp.Required(), mcp.Description("Flag ID (UUID)")),
)

func handleGetFlag(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	flagID, err := req.RequireString("flag_id")
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.get(fmt.Sprintf("/api/v1/flags/%s", flagID))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_create_flag
// ---------------------------------------------------------------------------

var createFlagTool = mcp.NewTool("ds_create_flag",
	mcp.WithDescription("Create a new feature flag in a project."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
	mcp.WithString("key", mcp.Required(), mcp.Description("Flag key (e.g. 'enable-dark-mode')")),
	mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable flag name")),
	mcp.WithString("flag_type", mcp.Required(), mcp.Description("Flag type: boolean, string, number, json"),
		mcp.Enum("boolean", "string", "number", "json")),
	mcp.WithString("category", mcp.Required(), mcp.Description("Flag category: release, feature, experiment, ops, permission"),
		mcp.Enum("release", "feature", "experiment", "ops", "permission")),
	mcp.WithString("default_value", mcp.Description("Default value for the flag (optional)")),
	mcp.WithString("description", mcp.Description("Flag description (optional)")),
)

func handleCreateFlag(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	key, err := req.RequireString("key")
	if err != nil {
		return errResult(err), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return errResult(err), nil
	}
	flagType, err := req.RequireString("flag_type")
	if err != nil {
		return errResult(err), nil
	}
	category, err := req.RequireString("category")
	if err != nil {
		return errResult(err), nil
	}

	body := map[string]interface{}{
		"key":       key,
		"name":      name,
		"flag_type": flagType,
		"category":  category,
	}
	if v := req.GetString("default_value", ""); v != "" {
		body["default_value"] = v
	}
	if v := req.GetString("description", ""); v != "" {
		body["description"] = v
	}

	data, err := c.post(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project), body)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_toggle_flag
// ---------------------------------------------------------------------------

var toggleFlagTool = mcp.NewTool("ds_toggle_flag",
	mcp.WithDescription("Toggle a feature flag on or off."),
	mcp.WithString("flag_id", mcp.Required(), mcp.Description("Flag ID (UUID)")),
	mcp.WithBoolean("enabled", mcp.Required(), mcp.Description("true to enable, false to disable")),
)

func handleToggleFlag(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	flagID, err := req.RequireString("flag_id")
	if err != nil {
		return errResult(err), nil
	}
	enabled := req.GetBool("enabled", true)

	body := map[string]interface{}{
		"enabled": enabled,
	}
	data, err := c.post(fmt.Sprintf("/api/v1/flags/%s/toggle", flagID), body)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
