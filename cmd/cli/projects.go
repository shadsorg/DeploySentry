package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// projectsCmd is the parent command for project management operations.
var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project", "proj"},
	Short:   "Manage projects",
	Long: `Create, list, and configure projects within your organization.

A project represents an application or service managed by DeploySentry.
Each project has its own environments, releases, feature flags, and
deployment configurations.

Examples:
  # Create a new project
  deploysentry projects create --name my-api --description "Backend API service"

  # List all projects
  deploysentry projects list

  # View project configuration
  deploysentry projects config`,
}

var projectsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	Long: `Create a new project within the current organization.

A project requires a name and optionally accepts a description,
default environments, and repository URL.

Examples:
  # Create a basic project
  deploysentry projects create --name my-api

  # Create a project with full details
  deploysentry projects create --name my-api \
    --description "Backend API service" \
    --repo https://github.com/myorg/my-api

  # Create with custom environments
  deploysentry projects create --name my-api \
    --environments dev,staging,production`,
	RunE: runProjectsCreate,
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects in the organization",
	Long: `List all projects in the current organization.

Examples:
  # List all projects
  deploysentry projects list

  # List in JSON format
  deploysentry projects list -o json`,
	RunE: runProjectsList,
}

var projectsConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update project configuration",
	Long: `View or update the configuration for the current project.

When called without flags, displays the current project configuration.
Use flags to update specific configuration values.

Examples:
  # View project config
  deploysentry projects config

  # Update the deployment strategy default
  deploysentry projects config --default-strategy canary

  # Update repository URL
  deploysentry projects config --repo https://github.com/myorg/my-api

  # View config in JSON format
  deploysentry projects config -o json`,
	RunE: runProjectsConfig,
}

func init() {
	// projects create flags
	projectsCreateCmd.Flags().String("name", "", "project name (required)")
	projectsCreateCmd.Flags().String("description", "", "project description")
	projectsCreateCmd.Flags().String("repo", "", "repository URL")
	projectsCreateCmd.Flags().String("environments", "dev,staging,production", "comma-separated list of environments")
	_ = projectsCreateCmd.MarkFlagRequired("name")

	// projects config flags
	projectsConfigCmd.Flags().String("default-strategy", "", "set default deployment strategy")
	projectsConfigCmd.Flags().String("repo", "", "update repository URL")
	projectsConfigCmd.Flags().String("default-env", "", "set default target environment")

	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsConfigCmd)

	rootCmd.AddCommand(projectsCmd)
}

func runProjectsCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	repo, _ := cmd.Flags().GetString("repo")
	environments, _ := cmd.Flags().GetString("environments")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"name": name,
	}
	if description != "" {
		body["description"] = description
	}
	if repo != "" {
		body["repository_url"] = repo
	}
	if environments != "" {
		body["environments"] = splitAndTrim(environments, ",")
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects", org)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	slug, _ := resp["slug"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "Project created successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:   %s\n", id)
	fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Slug: %s\n", slug)
	fmt.Fprintf(cmd.OutOrStdout(), "\nAdd '--project %s' to commands or set it in .deploysentry.yml.\n", slug)
	return nil
}

func runProjectsList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects", org)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	projects, _ := resp["projects"].([]interface{})
	if len(projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tNAME\tDESCRIPTION\tENVIRONMENTS\tCREATED")
	for _, p := range projects {
		proj, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		slug, _ := proj["slug"].(string)
		name, _ := proj["name"].(string)
		description, _ := proj["description"].(string)
		createdAt, _ := proj["created_at"].(string)

		envs := ""
		if e, ok := proj["environments"].([]interface{}); ok {
			envStrs := make([]string, 0, len(e))
			for _, env := range e {
				if s, ok := env.(string); ok {
					envStrs = append(envStrs, s)
				}
			}
			for i, s := range envStrs {
				if i > 0 {
					envs += ", "
				}
				envs += s
			}
		}

		// Truncate description for table display.
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", slug, name, description, envs, createdAt)
	}
	return w.Flush()
}

func runProjectsConfig(cmd *cobra.Command, args []string) error {
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

	// Check if any update flags are set.
	hasUpdates := cmd.Flags().Changed("default-strategy") ||
		cmd.Flags().Changed("repo") ||
		cmd.Flags().Changed("default-env")

	if hasUpdates {
		body := map[string]interface{}{}
		if cmd.Flags().Changed("default-strategy") {
			v, _ := cmd.Flags().GetString("default-strategy")
			body["default_strategy"] = v
		}
		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			body["repository_url"] = v
		}
		if cmd.Flags().Changed("default-env") {
			v, _ := cmd.Flags().GetString("default-env")
			body["default_environment"] = v
		}

		path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/config", org, project)
		resp, err := client.patch(path, body)
		if err != nil {
			return fmt.Errorf("failed to update project config: %w", err)
		}

		if getOutputFormat() == "json" {
			data, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Project configuration updated.")
		return nil
	}

	// No updates; display current config.
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/config", org, project)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get project config: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Project Configuration: %s\n\n", project)
	for key, value := range resp {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %v\n", key+":", value)
	}
	return nil
}

// splitAndTrim splits a string by a separator and trims whitespace.
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits a string by a separator.
func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
