package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// releasesCmd is the parent command for release operations.
var releasesCmd = &cobra.Command{
	Use:     "releases",
	Aliases: []string{"release", "rel"},
	Short:   "Manage releases",
	Long: `Create, list, inspect, and promote application releases.

Releases group together a set of flag changes and roll them out via a
traffic-percentage progression. Each release belongs to a single
application, identified by its slug via --app.

Examples:
  # Create a release on the "api" app
  deploysentry releases create --app api --name v1.2.0

  # List recent releases for an app
  deploysentry releases list --app api

  # Inspect a release by name
  deploysentry releases get v1.2.0 --app api

  # Promote a release to 50% traffic
  deploysentry releases promote v1.2.0 --app api --traffic-percent 50`,
}

var releasesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new release",
	Long: `Create a new release on an application.

A release is identified by a free-form --name (e.g. a version tag like
"v1.2.0", a sprint name, or any human-readable label). Releases start
in the draft state and are advanced via 'releases promote'.

Examples:
  # Create a release named v1.2.0
  deploysentry releases create --app api --name v1.2.0

  # Create a release with a description
  deploysentry releases create --app api --name v1.2.0 \
    --description "Q1 feature release"

  # Create a session-sticky release (sticky header required)
  deploysentry releases create --app api --name v1.2.0 \
    --session-sticky --sticky-header X-User-ID`,
	RunE: runReleasesCreate,
}

var releasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List releases for an application",
	Long: `List releases for the application identified by --app.

Examples:
  # List all releases for the api app
  deploysentry releases list --app api

  # JSON output
  deploysentry releases list --app api -o json`,
	RunE: runReleasesList,
}

var releasesGetCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Show details for a single release",
	Long: `Fetch a release by name (as registered when the release was created).

Examples:
  deploysentry releases get v1.2.0 --app api
  deploysentry releases get v1.2.0 --app api -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runReleasesGet,
}

var releasesPromoteCmd = &cobra.Command{
	Use:   "promote [name]",
	Short: "Advance a release to a target traffic percentage",
	Long: `Advance a release's traffic percentage. The promote endpoint accepts
a --traffic-percent value between 1 and 100.

Examples:
  # Promote v1.2.0 to 25% traffic
  deploysentry releases promote v1.2.0 --app api --traffic-percent 25

  # Full rollout
  deploysentry releases promote v1.2.0 --app api --traffic-percent 100`,
	Args: cobra.ExactArgs(1),
	RunE: runReleasesPromote,
}

func init() {
	// releases create flags
	releasesCreateCmd.Flags().String("app", "", "application slug (required)")
	releasesCreateCmd.Flags().String("name", "", "release name (required, e.g. v1.2.0)")
	releasesCreateCmd.Flags().String("description", "", "release description")
	releasesCreateCmd.Flags().Bool("session-sticky", false, "make the rollout session-sticky")
	releasesCreateCmd.Flags().String("sticky-header", "", "request header used for session stickiness")
	_ = releasesCreateCmd.MarkFlagRequired("app")
	_ = releasesCreateCmd.MarkFlagRequired("name")

	// releases list flags
	releasesListCmd.Flags().String("app", "", "application slug (required)")
	_ = releasesListCmd.MarkFlagRequired("app")

	// releases get flags
	releasesGetCmd.Flags().String("app", "", "application slug (required)")
	_ = releasesGetCmd.MarkFlagRequired("app")

	// releases promote flags
	releasesPromoteCmd.Flags().String("app", "", "application slug (required)")
	releasesPromoteCmd.Flags().Int("traffic-percent", 0, "target traffic percent (1-100, required)")
	_ = releasesPromoteCmd.MarkFlagRequired("app")
	_ = releasesPromoteCmd.MarkFlagRequired("traffic-percent")

	releasesCmd.AddCommand(releasesCreateCmd)
	releasesCmd.AddCommand(releasesListCmd)
	releasesCmd.AddCommand(releasesGetCmd)
	releasesCmd.AddCommand(releasesPromoteCmd)

	rootCmd.AddCommand(releasesCmd)
}

// resolveAppContext resolves --app to an application UUID, given the
// session's org and project slugs.
func resolveAppContext(client *apiClient, appSlug string) (appID string, err error) {
	org, err := requireOrg()
	if err != nil {
		return "", err
	}
	project, err := requireProject()
	if err != nil {
		return "", err
	}
	return resolveAppID(client, org, project, appSlug)
}

func runReleasesCreate(cmd *cobra.Command, _ []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	appSlug, _ := cmd.Flags().GetString("app")
	appID, err := resolveAppContext(client, appSlug)
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	sessionSticky, _ := cmd.Flags().GetBool("session-sticky")
	stickyHeader, _ := cmd.Flags().GetString("sticky-header")

	body := map[string]interface{}{"name": name}
	if description != "" {
		body["description"] = description
	}
	if sessionSticky {
		body["session_sticky"] = true
	}
	if stickyHeader != "" {
		body["sticky_header"] = stickyHeader
	}

	path := fmt.Sprintf("/api/v1/applications/%s/releases", appID)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	respName, _ := resp["name"].(string)
	if respName == "" {
		respName = name
	}
	status, _ := resp["status"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Release created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", respName)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", status)
	return nil
}

func runReleasesList(cmd *cobra.Command, _ []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	appSlug, _ := cmd.Flags().GetString("app")
	appID, err := resolveAppContext(client, appSlug)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/applications/%s/releases", appID)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	releases, _ := resp["releases"].([]interface{})
	if len(releases) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No releases found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSTATUS\tTRAFFIC\tCREATED")
	for _, r := range releases {
		rel, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := rel["name"].(string)
		status, _ := rel["status"].(string)
		createdAt, _ := rel["created_at"].(string)
		var trafficPct int
		if tp, ok := rel["traffic_percent"].(float64); ok {
			trafficPct = int(tp)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d%%\t%s\n", name, status, trafficPct, createdAt)
	}
	return w.Flush()
}

func runReleasesGet(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	appSlug, _ := cmd.Flags().GetString("app")
	appID, err := resolveAppContext(client, appSlug)
	if err != nil {
		return err
	}

	name := args[0]
	releaseID, err := resolveReleaseID(client, appID, name)
	if err != nil {
		return err
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/releases/%s", releaseID))
	if err != nil {
		return fmt.Errorf("failed to get release: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	respName, _ := resp["name"].(string)
	status, _ := resp["status"].(string)
	description, _ := resp["description"].(string)
	createdAt, _ := resp["created_at"].(string)
	var trafficPct int
	if tp, ok := resp["traffic_percent"].(float64); ok {
		trafficPct = int(tp)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Release: %s\n", respName)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:      %s\n", releaseID)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status:  %s\n", status)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Traffic: %d%%\n", trafficPct)
	if description != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Desc:    %s\n", description)
	}
	if createdAt != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Created: %s\n", createdAt)
	}
	return nil
}

func runReleasesPromote(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	appSlug, _ := cmd.Flags().GetString("app")
	appID, err := resolveAppContext(client, appSlug)
	if err != nil {
		return err
	}

	name := args[0]
	releaseID, err := resolveReleaseID(client, appID, name)
	if err != nil {
		return err
	}

	trafficPct, _ := cmd.Flags().GetInt("traffic-percent")
	body := map[string]interface{}{"traffic_percent": trafficPct}

	resp, err := client.post(fmt.Sprintf("/api/v1/releases/%s/promote", releaseID), body)
	if err != nil {
		return fmt.Errorf("failed to promote release: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	status, _ := resp["status"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Release promoted successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Name:    %s\n", name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:      %s\n", releaseID)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Traffic: %d%%\n", trafficPct)
	if status != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status:  %s\n", status)
	}
	return nil
}
