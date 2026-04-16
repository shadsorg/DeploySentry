package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// ds_list_orgs
// ---------------------------------------------------------------------------

var listOrgsTool = mcp.NewTool("ds_list_orgs",
	mcp.WithDescription("List all organizations the authenticated user belongs to."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func handleListOrgs(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.get("/api/v1/orgs")
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_list_projects
// ---------------------------------------------------------------------------

var listProjectsTool = mcp.NewTool("ds_list_projects",
	mcp.WithDescription("List projects in an organization."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
)

func handleListProjects(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	org, err := resolveOrg(req.GetString("org", ""))
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects", org))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_list_apps
// ---------------------------------------------------------------------------

var listAppsTool = mcp.NewTool("ds_list_apps",
	mcp.WithDescription("List applications in a project."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
	mcp.WithString("project", mcp.Description("Project slug (uses default from config if omitted)")),
)

func handleListApps(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps", org, project))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}

// ---------------------------------------------------------------------------
// ds_list_environments
// ---------------------------------------------------------------------------

var listEnvsTool = mcp.NewTool("ds_list_environments",
	mcp.WithDescription("List environments defined at the organization level."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("org", mcp.Description("Organization slug (uses default from config if omitted)")),
)

func handleListEnvironments(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	org, err := resolveOrg(req.GetString("org", ""))
	if err != nil {
		return errResult(err), nil
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/environments", org))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(data)
}
