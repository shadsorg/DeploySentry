package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// integrationsCmd is the parent for agentless-reporting integration operations.
var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage deploy-event webhook integrations (Railway, Render, Fly, …)",
	Long: `Manage deploy-event webhook integrations for the agentless reporting flow.

Each integration binds an application to a provider's webhook URL so deploys
happening outside DeploySentry (Railway, Render, Fly, Heroku, Vercel, Netlify,
GitHub Actions, or a generic CI step) are recorded as mode=record deployments.

Subcommands live under 'integrations deploy' and mirror the
POST/GET/DELETE /api/v1/integrations/deploys endpoints.

Examples:
  # Create a Railway integration
  deploysentry integrations deploy create \
    --app api-server --provider railway \
    --webhook-secret "$WEBHOOK_SECRET" \
    --provider-config '{"service_id":"svc-123"}' \
    --env-mapping production=dev-env-uuid,staging=stg-env-uuid

  # List integrations for an app
  deploysentry integrations deploy list --app api-server

  # Delete one
  deploysentry integrations deploy delete <integration-id>`,
}

var integrationsDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Manage deploy-event integrations",
}

var integrationsDeployCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a deploy-event integration for an application",
	RunE:  runIntegrationsDeployCreate,
}

var integrationsDeployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deploy-event integrations for an application",
	RunE:  runIntegrationsDeployList,
}

var integrationsDeployDeleteCmd = &cobra.Command{
	Use:   "delete <integration-id>",
	Short: "Delete a deploy-event integration",
	Args:  cobra.ExactArgs(1),
	RunE:  runIntegrationsDeployDelete,
}

func init() {
	// create
	integrationsDeployCreateCmd.Flags().String("app", "", "application slug or UUID (required)")
	integrationsDeployCreateCmd.Flags().String("provider", "", "provider: railway|render|fly|heroku|vercel|netlify|github-actions|generic (required)")
	integrationsDeployCreateCmd.Flags().String("webhook-secret", "", "HMAC signing secret (or bearer token when --auth-mode=bearer) (required)")
	integrationsDeployCreateCmd.Flags().String("auth-mode", "hmac", "auth mode: hmac (default) or bearer")
	integrationsDeployCreateCmd.Flags().String("provider-config", "{}", "JSON object with provider-specific config (e.g. '{\"service_id\":\"…\"}')")
	integrationsDeployCreateCmd.Flags().String("env-mapping", "", "comma-separated provider-env=ds-env pairs, e.g. 'production=<env-slug-or-uuid>,staging=<…>' (required)")
	integrationsDeployCreateCmd.Flags().StringSlice("version-extractor", nil, "dot-path tried in order when extracting version (repeatable)")
	_ = integrationsDeployCreateCmd.MarkFlagRequired("app")
	_ = integrationsDeployCreateCmd.MarkFlagRequired("provider")
	_ = integrationsDeployCreateCmd.MarkFlagRequired("webhook-secret")
	_ = integrationsDeployCreateCmd.MarkFlagRequired("env-mapping")

	// list
	integrationsDeployListCmd.Flags().String("app", "", "application slug or UUID (required)")
	_ = integrationsDeployListCmd.MarkFlagRequired("app")

	integrationsDeployCmd.AddCommand(integrationsDeployCreateCmd)
	integrationsDeployCmd.AddCommand(integrationsDeployListCmd)
	integrationsDeployCmd.AddCommand(integrationsDeployDeleteCmd)
	integrationsCmd.AddCommand(integrationsDeployCmd)
	rootCmd.AddCommand(integrationsCmd)
}

func runIntegrationsDeployCreate(cmd *cobra.Command, args []string) error {
	projectInput, _ := rootCmd.PersistentFlags().GetString("project")
	if projectInput == "" {
		projectInput, _ = cmd.Flags().GetString("project")
	}
	if projectInput == "" {
		// Fall back to viper (config file or DEPLOYSENTRY_PROJECT).
		projectInput, _ = requireProject()
	}

	appInput, _ := cmd.Flags().GetString("app")
	provider, _ := cmd.Flags().GetString("provider")
	secret, _ := cmd.Flags().GetString("webhook-secret")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	providerConfigRaw, _ := cmd.Flags().GetString("provider-config")
	envMappingRaw, _ := cmd.Flags().GetString("env-mapping")
	extractors, _ := cmd.Flags().GetStringSlice("version-extractor")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	// Resolve the app slug to UUID.
	_, projectSlug, perr := resolveProjectInput(client, projectInput)
	if perr != nil {
		return perr
	}
	appID, aerr := resolveAppInput(client, projectSlug, appInput)
	if aerr != nil {
		return aerr
	}

	// provider_config must be a JSON object.
	var providerConfig map[string]any
	if err := json.Unmarshal([]byte(providerConfigRaw), &providerConfig); err != nil {
		return fmt.Errorf("--provider-config must be a JSON object: %w", err)
	}

	// env_mapping: parse comma-separated key=value pairs, resolving each
	// value as a slug or UUID against /orgs/:slug/environments.
	envMapping := map[string]string{}
	for _, pair := range splitAndTrim(envMappingRaw, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return fmt.Errorf("invalid env-mapping entry %q (expected provider-env=ds-env)", pair)
		}
		envIDs, err := resolveEnvInputs(client, []string{kv[1]})
		if err != nil {
			return err
		}
		envMapping[kv[0]] = envIDs[0]
	}

	body := map[string]any{
		"application_id":  appID,
		"provider":        provider,
		"auth_mode":       authMode,
		"webhook_secret":  secret,
		"provider_config": providerConfig,
		"env_mapping":     envMapping,
	}
	if len(extractors) > 0 {
		body["version_extractors"] = extractors
	}

	resp, err := client.post("/api/v1/integrations/deploys", body)
	if err != nil {
		return fmt.Errorf("failed to create deploy integration: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	apiURL := client.baseURL
	webhookPath := fmt.Sprintf("/api/v1/integrations/%s/webhook", provider)
	if provider == "generic" {
		webhookPath = "/api/v1/integrations/deploys/webhook"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deploy integration created.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:           %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Provider:     %s\n", provider)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Webhook URL:  %s%s\n", apiURL, webhookPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nConfigure the provider to POST to that URL. Signing header and payload\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "expectations are documented in docs/Deploy_Integration_Guide.md.\n")
	if provider == "generic" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Include 'X-DeploySentry-Integration-Id: %s' on every call.\n", id)
	}
	return nil
}

func runIntegrationsDeployList(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	appInput, _ := cmd.Flags().GetString("app")
	projectInput, _ := cmd.Flags().GetString("project")
	if projectInput == "" {
		projectInput, _ = requireProject()
	}
	_, projectSlug, perr := resolveProjectInput(client, projectInput)
	if perr != nil {
		return perr
	}
	appID, aerr := resolveAppInput(client, projectSlug, appInput)
	if aerr != nil {
		return aerr
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/integrations/deploys?application_id=%s", appID))
	if err != nil {
		return fmt.Errorf("failed to list integrations: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	items, _ := resp["integrations"].([]interface{})
	if len(items) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No deploy integrations configured for this app.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tPROVIDER\tAUTH\tENABLED\tCREATED")
	for _, i := range items {
		it, ok := i.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := it["id"].(string)
		provider, _ := it["provider"].(string)
		authMode, _ := it["auth_mode"].(string)
		enabled, _ := it["enabled"].(bool)
		createdAt, _ := it["created_at"].(string)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\n", id, provider, authMode, enabled, createdAt)
	}
	return w.Flush()
}

func runIntegrationsDeployDelete(cmd *cobra.Command, args []string) error {
	id := args[0]
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	req, err := client.newRequest("DELETE", "/api/v1/integrations/deploys/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Integration %s deleted.\n", id)
	return nil
}
