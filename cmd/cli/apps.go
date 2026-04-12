package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// appsCmd is the parent command for application management operations.
var appsCmd = &cobra.Command{
	Use:     "apps",
	Aliases: []string{"app"},
	Short:   "Manage applications",
	Long: `Create, list, and inspect applications within a project.

An application represents a deployable unit (service, microservice, or app)
within a DeploySentry project. Each application tracks its own releases,
feature flags, and deployment history.

Examples:
  # List all applications in the current project
  deploysentry apps list

  # Create a new application
  deploysentry apps create --name my-service --slug my-service

  # Get details for a specific application
  deploysentry apps get my-service`,
}

var appsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new application",
	Long: `Create a new application within the current project.

An application requires a name and slug, and optionally accepts a description
and repository URL.

Examples:
  # Create a basic application
  deploysentry apps create --name my-service --slug my-service

  # Create an application with full details
  deploysentry apps create --name my-service --slug my-service \
    --description "Backend microservice" \
    --repo https://github.com/myorg/my-service`,
	RunE: runAppsCreate,
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications in the project",
	Long: `List all applications in the current project.

Examples:
  # List all applications
  deploysentry apps list

  # List in JSON format
  deploysentry apps list -o json`,
	RunE: runAppsList,
}

var appsGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Get application details",
	Long: `Get details for a specific application by its slug.

Examples:
  # Get application details
  deploysentry apps get my-service

  # Get in JSON format
  deploysentry apps get my-service -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runAppsGet,
}

func init() {
	// apps create flags
	appsCreateCmd.Flags().String("name", "", "application name (required)")
	appsCreateCmd.Flags().String("slug", "", "application slug (required)")
	appsCreateCmd.Flags().String("description", "", "application description")
	appsCreateCmd.Flags().String("repo", "", "repository URL")
	_ = appsCreateCmd.MarkFlagRequired("name")
	_ = appsCreateCmd.MarkFlagRequired("slug")

	appsCmd.AddCommand(appsCreateCmd)
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsGetCmd)

	rootCmd.AddCommand(appsCmd)
}

func runAppsCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	slug, _ := cmd.Flags().GetString("slug")
	description, _ := cmd.Flags().GetString("description")
	repo, _ := cmd.Flags().GetString("repo")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"name": name,
		"slug": slug,
	}
	if description != "" {
		body["description"] = description
	}
	if repo != "" {
		body["repository_url"] = repo
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps", org, project)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	respSlug, _ := resp["slug"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Application created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:   %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Slug: %s\n", respSlug)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nAdd '--app %s' to commands or set it in .deploysentry.yml.\n", respSlug)
	return nil
}

func runAppsList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps", org, project)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	apps, _ := resp["apps"].([]interface{})
	if len(apps) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No applications found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SLUG\tNAME\tDESCRIPTION\tREPO\tCREATED")
	for _, a := range apps {
		app, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		slug, _ := app["slug"].(string)
		name, _ := app["name"].(string)
		description, _ := app["description"].(string)
		repo, _ := app["repository_url"].(string)
		createdAt, _ := app["created_at"].(string)

		// Truncate description for table display.
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		// Truncate repo for table display.
		if len(repo) > 40 {
			repo = repo[:37] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", slug, name, description, repo, createdAt)
	}
	return w.Flush()
}

func runAppsGet(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	slug := args[0]

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, project, slug)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Application: %s\n\n", slug)
	for key, value := range resp {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %v\n", key+":", value)
	}
	return nil
}
