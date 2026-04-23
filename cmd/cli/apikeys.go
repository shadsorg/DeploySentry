package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// apikeysCmd is the parent command for API key management operations.
var apikeysCmd = &cobra.Command{
	Use:     "apikeys",
	Aliases: []string{"apikey", "api-keys"},
	Short:   "Manage API keys",
	Long: `Create, list, and revoke API keys for programmatic access to DeploySentry.

API keys are user-scoped credentials that grant access to the DeploySentry
API. Each key can be assigned one or more scopes to limit what it can do.
The full token is displayed only at creation time — store it securely.

Examples:
  # List your API keys
  deploysentry apikeys list

  # Create a new key
  deploysentry apikeys create --name "CI Pipeline" --scopes "deploy:write,flags:read"

  # Revoke a key
  deploysentry apikeys revoke abc123`,
}

var apikeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	Long: `List all API keys associated with your account.

Examples:
  # List all keys
  deploysentry apikeys list

  # Output as JSON
  deploysentry apikeys list -o json`,
	RunE: runAPIKeysList,
}

var apikeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	Long: `Create a new API key with the given name and permission scopes.

The full token is displayed only once immediately after creation.
Make sure to copy it to a secure location before closing the terminal.

Available scopes:
  flags:read, flags:write             — feature flag API access
  deploys:read, deploys:write         — deployment API access
  releases:read, releases:write       — release API access
  status:write                        — POST /applications/:id/status (SDK reporter, agentless reporting)
  apikey:manage                       — create / rotate / revoke other API keys
  admin                               — superset; grants every scope above

Requires the caller to hold apikey:manage (or admin) — see Getting_Started.md
for how to bootstrap the first manage-capable key from the dashboard.

Examples:
  # CI pipeline key
  deploysentry apikeys create --name "CI Pipeline" \
    --scopes "deploys:write,flags:read,releases:read"

  # SDK status reporter key (scoped to one app + env)
  deploysentry apikeys create --name "api-server prod reporter" \
    --scopes "flags:read,status:write" \
    --app <app-uuid> --env <env-uuid>

  # Read-only monitoring key
  deploysentry apikeys create --name "Monitoring" --scopes "flags:read,releases:read"`,
	RunE: runAPIKeysCreate,
}

var apikeysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key",
	Long: `Revoke an API key by its ID. The key will immediately stop working
and cannot be restored.

Examples:
  # Revoke a key
  deploysentry apikeys revoke abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runAPIKeysRevoke,
}

func init() {
	// apikeys create flags
	apikeysCreateCmd.Flags().String("name", "", "human-readable name for the API key (required)")
	apikeysCreateCmd.Flags().String("scopes", "", "comma-separated list of permission scopes (required)")
	apikeysCreateCmd.Flags().StringSlice("env", nil, "environment ID(s) to restrict key to (repeatable, omit for unrestricted)")
	apikeysCreateCmd.Flags().String("project", "", "project UUID to restrict key to (optional)")
	apikeysCreateCmd.Flags().String("app", "", "application UUID to restrict key to (optional; used with SDK reporter + /status calls)")
	_ = apikeysCreateCmd.MarkFlagRequired("name")
	_ = apikeysCreateCmd.MarkFlagRequired("scopes")

	apikeysCmd.AddCommand(apikeysListCmd)
	apikeysCmd.AddCommand(apikeysCreateCmd)
	apikeysCmd.AddCommand(apikeysRevokeCmd)

	rootCmd.AddCommand(apikeysCmd)
}

func runAPIKeysList(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	resp, err := client.get("/api/v1/api-keys")
	if err != nil {
		return fmt.Errorf("failed to list API keys: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	apiKeys, _ := resp["api_keys"].([]interface{})
	if len(apiKeys) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No API keys found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tPREFIX\tSCOPES\tENVIRONMENTS\tCREATED\tLAST_USED\tEXPIRES")
	for _, k := range apiKeys {
		key, ok := k.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := key["id"].(string)
		name, _ := key["name"].(string)
		prefix, _ := key["prefix"].(string)
		createdAt, _ := key["created_at"].(string)
		lastUsed, _ := key["last_used_at"].(string)
		expiresAt, _ := key["expires_at"].(string)

		scopes := ""
		if scopeList, ok := key["scopes"].([]interface{}); ok {
			scopeStrs := make([]string, 0, len(scopeList))
			for _, s := range scopeList {
				if str, ok := s.(string); ok {
					scopeStrs = append(scopeStrs, str)
				}
			}
			scopes = strings.Join(scopeStrs, ",")
			if len(scopes) > 30 {
				scopes = scopes[:27] + "..."
			}
		}

		envDisplay := "All"
		if envIDsRaw, ok := key["environment_ids"]; ok {
			if arr, ok := envIDsRaw.([]interface{}); ok && len(arr) > 0 {
				envDisplay = fmt.Sprintf("%d env(s)", len(arr))
			}
		}

		if lastUsed == "" {
			lastUsed = "never"
		}
		if expiresAt == "" {
			expiresAt = "never"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id, name, prefix, scopes, envDisplay, createdAt, lastUsed, expiresAt)
	}
	return w.Flush()
}

func runAPIKeysCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	scopesRaw, _ := cmd.Flags().GetString("scopes")

	scopes := splitAndTrim(scopesRaw, ",")
	envInputs, _ := cmd.Flags().GetStringSlice("env")
	projectInput, _ := cmd.Flags().GetString("project")
	appInput, _ := cmd.Flags().GetString("app")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	// --project / --app / --env accept either slug or UUID. Resolve
	// slugs client-side so the server only ever sees UUIDs (which is
	// what the POST /api-keys endpoint currently requires).
	projectID, projectSlug, err := resolveProjectInput(client, projectInput)
	if err != nil {
		return err
	}
	var appID string
	if appInput != "" {
		if projectID == "" {
			return fmt.Errorf("--app requires --project to resolve the application (pass a project slug or UUID)")
		}
		appID, err = resolveAppInput(client, projectSlug, appInput)
		if err != nil {
			return err
		}
	}
	envIDs, err := resolveEnvInputs(client, envInputs)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}
	if len(envIDs) > 0 {
		body["environment_ids"] = envIDs
	}
	if projectID != "" {
		body["project_id"] = projectID
	}
	if appID != "" {
		body["application_id"] = appID
	}

	resp, err := client.post("/api/v1/api-keys", body)
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	id, _ := resp["id"].(string)
	token, _ := resp["token"].(string)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", id)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Scopes: %s\n", strings.Join(scopes, ", "))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token (shown only once — save it now):\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", token)
	return nil
}

func runAPIKeysRevoke(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/api-keys/%s", keyID)
	_, err = client.delete(path)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key %s revoked successfully.\n", keyID)
	return nil
}
