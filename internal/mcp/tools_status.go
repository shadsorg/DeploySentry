package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
)

var statusTool = mcp.NewTool("ds_status",
	mcp.WithDescription("Check DeploySentry CLI configuration and authentication status. Reports org, project, API URL, and any issues with fix instructions."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func handleStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := map[string]interface{}{
		"api_url": viper.GetString("api_url"),
		"org":     viper.GetString("org"),
		"project": viper.GetString("project"),
	}

	var issues []string

	_, err := checkReady()
	if err != nil {
		status["authenticated"] = false
		issues = append(issues, fmt.Sprintf("Auth: %s", err.Error()))
	} else {
		status["authenticated"] = true
	}

	if viper.GetString("org") == "" {
		issues = append(issues, "No default org configured. Set DEPLOYSENTRY_ORG or add 'org' to ~/.deploysentry.yml, or pass org parameter to each tool.")
	}
	if viper.GetString("project") == "" {
		issues = append(issues, "No default project configured. Set DEPLOYSENTRY_PROJECT or add 'project' to ~/.deploysentry.yml, or pass project parameter to each tool.")
	}

	if len(issues) > 0 {
		status["issues"] = issues
	} else {
		status["issues"] = "none"
	}

	return jsonResult(status)
}
