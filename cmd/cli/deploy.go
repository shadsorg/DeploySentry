package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// deployCmd is the parent command for deployment operations.
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Manage deployments across environments",
	Long: `Create, monitor, and control deployments across your environments.

The deploy command group supports canary, blue-green, and rolling
deployment strategies with built-in rollback, pause/resume, and
promotion capabilities.

Examples:
  # Create a canary deployment to production
  deploysentry deploy create --app api --version v1.2.0 --artifact ghcr.io/acme/api:v1.2.0 --strategy canary

  # Watch deployment status in real time
  deploysentry deploy status <deployment-id> --watch

  # Promote a canary deployment to the next phase
  deploysentry deploy promote <deployment-id>

  # Rollback a deployment
  deploysentry deploy rollback <deployment-id> --reason "elevated error rate"`,
}

var deployCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new deployment",
	Long: `Create a new deployment for an application to a target environment.

Required: --app, --version, --artifact, and an environment (via --env or
DS config). The deployment strategy defaults to "rolling" but can be set
to "canary", "blue-green", or any named rollout strategy from scope ancestry.

Flags:
  --app          Application slug or UUID (required)
  --version      Version string for the deployment (required)
  --artifact     Artifact reference (image tag, asset URL, etc.) (required)
  --env          Target environment slug or UUID (or set via 'env' config)
  --strategy     Deployment strategy: rolling, canary, blue-green, or a named rollout strategy (default: rolling)
  --commit-sha   Commit SHA associated with this deployment
  --mode         Deployment mode: orchestrate (default) or record
  --source       Optional free-form source label (e.g. "ci", "manual")
  --description  (deprecated; ignored — server has no description field)
  --apply-immediately  Skip rollout staging and apply immediately

Examples:
  # Rolling deployment
  deploysentry deploy create --app api --version v1.2.0 --artifact ghcr.io/acme/api:v1.2.0 --env staging

  # Canary
  deploysentry deploy create --app api --version v1.3.0 --artifact ghcr.io/acme/api:v1.3.0 --env production --strategy canary

  # Record-mode deploy (platform already shipped, just track it)
  deploysentry deploy create --app api --version v1.4.0 --artifact ghcr.io/acme/api:v1.4.0 --env production --mode record`,
	RunE: runDeployCreate,
}

var deployStatusCmd = &cobra.Command{
	Use:   "status <deployment-id>",
	Short: "Show deployment status",
	Long: `Display the current status of a deployment by UUID.

Use --watch to continuously poll for status updates.

Examples:
  deploysentry deploy status 7c0b8c80-...
  deploysentry deploy status 7c0b8c80-... --watch`,
	Args: cobra.ExactArgs(1),
	RunE: runDeployStatus,
}

var deployPromoteCmd = &cobra.Command{
	Use:   "promote <deployment-id>",
	Short: "Promote a canary deployment to the next phase",
	Long: `Promote a canary deployment to the next traffic percentage phase.

Examples:
  deploysentry deploy promote 7c0b8c80-...`,
	Args: cobra.ExactArgs(1),
	RunE: runDeployPromote,
}

var deployRollbackCmd = &cobra.Command{
	Use:   "rollback <deployment-id>",
	Short: "Trigger a rollback of a deployment",
	Long: `Immediately roll back a deployment. The --reason flag is required.

Examples:
  deploysentry deploy rollback 7c0b8c80-... --reason "elevated error rates"`,
	Args: cobra.ExactArgs(1),
	RunE: runDeployRollback,
}

var deployPauseCmd = &cobra.Command{
	Use:   "pause <deployment-id>",
	Short: "Pause an in-progress deployment",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeployPause,
}

var deployResumeCmd = &cobra.Command{
	Use:   "resume <deployment-id>",
	Short: "Resume a paused deployment",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeployResume,
}

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments for an application",
	Long: `List deployments for the given application, optionally filtered by
environment.

The server's list endpoint is application-scoped and requires --app.

Examples:
  deploysentry deploy list --app api
  deploysentry deploy list --app api --env production
  deploysentry deploy list --app api -o json`,
	RunE: runDeployList,
}

func init() {
	// deploy create flags
	deployCreateCmd.Flags().String("app", "", "application slug or UUID (required)")
	deployCreateCmd.Flags().String("version", "", "version string for the deployment (required)")
	deployCreateCmd.Flags().String("artifact", "", "artifact reference (image tag, etc.) (required)")
	deployCreateCmd.Flags().String("env", "", "target environment (or use 'env' config)")
	deployCreateCmd.Flags().String("strategy", "rolling", "deployment strategy: rolling, canary, blue-green, or a named rollout strategy")
	deployCreateCmd.Flags().String("commit-sha", "", "commit SHA associated with this deployment")
	deployCreateCmd.Flags().String("mode", "", "deployment mode: orchestrate (default) or record")
	deployCreateCmd.Flags().String("source", "", "optional source label (e.g. ci, manual)")
	deployCreateCmd.Flags().Bool("apply-immediately", false, "skip rollout staging; apply immediately")
	_ = deployCreateCmd.MarkFlagRequired("app")
	_ = deployCreateCmd.MarkFlagRequired("version")
	_ = deployCreateCmd.MarkFlagRequired("artifact")

	// deploy status flags
	deployStatusCmd.Flags().Bool("watch", false, "continuously watch deployment status")
	deployStatusCmd.Flags().Duration("interval", 5*time.Second, "polling interval for --watch")

	// deploy rollback flags
	deployRollbackCmd.Flags().String("reason", "", "reason for the rollback (required by API)")
	_ = deployRollbackCmd.MarkFlagRequired("reason")

	// deploy list flags
	deployListCmd.Flags().String("app", "", "application slug or UUID (required)")
	deployListCmd.Flags().String("env", "", "filter by environment slug or UUID")
	deployListCmd.Flags().Int("limit", 20, "maximum number of results")
	_ = deployListCmd.MarkFlagRequired("app")

	deployCmd.AddCommand(deployCreateCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.AddCommand(deployPromoteCmd)
	deployCmd.AddCommand(deployRollbackCmd)
	deployCmd.AddCommand(deployPauseCmd)
	deployCmd.AddCommand(deployResumeCmd)
	deployCmd.AddCommand(deployListCmd)

	rootCmd.AddCommand(deployCmd)
}

func runDeployCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}

	appSlug, _ := cmd.Flags().GetString("app")
	version, _ := cmd.Flags().GetString("version")
	artifact, _ := cmd.Flags().GetString("artifact")
	envInput, _ := cmd.Flags().GetString("env")
	if envInput == "" {
		envInput = getEnv()
	}
	if envInput == "" {
		return fmt.Errorf("environment is required; set via --env flag or config")
	}
	strategy, _ := cmd.Flags().GetString("strategy")
	commitSHA, _ := cmd.Flags().GetString("commit-sha")
	mode, _ := cmd.Flags().GetString("mode")
	source, _ := cmd.Flags().GetString("source")

	// Validate built-in strategy types; named rollout strategies from scope ancestry are passed through.
	builtinStrategies := map[string]bool{"rolling": true, "canary": true, "blue-green": true}
	isNamedStrategy := !builtinStrategies[strategy]

	applyImmediately, _ := cmd.Flags().GetBool("apply-immediately")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	// Resolve app slug -> UUID and env slug -> UUID.
	appID, err := resolveAppID(client, org, projectSlug, appSlug)
	if err != nil {
		return err
	}
	envID, err := resolveEnvID(client, org, envInput)
	if err != nil {
		return err
	}

	deployStrategy := strategy
	if isNamedStrategy {
		deployStrategy = "rolling" // server default; rollout sub-map carries the named strategy
	}

	body := map[string]interface{}{
		"application_id": appID,
		"environment_id": envID,
		"version":        version,
		"artifact":       artifact,
		"strategy":       deployStrategy,
	}
	if commitSHA != "" {
		body["commit_sha"] = commitSHA
	}
	if mode != "" {
		body["mode"] = mode
	}
	if source != "" {
		body["source"] = source
	}

	// Attach rollout sub-map when a named strategy or apply-immediately is requested.
	if isNamedStrategy || applyImmediately {
		rollout := map[string]any{}
		if isNamedStrategy {
			rollout["strategy_name"] = strategy
		}
		if applyImmediately {
			rollout["apply_immediately"] = true
		}
		body["rollout"] = rollout
	}

	resp, err := client.post("/api/v1/deployments", body)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	status, _ := resp["status"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deployment created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Application: %s\n", appSlug)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Version:     %s\n", version)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Environment: %s\n", envInput)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Strategy:    %s\n", strategy)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status:      %s\n", status)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'deploysentry deploy status %s --watch' to monitor progress.\n", id)
	return nil
}

func runDeployStatus(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	watch, _ := cmd.Flags().GetBool("watch")
	interval, _ := cmd.Flags().GetDuration("interval")
	deploymentID := args[0]

	printStatus := func() error {
		path := fmt.Sprintf("/api/v1/deployments/%s", deploymentID)
		resp, err := client.get(path)
		if err != nil {
			return fmt.Errorf("failed to get deployment status: %w", err)
		}

		if getOutputFormat() == "json" {
			data, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		id, _ := resp["id"].(string)
		status, _ := resp["status"].(string)
		version, _ := resp["version"].(string)
		appID, _ := resp["application_id"].(string)
		envID, _ := resp["environment_id"].(string)
		strategy, _ := resp["strategy"].(string)
		traffic, _ := resp["traffic_percent"].(float64)
		createdAt, _ := resp["created_at"].(string)

		if watch {
			// Clear screen for watch mode.
			_, _ = fmt.Fprint(cmd.OutOrStdout(), "\033[2J\033[H")
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deployment Status\n")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:             %s\n", id)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Version:        %s\n", version)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Application:    %s\n", appID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Environment:    %s\n", envID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Strategy:       %s\n", strategy)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status:         %s\n", status)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Traffic:        %.0f%%\n", traffic)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Created:        %s\n", createdAt)

		// Print traffic bar.
		barWidth := 40
		filled := int(traffic / 100.0 * float64(barWidth))
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("=", filled) + strings.Repeat("-", barWidth-filled)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n  [%s] %.0f%%\n", bar, traffic)

		return nil
	}

	if !watch {
		return printStatus()
	}

	// Watch mode: continuously poll.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := printStatus(); err != nil {
		return err
	}
	for {
		select {
		case <-ticker.C:
			if err := printStatus(); err != nil {
				return err
			}
		case <-sigCh:
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nStopped watching.")
			return nil
		}
	}
}

func runDeployPromote(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	deploymentID := args[0]

	path := fmt.Sprintf("/api/v1/deployments/%s/promote", deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to promote deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	status, _ := resp["status"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deployment promoted successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", status)
	return nil
}

func runDeployRollback(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	deploymentID := args[0]
	reason, _ := cmd.Flags().GetString("reason")

	body := map[string]interface{}{"reason": reason}

	path := fmt.Sprintf("/api/v1/deployments/%s/rollback", deploymentID)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to rollback deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rollback initiated successfully.\n")
	if depID, ok := resp["deployment_id"].(string); ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Deployment ID: %s\n", depID)
	}
	if status, ok := resp["status"].(string); ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Status:        %s\n", status)
	}
	return nil
}

func runDeployPause(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	deploymentID := args[0]

	path := fmt.Sprintf("/api/v1/deployments/%s/pause", deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to pause deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deployment paused.\n")
	return nil
}

func runDeployResume(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	deploymentID := args[0]

	path := fmt.Sprintf("/api/v1/deployments/%s/resume", deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to resume deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deployment resumed.\n")
	return nil
}

func runDeployList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	appSlug, _ := cmd.Flags().GetString("app")
	appID, err := resolveAppID(client, org, projectSlug, appSlug)
	if err != nil {
		return err
	}

	params := []string{"app_id=" + appID}
	if envInput, _ := cmd.Flags().GetString("env"); envInput != "" {
		envID, err := resolveEnvID(client, org, envInput)
		if err != nil {
			return err
		}
		params = append(params, "environment_id="+envID)
	} else if e := getEnv(); e != "" {
		envID, err := resolveEnvID(client, org, e)
		if err != nil {
			return err
		}
		params = append(params, "environment_id="+envID)
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}

	path := "/api/v1/deployments?" + strings.Join(params, "&")
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	deployments, _ := resp["deployments"].([]interface{})
	if len(deployments) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No deployments found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tVERSION\tSTRATEGY\tSTATUS\tTRAFFIC\tCREATED")
	for _, d := range deployments {
		dep, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := dep["id"].(string)
		version, _ := dep["version"].(string)
		strategy, _ := dep["strategy"].(string)
		status, _ := dep["status"].(string)
		traffic, _ := dep["traffic_percent"].(float64)
		createdAt, _ := dep["created_at"].(string)

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f%%\t%s\n",
			id, version, strategy, status, traffic, createdAt)
	}
	return w.Flush()
}

// clientFromConfig creates an API client from the current configuration.
// It automatically refreshes expired OAuth tokens before returning the client.
func clientFromConfig() (*apiClient, error) {
	apiURL := viper.GetString("api_url")
	if apiURL == "" {
		apiURL = "https://api.dr-sentry.com"
	}
	client := newAPIClient(apiURL)

	// Prefer credentials written by `deploysentry auth login`.
	creds, err := ensureValidToken()
	if err == nil {
		if creds.TokenType == tokenTypeAPIKey {
			client.setAPIKey(creds.AccessToken)
		} else {
			client.setToken(creds.AccessToken)
		}
	} else {
		// Fall back to an ambient API key in config/env.
		apiKey := viper.GetString("api_key")
		if apiKey != "" {
			client.setAPIKey(apiKey)
		}
	}

	return client, nil
}
