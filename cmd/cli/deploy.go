package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
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
  deploysentry deploy create --release v1.2.0 --env production --strategy canary

  # Watch deployment status in real time
  deploysentry deploy status --watch

  # Promote a canary deployment to the next phase
  deploysentry deploy promote

  # Rollback a failed deployment
  deploysentry deploy rollback`,
}

var deployCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new deployment",
	Long: `Create a new deployment for a release to a target environment.

You must specify the release version and target environment. The
deployment strategy defaults to "rolling" but can be set to "canary"
or "blue-green".

Flags:
  --release     The release version to deploy (required)
  --env         Target environment (required)
  --strategy    Deployment strategy: rolling, canary, blue-green (default: rolling)
  --description Optional description for the deployment

Examples:
  # Create a rolling deployment
  deploysentry deploy create --release v1.2.0 --env staging

  # Create a canary deployment with description
  deploysentry deploy create --release v1.3.0 --env production --strategy canary \
    --description "Gradual rollout of new checkout flow"

  # Create a blue-green deployment
  deploysentry deploy create --release v2.0.0 --env production --strategy blue-green`,
	RunE: runDeployCreate,
}

var deployStatusCmd = &cobra.Command{
	Use:   "status [deployment-id]",
	Short: "Show deployment status",
	Long: `Display the current status of a deployment.

If no deployment ID is specified, the most recent deployment for the
current project and environment is shown.

Use --watch to continuously poll for status updates.

Examples:
  # Show status of the latest deployment
  deploysentry deploy status

  # Show status of a specific deployment
  deploysentry deploy status deploy_abc123

  # Watch deployment progress in real time
  deploysentry deploy status --watch

  # Watch a specific deployment
  deploysentry deploy status deploy_abc123 --watch`,
	RunE: runDeployStatus,
}

var deployPromoteCmd = &cobra.Command{
	Use:   "promote [deployment-id]",
	Short: "Promote a canary deployment to the next phase",
	Long: `Promote a canary deployment to the next traffic percentage phase.

This advances the deployment to the next configured canary step,
increasing traffic to the new version. If no deployment ID is given,
the current active canary deployment is promoted.

Examples:
  # Promote the current canary deployment
  deploysentry deploy promote

  # Promote a specific deployment
  deploysentry deploy promote deploy_abc123`,
	RunE: runDeployPromote,
}

var deployRollbackCmd = &cobra.Command{
	Use:   "rollback [deployment-id]",
	Short: "Trigger a rollback of a deployment",
	Long: `Immediately roll back a deployment to the previous stable version.

This command triggers an automated rollback, shifting all traffic
back to the previous version. If no deployment ID is specified, the
most recent active deployment is rolled back.

Examples:
  # Rollback the latest deployment
  deploysentry deploy rollback

  # Rollback a specific deployment
  deploysentry deploy rollback deploy_abc123

  # Rollback with a reason
  deploysentry deploy rollback --reason "Elevated error rates detected"`,
	RunE: runDeployRollback,
}

var deployPauseCmd = &cobra.Command{
	Use:   "pause [deployment-id]",
	Short: "Pause an in-progress deployment",
	Long: `Pause an in-progress deployment, freezing it at the current state.

This is useful when you need to investigate metrics before proceeding
with a canary promotion. The deployment can be resumed later.

Examples:
  # Pause the current deployment
  deploysentry deploy pause

  # Pause a specific deployment
  deploysentry deploy pause deploy_abc123`,
	RunE: runDeployPause,
}

var deployResumeCmd = &cobra.Command{
	Use:   "resume [deployment-id]",
	Short: "Resume a paused deployment",
	Long: `Resume a previously paused deployment.

The deployment will continue from where it was paused, proceeding
with the next canary phase or completing the rollout.

Examples:
  # Resume the current paused deployment
  deploysentry deploy resume

  # Resume a specific deployment
  deploysentry deploy resume deploy_abc123`,
	RunE: runDeployResume,
}

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	Long: `List deployments for the current project, optionally filtered by
environment and status.

Results are sorted by creation time, most recent first.

Examples:
  # List all deployments
  deploysentry deploy list

  # List deployments for a specific environment
  deploysentry deploy list --env production

  # List only failed deployments
  deploysentry deploy list --status failed

  # List deployments in JSON format
  deploysentry deploy list -o json`,
	RunE: runDeployList,
}

var deployLogsCmd = &cobra.Command{
	Use:   "logs [deployment-id]",
	Short: "Stream deployment logs",
	Long: `Stream real-time logs for a deployment.

Logs include deployment events, health check results, traffic
shifting updates, and rollback notifications.

Use Ctrl+C to stop streaming.

Examples:
  # Stream logs for the latest deployment
  deploysentry deploy logs

  # Stream logs for a specific deployment
  deploysentry deploy logs deploy_abc123

  # Show only the last 50 log lines
  deploysentry deploy logs --tail 50`,
	RunE: runDeployLogs,
}

func init() {
	// deploy create flags
	deployCreateCmd.Flags().String("release", "", "release version to deploy (required)")
	deployCreateCmd.Flags().String("env", "", "target environment (required)")
	deployCreateCmd.Flags().String("strategy", "rolling", "deployment strategy: rolling, canary, blue-green")
	deployCreateCmd.Flags().String("description", "", "optional description for the deployment")
	_ = deployCreateCmd.MarkFlagRequired("release")

	// deploy status flags
	deployStatusCmd.Flags().Bool("watch", false, "continuously watch deployment status")
	deployStatusCmd.Flags().Duration("interval", 5*time.Second, "polling interval for --watch")

	// deploy rollback flags
	deployRollbackCmd.Flags().String("reason", "", "reason for the rollback")

	// deploy list flags
	deployListCmd.Flags().String("env", "", "filter by environment")
	deployListCmd.Flags().String("status", "", "filter by status (pending, running, completed, failed, rolled_back)")
	deployListCmd.Flags().Int("limit", 20, "maximum number of results")

	// deploy logs flags
	deployLogsCmd.Flags().Int("tail", 0, "number of recent log lines to show (0 for all)")

	deployCmd.AddCommand(deployCreateCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.AddCommand(deployPromoteCmd)
	deployCmd.AddCommand(deployRollbackCmd)
	deployCmd.AddCommand(deployPauseCmd)
	deployCmd.AddCommand(deployResumeCmd)
	deployCmd.AddCommand(deployListCmd)
	deployCmd.AddCommand(deployLogsCmd)

	rootCmd.AddCommand(deployCmd)
}

func runDeployCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	release, _ := cmd.Flags().GetString("release")
	env, _ := cmd.Flags().GetString("env")
	if env == "" {
		env = getEnv()
	}
	if env == "" {
		return fmt.Errorf("environment is required; set via --env flag or config")
	}
	strategy, _ := cmd.Flags().GetString("strategy")
	description, _ := cmd.Flags().GetString("description")

	// Validate strategy.
	validStrategies := map[string]bool{"rolling": true, "canary": true, "blue-green": true}
	if !validStrategies[strategy] {
		return fmt.Errorf("invalid strategy %q; must be one of: rolling, canary, blue-green", strategy)
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"release":     release,
		"environment": env,
		"strategy":    strategy,
	}
	if description != "" {
		body["description"] = description
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments", org, project)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	status, _ := resp["status"].(string)
	fmt.Fprintf(cmd.OutOrStdout(), "Deployment created successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", id)
	fmt.Fprintf(cmd.OutOrStdout(), "  Release:     %s\n", release)
	fmt.Fprintf(cmd.OutOrStdout(), "  Environment: %s\n", env)
	fmt.Fprintf(cmd.OutOrStdout(), "  Strategy:    %s\n", strategy)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:      %s\n", status)
	fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'deploysentry deploy status %s --watch' to monitor progress.\n", id)
	return nil
}

func runDeployStatus(cmd *cobra.Command, args []string) error {
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

	watch, _ := cmd.Flags().GetBool("watch")
	interval, _ := cmd.Flags().GetDuration("interval")

	var deploymentID string
	if len(args) > 0 {
		deploymentID = args[0]
	}

	printStatus := func() error {
		var path string
		if deploymentID != "" {
			path = fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s", org, project, deploymentID)
		} else {
			path = fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/latest", org, project)
			env := getEnv()
			if env != "" {
				path += "?environment=" + env
			}
		}

		resp, err := client.get(path)
		if err != nil {
			return fmt.Errorf("failed to get deployment status: %w", err)
		}

		if getOutputFormat() == "json" {
			data, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		id, _ := resp["id"].(string)
		status, _ := resp["status"].(string)
		release, _ := resp["release"].(string)
		env, _ := resp["environment"].(string)
		strategy, _ := resp["strategy"].(string)
		progress, _ := resp["progress"].(float64)
		createdAt, _ := resp["created_at"].(string)

		if watch {
			// Clear screen for watch mode.
			fmt.Fprint(cmd.OutOrStdout(), "\033[2J\033[H")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deployment Status\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", id)
		fmt.Fprintf(cmd.OutOrStdout(), "  Release:     %s\n", release)
		fmt.Fprintf(cmd.OutOrStdout(), "  Environment: %s\n", env)
		fmt.Fprintf(cmd.OutOrStdout(), "  Strategy:    %s\n", strategy)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status:      %s\n", status)
		fmt.Fprintf(cmd.OutOrStdout(), "  Progress:    %.0f%%\n", progress)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:     %s\n", createdAt)

		// Print progress bar.
		barWidth := 40
		filled := int(progress / 100.0 * float64(barWidth))
		bar := strings.Repeat("=", filled) + strings.Repeat("-", barWidth-filled)
		fmt.Fprintf(cmd.OutOrStdout(), "\n  [%s] %.0f%%\n", bar, progress)

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

	// Print immediately, then on each tick.
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
			fmt.Fprintln(cmd.OutOrStdout(), "\nStopped watching.")
			return nil
		}
	}
}

func runDeployPromote(cmd *cobra.Command, args []string) error {
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

	deploymentID := "latest"
	if len(args) > 0 {
		deploymentID = args[0]
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s/promote", org, project, deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to promote deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	status, _ := resp["status"].(string)
	progress, _ := resp["progress"].(float64)
	fmt.Fprintf(cmd.OutOrStdout(), "Deployment promoted successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:   %s\n", status)
	fmt.Fprintf(cmd.OutOrStdout(), "  Progress: %.0f%%\n", progress)
	return nil
}

func runDeployRollback(cmd *cobra.Command, args []string) error {
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

	deploymentID := "latest"
	if len(args) > 0 {
		deploymentID = args[0]
	}

	body := map[string]interface{}{}
	reason, _ := cmd.Flags().GetString("reason")
	if reason != "" {
		body["reason"] = reason
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s/rollback", org, project, deploymentID)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to rollback deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rollback initiated successfully.\n")
	if id, ok := resp["id"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Deployment ID: %s\n", id)
	}
	if status, ok := resp["status"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Status:        %s\n", status)
	}
	return nil
}

func runDeployPause(cmd *cobra.Command, args []string) error {
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

	deploymentID := "latest"
	if len(args) > 0 {
		deploymentID = args[0]
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s/pause", org, project, deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to pause deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deployment paused.\n")
	return nil
}

func runDeployResume(cmd *cobra.Command, args []string) error {
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

	deploymentID := "latest"
	if len(args) > 0 {
		deploymentID = args[0]
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s/resume", org, project, deploymentID)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to resume deployment: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deployment resumed.\n")
	return nil
}

func runDeployList(cmd *cobra.Command, args []string) error {
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

	// Build query parameters.
	params := []string{}
	if env, _ := cmd.Flags().GetString("env"); env != "" {
		params = append(params, "environment="+env)
	} else if e := getEnv(); e != "" {
		params = append(params, "environment="+e)
	}
	if status, _ := cmd.Flags().GetString("status"); status != "" {
		params = append(params, "status="+status)
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments", org, project)
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	deployments, _ := resp["deployments"].([]interface{})
	if len(deployments) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No deployments found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tRELEASE\tENVIRONMENT\tSTRATEGY\tSTATUS\tPROGRESS\tCREATED")
	for _, d := range deployments {
		dep, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := dep["id"].(string)
		release, _ := dep["release"].(string)
		env, _ := dep["environment"].(string)
		strategy, _ := dep["strategy"].(string)
		status, _ := dep["status"].(string)
		progress, _ := dep["progress"].(float64)
		createdAt, _ := dep["created_at"].(string)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.0f%%\t%s\n",
			id, release, env, strategy, status, progress, createdAt)
	}
	return w.Flush()
}

func runDeployLogs(cmd *cobra.Command, args []string) error {
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

	deploymentID := "latest"
	if len(args) > 0 {
		deploymentID = args[0]
	}
	tail, _ := cmd.Flags().GetInt("tail")

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/deployments/%s/logs", org, project, deploymentID)
	if tail > 0 {
		path += fmt.Sprintf("?tail=%d", tail)
	}

	// Stream logs via SSE (Server-Sent Events).
	req, err := client.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to create log stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp2, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to log stream: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("log stream returned status %d", resp2.StatusCode)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Streaming deployment logs (Ctrl+C to stop)...\n\n")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	scanner := bufio.NewScanner(resp2.Body)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		for scanner.Scan() {
			line := scanner.Text()
			// Parse SSE data lines.
			if strings.HasPrefix(line, "data: ") {
				fmt.Fprintln(cmd.OutOrStdout(), line[6:])
			}
		}
	}()

	select {
	case <-doneCh:
		fmt.Fprintln(cmd.OutOrStdout(), "\nLog stream ended.")
	case <-sigCh:
		fmt.Fprintln(cmd.OutOrStdout(), "\nStopped streaming logs.")
	}
	return nil
}

// clientFromConfig creates an API client from the current configuration.
// It automatically refreshes expired OAuth tokens before returning the client.
func clientFromConfig() (*apiClient, error) {
	apiURL := viper.GetString("api_url")
	if apiURL == "" {
		apiURL = "https://api.deploysentry.io"
	}
	client := newAPIClient(apiURL)

	// Try loading credentials with automatic token refresh on expiry.
	creds, err := ensureValidToken()
	if err == nil {
		client.setToken(creds.AccessToken)
	} else {
		// Check for API key in config or environment.
		apiKey := viper.GetString("api_key")
		if apiKey != "" {
			client.setAPIKey(apiKey)
		}
	}

	return client, nil
}
