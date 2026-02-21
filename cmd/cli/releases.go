package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// releasesCmd is the parent command for release operations.
var releasesCmd = &cobra.Command{
	Use:     "releases",
	Aliases: []string{"release", "rel"},
	Short:   "Manage releases",
	Long: `Create, list, and promote releases across environments.

Releases represent versioned snapshots of your application that can be
deployed to one or more environments. Use the release pipeline to
promote releases from dev through staging to production.

Examples:
  # Create a release from the current commit
  deploysentry releases create --version v1.2.0

  # List recent releases
  deploysentry releases list

  # View release status across all environments
  deploysentry releases status v1.2.0

  # Promote a release to the next environment
  deploysentry releases promote v1.2.0`,
}

var releasesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new release",
	Long: `Create a new release with a version tag and optional commit reference.

If --commit is not specified, the HEAD commit of the current branch
is used (when run inside a Git repository).

Examples:
  # Create a release with explicit version
  deploysentry releases create --version v1.2.0

  # Create a release pinned to a specific commit
  deploysentry releases create --version v1.2.0 --commit abc123def

  # Create a release with metadata
  deploysentry releases create --version v1.2.0 \
    --description "Q1 feature release" \
    --commit abc123def`,
	RunE: runReleasesCreate,
}

var releasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List releases",
	Long: `List releases for the current project, sorted by creation time.

Examples:
  # List all releases
  deploysentry releases list

  # List releases in JSON format
  deploysentry releases list -o json

  # Limit results
  deploysentry releases list --limit 10`,
	RunE: runReleasesList,
}

var releasesStatusCmd = &cobra.Command{
	Use:   "status [version]",
	Short: "Show release status across environments",
	Long: `Display the deployment status of a release across all configured
environments, showing which environments have the release deployed,
its current state, and deployment timestamps.

If no version is specified, the latest release is shown.

Examples:
  # Show status of a specific release
  deploysentry releases status v1.2.0

  # Show status of the latest release
  deploysentry releases status

  # Show status in JSON format
  deploysentry releases status v1.2.0 -o json`,
	RunE: runReleasesStatus,
}

var releasesPromoteCmd = &cobra.Command{
	Use:   "promote [version]",
	Short: "Promote a release to the next environment",
	Long: `Promote a release to the next environment in the pipeline.

The promotion pipeline is defined in your project configuration
(typically dev -> staging -> production). This command advances the
release to the next stage.

If --to is specified, the release is promoted directly to that
environment (skipping intermediate stages if allowed by policy).

Examples:
  # Promote the latest release to the next environment
  deploysentry releases promote

  # Promote a specific version
  deploysentry releases promote v1.2.0

  # Promote directly to production
  deploysentry releases promote v1.2.0 --to production

  # Promote with a specific deployment strategy
  deploysentry releases promote v1.2.0 --strategy canary`,
	RunE: runReleasesPromote,
}

func init() {
	// releases create flags
	releasesCreateCmd.Flags().String("version", "", "release version tag (required)")
	releasesCreateCmd.Flags().String("commit", "", "Git commit SHA (defaults to HEAD)")
	releasesCreateCmd.Flags().String("description", "", "release description")
	_ = releasesCreateCmd.MarkFlagRequired("version")

	// releases list flags
	releasesListCmd.Flags().Int("limit", 20, "maximum number of results")

	// releases promote flags
	releasesPromoteCmd.Flags().String("to", "", "target environment to promote to")
	releasesPromoteCmd.Flags().String("strategy", "", "deployment strategy for the promotion")

	releasesCmd.AddCommand(releasesCreateCmd)
	releasesCmd.AddCommand(releasesListCmd)
	releasesCmd.AddCommand(releasesStatusCmd)
	releasesCmd.AddCommand(releasesPromoteCmd)

	rootCmd.AddCommand(releasesCmd)
}

func runReleasesCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	version, _ := cmd.Flags().GetString("version")
	commitSHA, _ := cmd.Flags().GetString("commit")
	description, _ := cmd.Flags().GetString("description")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"version": version,
	}
	if commitSHA != "" {
		body["commit"] = commitSHA
	}
	if description != "" {
		body["description"] = description
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/releases", org, project)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	status, _ := resp["status"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "Release created successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:      %s\n", id)
	fmt.Fprintf(cmd.OutOrStdout(), "  Version: %s\n", version)
	if commitSHA != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Commit:  %s\n", commitSHA)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:  %s\n", status)
	return nil
}

func runReleasesList(cmd *cobra.Command, args []string) error {
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

	limit, _ := cmd.Flags().GetInt("limit")

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/releases?limit=%d", org, project, limit)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	releases, _ := resp["releases"].([]interface{})
	if len(releases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No releases found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tCOMMIT\tSTATUS\tENVIRONMENTS\tCREATED")
	for _, r := range releases {
		rel, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		ver, _ := rel["version"].(string)
		commit, _ := rel["commit"].(string)
		status, _ := rel["status"].(string)
		createdAt, _ := rel["created_at"].(string)

		envs := ""
		if e, ok := rel["environments"].([]interface{}); ok {
			envStrs := make([]string, 0, len(e))
			for _, env := range e {
				if s, ok := env.(string); ok {
					envStrs = append(envStrs, s)
				}
			}
			envs = strings.Join(envStrs, ", ")
		}

		// Truncate commit to short form.
		if len(commit) > 8 {
			commit = commit[:8]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ver, commit, status, envs, createdAt)
	}
	return w.Flush()
}

func runReleasesStatus(cmd *cobra.Command, args []string) error {
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

	var version string
	if len(args) > 0 {
		version = args[0]
	}

	var path string
	if version != "" {
		path = fmt.Sprintf("/api/v1/orgs/%s/projects/%s/releases/%s/status", org, project, version)
	} else {
		path = fmt.Sprintf("/api/v1/orgs/%s/projects/%s/releases/latest/status", org, project)
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get release status: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	ver, _ := resp["version"].(string)
	commit, _ := resp["commit"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "Release: %s\n", ver)
	if commit != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Commit:  %s\n", commit)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	envStatuses, _ := resp["environments"].([]interface{})
	if len(envStatuses) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Not deployed to any environment.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENVIRONMENT\tSTATUS\tDEPLOYED AT\tSTRATEGY\tPROGRESS")
	for _, e := range envStatuses {
		envStatus, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		env, _ := envStatus["environment"].(string)
		status, _ := envStatus["status"].(string)
		deployedAt, _ := envStatus["deployed_at"].(string)
		strategy, _ := envStatus["strategy"].(string)
		progress, _ := envStatus["progress"].(float64)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f%%\n",
			env, status, deployedAt, strategy, progress)
	}
	return w.Flush()
}

func runReleasesPromote(cmd *cobra.Command, args []string) error {
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

	version := "latest"
	if len(args) > 0 {
		version = args[0]
	}

	body := map[string]interface{}{}
	if to, _ := cmd.Flags().GetString("to"); to != "" {
		body["target_environment"] = to
	}
	if strategy, _ := cmd.Flags().GetString("strategy"); strategy != "" {
		body["strategy"] = strategy
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/releases/%s/promote", org, project, version)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to promote release: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	targetEnv, _ := resp["environment"].(string)
	deployID, _ := resp["deployment_id"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "Release promoted successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Version:       %s\n", version)
	fmt.Fprintf(cmd.OutOrStdout(), "  Target Env:    %s\n", targetEnv)
	fmt.Fprintf(cmd.OutOrStdout(), "  Deployment ID: %s\n", deployID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'deploysentry deploy status %s --watch' to monitor.\n", deployID)
	return nil
}
