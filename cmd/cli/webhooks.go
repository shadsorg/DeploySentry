package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// webhooksCmd is the parent command for webhook operations.
var webhooksCmd = &cobra.Command{
	Use:     "webhooks",
	Aliases: []string{"webhook", "wh"},
	Short:   "Manage webhooks for event notifications",
	Long: `Create and manage webhook endpoints that receive real-time notifications
when events occur in your DeploySentry project.

Webhooks allow you to integrate with external systems by sending HTTP requests
when flags are toggled, deployments complete, or other events happen.

Examples:
  # Create a webhook for deployment events
  deploysentry webhooks create --name "Slack Notifications" \
    --url "https://hooks.slack.com/services/..." \
    --events "deployment.completed,deployment.failed"

  # List all webhooks
  deploysentry webhooks list

  # Test a webhook
  deploysentry webhooks test abc123 --event "deployment.completed"

  # View webhook delivery history
  deploysentry webhooks deliveries abc123`,
}

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook endpoint",
	Long: `Create a new webhook endpoint that will receive HTTP requests when
specified events occur in your project.

The webhook will receive a JSON payload with event details and will be
signed with HMAC-SHA256 for security verification.

Examples:
  # Create a webhook for all deployment events
  deploysentry webhooks create --name "Deploy Notifications" \
    --url "https://api.example.com/webhooks/deploys" \
    --events "deployment.completed,deployment.failed"

  # Create a webhook for flag changes only
  deploysentry webhooks create --name "Flag Monitor" \
    --url "https://monitor.example.com/flags" \
    --events "flag.created,flag.toggled,flag.archived"

  # Create with custom retry and timeout settings
  deploysentry webhooks create --name "Critical Alerts" \
    --url "https://alerts.example.com/webhook" \
    --events "deployment.failed,system.alert" \
    --retry-attempts 5 --timeout 30`,
	RunE: runWebhooksCreate,
}

var webhooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhook endpoints",
	Long: `List all webhook endpoints configured for your organization.
Optionally filter by project, status, or event types.

Examples:
  # List all webhooks
  deploysentry webhooks list

  # List webhooks for a specific project
  deploysentry webhooks list --project my-project

  # List only active webhooks
  deploysentry webhooks list --active

  # List webhooks that listen for deployment events
  deploysentry webhooks list --events deployment.completed

  # Output as JSON
  deploysentry webhooks list -o json`,
	RunE: runWebhooksList,
}

var webhooksGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get webhook endpoint details",
	Long: `Display detailed information about a specific webhook endpoint
including its configuration, recent deliveries, and statistics.

Examples:
  # Get webhook details
  deploysentry webhooks get abc123

  # Get webhook details in JSON format
  deploysentry webhooks get abc123 -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhooksGet,
}

var webhooksUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a webhook endpoint",
	Long: `Update the configuration of an existing webhook endpoint.
You can modify the URL, events, name, or other settings.

Examples:
  # Update webhook URL
  deploysentry webhooks update abc123 --url "https://new-url.example.com/webhook"

  # Add more events to an existing webhook
  deploysentry webhooks update abc123 --events "flag.created,flag.toggled,deployment.completed"

  # Disable a webhook
  deploysentry webhooks update abc123 --disable

  # Change retry settings
  deploysentry webhooks update abc123 --retry-attempts 5 --timeout 20`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhooksUpdate,
}

var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a webhook endpoint",
	Long: `Delete a webhook endpoint. This will stop all future deliveries
to this endpoint and cannot be undone.

Examples:
  # Delete a webhook
  deploysentry webhooks delete abc123

  # Delete with confirmation prompt
  deploysentry webhooks delete abc123 --confirm`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhooksDelete,
}

var webhooksTestCmd = &cobra.Command{
	Use:   "test <id>",
	Short: "Send a test payload to a webhook",
	Long: `Send a test event payload to a webhook endpoint to verify
it's working correctly. This will create a real delivery attempt
but mark it as a test.

Examples:
  # Test webhook with deployment completed event
  deploysentry webhooks test abc123 --event "deployment.completed"

  # Test with custom payload
  deploysentry webhooks test abc123 --event "flag.toggled" \
    --payload '{"flag_key": "test-flag", "enabled": true}'

  # Test and show full response
  deploysentry webhooks test abc123 --event "system.alert" --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhooksTest,
}

var webhooksDeliveriesCmd = &cobra.Command{
	Use:     "deliveries <webhook-id>",
	Aliases: []string{"delivery", "logs"},
	Short:   "View webhook delivery history",
	Long: `View the delivery history for a webhook, including successful
deliveries, failures, and retry attempts.

Examples:
  # View recent deliveries
  deploysentry webhooks deliveries abc123

  # View only failed deliveries
  deploysentry webhooks deliveries abc123 --status failed

  # View deliveries for specific event type
  deploysentry webhooks deliveries abc123 --event-type "deployment.completed"

  # View detailed delivery information
  deploysentry webhooks deliveries abc123 --detailed`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhooksDeliveries,
}

var webhooksEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "List available webhook events",
	Long: `List all available webhook event types that you can subscribe to.
Each event type represents a different action or occurrence in DeploySentry.

Examples:
  # List all available events
  deploysentry webhooks events

  # Filter events by category
  deploysentry webhooks events --category deployment

  # Show event descriptions
  deploysentry webhooks events --detailed`,
	RunE: runWebhooksEvents,
}

func init() {
	// webhooks create flags
	webhooksCreateCmd.Flags().String("name", "", "human-readable name for the webhook")
	webhooksCreateCmd.Flags().String("url", "", "webhook endpoint URL")
	webhooksCreateCmd.Flags().StringSlice("events", nil, "comma-separated list of events to listen for")
	webhooksCreateCmd.Flags().Int("retry-attempts", 3, "number of retry attempts for failed deliveries")
	webhooksCreateCmd.Flags().Int("timeout", 10, "HTTP timeout in seconds")
	_ = webhooksCreateCmd.MarkFlagRequired("name")
	_ = webhooksCreateCmd.MarkFlagRequired("url")
	_ = webhooksCreateCmd.MarkFlagRequired("events")

	// webhooks list flags
	webhooksListCmd.Flags().String("project", "", "filter by project")
	webhooksListCmd.Flags().Bool("active", false, "show only active webhooks")
	webhooksListCmd.Flags().Bool("inactive", false, "show only inactive webhooks")
	webhooksListCmd.Flags().StringSlice("events", nil, "filter by event types")
	webhooksListCmd.Flags().Int("limit", 50, "maximum number of results")

	// webhooks update flags
	webhooksUpdateCmd.Flags().String("name", "", "update webhook name")
	webhooksUpdateCmd.Flags().String("url", "", "update webhook URL")
	webhooksUpdateCmd.Flags().StringSlice("events", nil, "update events list")
	webhooksUpdateCmd.Flags().Bool("enable", false, "enable the webhook")
	webhooksUpdateCmd.Flags().Bool("disable", false, "disable the webhook")
	webhooksUpdateCmd.Flags().Int("retry-attempts", -1, "update retry attempts")
	webhooksUpdateCmd.Flags().Int("timeout", -1, "update timeout in seconds")

	// webhooks delete flags
	webhooksDeleteCmd.Flags().Bool("confirm", false, "skip confirmation prompt")

	// webhooks test flags
	webhooksTestCmd.Flags().String("event", "", "event type to test")
	webhooksTestCmd.Flags().String("payload", "{}", "custom JSON payload")
	_ = webhooksTestCmd.MarkFlagRequired("event")

	// webhooks deliveries flags
	webhooksDeliveriesCmd.Flags().String("status", "", "filter by delivery status (pending, sent, failed, cancelled)")
	webhooksDeliveriesCmd.Flags().String("event-type", "", "filter by event type")
	webhooksDeliveriesCmd.Flags().Bool("detailed", false, "show detailed delivery information")
	webhooksDeliveriesCmd.Flags().Int("limit", 50, "maximum number of results")

	// webhooks events flags
	webhooksEventsCmd.Flags().String("category", "", "filter by event category (flag, deployment, release, system)")
	webhooksEventsCmd.Flags().Bool("detailed", false, "show event descriptions")

	// Build command tree
	webhooksCmd.AddCommand(webhooksCreateCmd)
	webhooksCmd.AddCommand(webhooksListCmd)
	webhooksCmd.AddCommand(webhooksGetCmd)
	webhooksCmd.AddCommand(webhooksUpdateCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
	webhooksCmd.AddCommand(webhooksTestCmd)
	webhooksCmd.AddCommand(webhooksDeliveriesCmd)
	webhooksCmd.AddCommand(webhooksEventsCmd)

	rootCmd.AddCommand(webhooksCmd)
}

func runWebhooksCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project := getProject() // Optional for webhooks

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	events, _ := cmd.Flags().GetStringSlice("events")
	retryAttempts, _ := cmd.Flags().GetInt("retry-attempts")
	timeout, _ := cmd.Flags().GetInt("timeout")

	body := map[string]interface{}{
		"name":            name,
		"url":             url,
		"events":          events,
		"retry_attempts":  retryAttempts,
		"timeout_seconds": timeout,
	}

	if project != "" {
		body["project_id"] = project
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks", org)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	webhook, _ := resp["webhook"].(map[string]interface{})
	fmt.Fprintf(cmd.OutOrStdout(), "Webhook created successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", webhook["id"])
	fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", webhook["name"])
	fmt.Fprintf(cmd.OutOrStdout(), "  URL:    %s\n", webhook["url"])
	fmt.Fprintf(cmd.OutOrStdout(), "  Events: %s\n", strings.Join(events, ", "))
	return nil
}

func runWebhooksList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	params := []string{}
	if project, _ := cmd.Flags().GetString("project"); project != "" {
		params = append(params, "project_id="+project)
	}
	if active, _ := cmd.Flags().GetBool("active"); active {
		params = append(params, "is_active=true")
	}
	if inactive, _ := cmd.Flags().GetBool("inactive"); inactive {
		params = append(params, "is_active=false")
	}
	if events, _ := cmd.Flags().GetStringSlice("events"); len(events) > 0 {
		for _, event := range events {
			params = append(params, "events="+event)
		}
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks", org)
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	webhooks, _ := resp["webhooks"].([]interface{})
	if len(webhooks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No webhooks found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tURL\tSTATUS\tEVENTS\tCREATED")
	for _, wh := range webhooks {
		webhook, ok := wh.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := webhook["id"].(string)
		name, _ := webhook["name"].(string)
		url, _ := webhook["url"].(string)
		isActive, _ := webhook["is_active"].(bool)
		createdAt, _ := webhook["created_at"].(string)

		status := "inactive"
		if isActive {
			status = "active"
		}

		events := ""
		if eventList, ok := webhook["events"].([]interface{}); ok {
			eventStrs := make([]string, 0, len(eventList))
			for _, e := range eventList {
				if s, ok := e.(string); ok {
					eventStrs = append(eventStrs, s)
				}
			}
			events = strings.Join(eventStrs, ",")
			if len(events) > 30 {
				events = events[:27] + "..."
			}
		}

		// Truncate URL for display
		displayURL := url
		if len(displayURL) > 40 {
			displayURL = displayURL[:37] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			id[:8], name, displayURL, status, events, createdAt)
	}
	return w.Flush()
}

func runWebhooksGet(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	webhookID := args[0]
	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks/%s", org, webhookID)

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	webhook, _ := resp["webhook"].(map[string]interface{})
	fmt.Fprintf(cmd.OutOrStdout(), "Webhook Details:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:              %s\n", webhook["id"])
	fmt.Fprintf(cmd.OutOrStdout(), "  Name:            %s\n", webhook["name"])
	fmt.Fprintf(cmd.OutOrStdout(), "  URL:             %s\n", webhook["url"])

	isActive, _ := webhook["is_active"].(bool)
	status := "inactive"
	if isActive {
		status = "active"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:          %s\n", status)

	if retryAttempts, ok := webhook["retry_attempts"].(float64); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Retry Attempts:  %.0f\n", retryAttempts)
	}
	if timeout, ok := webhook["timeout_seconds"].(float64); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Timeout:         %.0fs\n", timeout)
	}

	if eventList, ok := webhook["events"].([]interface{}); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Events:\n")
		for _, e := range eventList {
			if s, ok := e.(string); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", s)
			}
		}
	}

	if createdAt, ok := webhook["created_at"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:         %s\n", createdAt)
	}
	if updatedAt, ok := webhook["updated_at"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Updated:         %s\n", updatedAt)
	}

	return nil
}

func runWebhooksUpdate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	webhookID := args[0]
	body := map[string]interface{}{}

	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		body["name"] = name
	}
	if cmd.Flags().Changed("url") {
		url, _ := cmd.Flags().GetString("url")
		body["url"] = url
	}
	if cmd.Flags().Changed("events") {
		events, _ := cmd.Flags().GetStringSlice("events")
		body["events"] = events
	}
	if cmd.Flags().Changed("enable") {
		body["is_active"] = true
	}
	if cmd.Flags().Changed("disable") {
		body["is_active"] = false
	}
	if cmd.Flags().Changed("retry-attempts") {
		retryAttempts, _ := cmd.Flags().GetInt("retry-attempts")
		if retryAttempts >= 0 {
			body["retry_attempts"] = retryAttempts
		}
	}
	if cmd.Flags().Changed("timeout") {
		timeout, _ := cmd.Flags().GetInt("timeout")
		if timeout > 0 {
			body["timeout_seconds"] = timeout
		}
	}

	if len(body) == 0 {
		return fmt.Errorf("no updates specified")
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks/%s", org, webhookID)
	resp, err := client.put(path, body)
	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Webhook updated successfully.\n")
	return nil
}

func runWebhooksDelete(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	webhookID := args[0]

	confirm, _ := cmd.Flags().GetBool("confirm")
	if !confirm {
		fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to delete webhook %s? This cannot be undone.\n", webhookID)
		fmt.Fprint(cmd.OutOrStdout(), "Type 'yes' to confirm: ")
		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
			return nil
		}
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks/%s", org, webhookID)
	_, err = client.delete(path)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Webhook deleted successfully.\n")
	return nil
}

func runWebhooksTest(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	webhookID := args[0]
	eventType, _ := cmd.Flags().GetString("event")
	payloadStr, _ := cmd.Flags().GetString("payload")

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("invalid payload JSON: %w", err)
	}

	body := map[string]interface{}{
		"event_type": eventType,
		"payload":    payload,
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks/%s/test", org, webhookID)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to test webhook: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	success, _ := resp["success"].(bool)
	if success {
		fmt.Fprintf(cmd.OutOrStdout(), "%s Test webhook sent successfully.\n", colorGreen("✓"))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s Test webhook failed.\n", colorRed("✗"))
		if errorMsg, ok := resp["error"].(string); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "Error: %s\n", errorMsg)
		}
	}

	if delivery, ok := resp["delivery"].(map[string]interface{}); ok {
		if httpStatus, ok := delivery["http_status"].(float64); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "HTTP Status: %.0f\n", httpStatus)
		}
		if isVerbose() {
			if responseBody, ok := delivery["response_body"].(string); ok && responseBody != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Response: %s\n", responseBody)
			}
		}
	}

	return nil
}

func runWebhooksDeliveries(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	webhookID := args[0]

	params := []string{}
	if status, _ := cmd.Flags().GetString("status"); status != "" {
		params = append(params, "status="+status)
	}
	if eventType, _ := cmd.Flags().GetString("event-type"); eventType != "" {
		params = append(params, "event_type="+eventType)
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/webhooks/%s/deliveries", org, webhookID)
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get webhook deliveries: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	deliveries, _ := resp["deliveries"].([]interface{})
	if len(deliveries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No deliveries found.")
		return nil
	}

	detailed, _ := cmd.Flags().GetBool("detailed")

	if detailed {
		for i, d := range deliveries {
			delivery, ok := d.(map[string]interface{})
			if !ok {
				continue
			}

			if i > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Delivery %s:\n", delivery["id"])
			fmt.Fprintf(cmd.OutOrStdout(), "  Event Type: %s\n", delivery["event_type"])
			fmt.Fprintf(cmd.OutOrStdout(), "  Status:     %s\n", delivery["status"])
			if httpStatus, ok := delivery["http_status"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  HTTP Status: %.0f\n", httpStatus)
			}
			if attemptCount, ok := delivery["attempt_count"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Attempts:   %.0f\n", attemptCount)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Created:    %s\n", delivery["created_at"])
			if sentAt, ok := delivery["sent_at"].(string); ok && sentAt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Sent:       %s\n", sentAt)
			}
			if errorMsg, ok := delivery["error_message"].(string); ok && errorMsg != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Error:      %s\n", errorMsg)
			}
		}
	} else {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tEVENT\tSTATUS\tHTTP\tATTEMPTS\tCREATED")
		for _, d := range deliveries {
			delivery, ok := d.(map[string]interface{})
			if !ok {
				continue
			}

			id, _ := delivery["id"].(string)
			eventType, _ := delivery["event_type"].(string)
			status, _ := delivery["status"].(string)
			httpStatus := ""
			if hs, ok := delivery["http_status"].(float64); ok {
				httpStatus = fmt.Sprintf("%.0f", hs)
			}
			attemptCount, _ := delivery["attempt_count"].(float64)
			createdAt, _ := delivery["created_at"].(string)

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f\t%s\n",
				id[:8], eventType, status, httpStatus, attemptCount, createdAt)
		}
		w.Flush()
	}

	return nil
}

func runWebhooksEvents(cmd *cobra.Command, args []string) error {
	// This could be enhanced to fetch from API, but for now return static list
	events := map[string][]string{
		"flag": {
			"flag.created",
			"flag.updated",
			"flag.toggled",
			"flag.archived",
			"flag.evaluated",
		},
		"deployment": {
			"deployment.created",
			"deployment.started",
			"deployment.completed",
			"deployment.failed",
			"deployment.rolledback",
			"deployment.paused",
			"deployment.resumed",
		},
		"release": {
			"release.created",
			"release.promoted",
		},
		"system": {
			"system.alert",
			"system.recovery",
			"audit.log",
		},
	}

	category, _ := cmd.Flags().GetString("category")
	detailed, _ := cmd.Flags().GetBool("detailed")

	if category != "" {
		if eventList, ok := events[category]; ok {
			for _, event := range eventList {
				fmt.Fprintln(cmd.OutOrStdout(), event)
			}
		} else {
			return fmt.Errorf("unknown category: %s", category)
		}
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Available Webhook Events:")
	for cat, eventList := range events {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s Events:\n", strings.Title(cat))
		for _, event := range eventList {
			if detailed {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s - %s\n", event, getEventDescription(event))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", event)
			}
		}
	}

	return nil
}

func getEventDescription(event string) string {
	descriptions := map[string]string{
		"flag.created":           "A new feature flag was created",
		"flag.updated":           "A feature flag configuration was updated",
		"flag.toggled":           "A feature flag was enabled or disabled",
		"flag.archived":          "A feature flag was archived",
		"flag.evaluated":         "A feature flag was evaluated (high volume)",
		"deployment.created":     "A new deployment was created",
		"deployment.started":     "A deployment process started",
		"deployment.completed":   "A deployment completed successfully",
		"deployment.failed":      "A deployment failed",
		"deployment.rolledback":  "A deployment was rolled back",
		"deployment.paused":      "A deployment was paused",
		"deployment.resumed":     "A deployment was resumed",
		"release.created":        "A new release was created",
		"release.promoted":       "A release was promoted to an environment",
		"system.alert":           "A system alert was triggered",
		"system.recovery":        "A system recovered from an alert condition",
		"audit.log":              "An audit log event occurred",
	}

	if desc, ok := descriptions[event]; ok {
		return desc
	}
	return "No description available"
}