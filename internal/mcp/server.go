package mcpserver

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates an MCP server with all DeploySentry tools registered.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"deploysentry",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Status
	s.AddTool(statusTool, handleStatus)

	// Entities
	s.AddTool(listOrgsTool, handleListOrgs)
	s.AddTool(listProjectsTool, handleListProjects)
	s.AddTool(listAppsTool, handleListApps)
	s.AddTool(listEnvsTool, handleListEnvironments)

	// Deploy & API keys
	s.AddTool(createAPIKeyTool, handleCreateAPIKey)
	s.AddTool(appDeployStatusTool, handleAppDeployStatus)
	s.AddTool(generateWorkflowTool, handleGenerateWorkflow)
	s.AddTool(createDeploymentTool, handleCreateDeployment)
	s.AddTool(promoteDeploymentTool, handlePromoteDeployment)
	s.AddTool(rollbackDeploymentTool, handleRollbackDeployment)
	s.AddTool(advanceDeploymentTool, handleAdvanceDeployment)
	s.AddTool(deploymentPhasesTool, handleDeploymentPhases)

	// Flags
	s.AddTool(listFlagsTool, handleListFlags)
	s.AddTool(getFlagTool, handleGetFlag)
	s.AddTool(createFlagTool, handleCreateFlag)
	s.AddTool(toggleFlagTool, handleToggleFlag)

	// Agents & Traffic
	s.AddTool(listAgentsTool, handleListAgents)
	s.AddTool(getTrafficStateTool, handleGetTrafficState)
	s.AddTool(setupLocalDeployTool, handleSetupLocalDeploy)

	return s
}
