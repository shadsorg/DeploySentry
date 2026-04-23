package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// environmentsCmd is the parent command for org-scoped environment operations.
//
// Environments in DeploySentry are org-level (every application inherits the
// same set); this command surfaces that first-class concept in the CLI so
// callers don't have to curl the raw API.
var environmentsCmd = &cobra.Command{
	Use:     "environments",
	Aliases: []string{"environment", "envs", "env"},
	Short:   "Manage org-level environments",
	Long: `List and inspect environments in the current organization.

Environments are defined at the org level (production, staging, preview,
etc.) and are inherited by every application in the org. This command is
the CLI surface for the environments themselves; for environment-scoped
key/value settings, use 'deploysentry settings list --scope environment'.

Examples:
  # List all environments in the current org
  deploysentry environments list

  # Human-readable output (tab-separated table) is default;
  # JSON is available for scripting
  deploysentry environments list -o json`,
}

var environmentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments in the current organization",
	Long: `List all environments defined at the organization level.

Requires --org (or DEPLOYSENTRY_ORG, or 'org' set via 'deploysentry orgs set').`,
	RunE: runEnvironmentsList,
}

func init() {
	environmentsCmd.AddCommand(environmentsListCmd)
	rootCmd.AddCommand(environmentsCmd)
}

func runEnvironmentsList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/environments", org)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	envs, _ := resp["environments"].([]interface{})
	if len(envs) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No environments found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SLUG\tNAME\tPRODUCTION\tSORT")
	for _, e := range envs {
		env, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		slug, _ := env["slug"].(string)
		name, _ := env["name"].(string)
		isProd, _ := env["is_production"].(bool)
		sortOrder, _ := env["sort_order"].(float64)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%v\t%d\n", slug, name, isProd, int(sortOrder))
	}
	return w.Flush()
}
