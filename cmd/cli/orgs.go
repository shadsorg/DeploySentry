package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// orgsCmd is the parent command for organization management operations.
var orgsCmd = &cobra.Command{
	Use:     "orgs",
	Aliases: []string{"org"},
	Short:   "Manage organizations",
	Long: `Create, list, and configure organizations in DeploySentry.

An organization is the top-level grouping that contains projects, members,
and billing settings. Users may belong to one or more organizations.

Examples:
  # List all organizations you belong to
  deploysentry orgs list

  # Create a new organization
  deploysentry orgs create --name "Acme Corp" --slug acme-corp

  # Set the active organization in your local config
  deploysentry orgs set acme-corp`,
}

var orgsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new organization",
	Long: `Create a new organization in DeploySentry.

An organization requires a name and a unique slug. The slug is used
in API paths and CLI commands to identify the organization.

Examples:
  # Create a new organization
  deploysentry orgs create --name "Acme Corp" --slug acme-corp`,
	RunE: runOrgsCreate,
}

var orgsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations you belong to",
	Long: `List all organizations the authenticated user belongs to.

Examples:
  # List all organizations
  deploysentry orgs list

  # List in JSON format
  deploysentry orgs list -o json`,
	RunE: runOrgsList,
}

var orgsSetCmd = &cobra.Command{
	Use:   "set <slug>",
	Short: "Set the active organization in local config",
	Long: `Set the active organization slug in your local .deploysentry.yml config file.

This is a convenience command so you don't have to pass --org on every command.

Examples:
  # Set the active organization
  deploysentry orgs set acme-corp`,
	Args: cobra.ExactArgs(1),
	RunE: runOrgsSet,
}

func init() {
	// orgs create flags
	orgsCreateCmd.Flags().String("name", "", "organization name (required)")
	orgsCreateCmd.Flags().String("slug", "", "organization slug (required)")
	_ = orgsCreateCmd.MarkFlagRequired("name")
	_ = orgsCreateCmd.MarkFlagRequired("slug")

	orgsCmd.AddCommand(orgsCreateCmd)
	orgsCmd.AddCommand(orgsListCmd)
	orgsCmd.AddCommand(orgsSetCmd)

	rootCmd.AddCommand(orgsCmd)
}

func runOrgsCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	slug, _ := cmd.Flags().GetString("slug")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"name": name,
		"slug": slug,
	}

	resp, err := client.post("/api/v1/orgs", body)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	retSlug, _ := resp["slug"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Organization created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:   %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Slug: %s\n", retSlug)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nRun 'deploysentry orgs set %s' to make it your active organization.\n", retSlug)
	return nil
}

func runOrgsList(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	resp, err := client.get("/api/v1/orgs")
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	orgs, _ := resp["orgs"].([]interface{})
	if len(orgs) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No organizations found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SLUG\tNAME\tPLAN\tCREATED")
	for _, o := range orgs {
		org, ok := o.(map[string]interface{})
		if !ok {
			continue
		}
		slug, _ := org["slug"].(string)
		name, _ := org["name"].(string)
		plan, _ := org["plan"].(string)
		createdAt, _ := org["created_at"].(string)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", slug, name, plan, createdAt)
	}
	return w.Flush()
}

func runOrgsSet(cmd *cobra.Command, args []string) error {
	slug := args[0]

	viper.Set("org", slug)

	if err := viper.WriteConfig(); err != nil {
		// No config file exists yet; create one.
		if err2 := viper.SafeWriteConfig(); err2 != nil {
			return fmt.Errorf("failed to write config: %w", err2)
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Active organization set to %q in .deploysentry.yml.\n", slug)
	return nil
}
