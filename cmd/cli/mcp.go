package main

import (
	"fmt"

	mcpserver "github.com/shadsorg/deploysentry/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol (MCP) server for AI assistants",
	Long: `Run DeploySentry as an MCP server so AI assistants like Claude can
manage organizations, projects, feature flags, and deployments.

Add this to your Claude Code MCP config (~/.claude/mcp.json):

  {
    "mcpServers": {
      "deploysentry": {
        "command": "deploysentry",
        "args": ["mcp", "serve"]
      }
    }
  }`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP stdio server",
	Long: `Start the DeploySentry MCP server using stdio transport.
The server exposes tools for managing orgs, projects, apps, flags,
deployments, and API keys.

Configuration is read from ~/.deploysentry.yml, environment variables
(DEPLOYSENTRY_URL, DEPLOYSENTRY_API_KEY, DEPLOYSENTRY_ORG, etc.),
and ~/.config/deploysentry/credentials.json (OAuth).

Example Claude Code config (~/.claude/mcp.json):

  {
    "mcpServers": {
      "deploysentry": {
        "command": "deploysentry",
        "args": ["mcp", "serve"]
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := mcpserver.NewServer()
		if err := server.ServeStdio(s); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}
