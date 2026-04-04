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

Examples:
  # Create a key with multiple scopes
  deploysentry apikeys create --name "CI Pipeline" \
    --scopes "deploy:write,flags:read,releases:read"

  # Create a read-only key
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
	_, _ = fmt.Fprintln(w, "ID\tNAME\tPREFIX\tSCOPES\tCREATED\tLAST_USED\tEXPIRES")
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

		if lastUsed == "" {
			lastUsed = "never"
		}
		if expiresAt == "" {
			expiresAt = "never"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id, name, prefix, scopes, createdAt, lastUsed, expiresAt)
	}
	return w.Flush()
}

func runAPIKeysCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	scopesRaw, _ := cmd.Flags().GetString("scopes")

	scopes := splitAndTrim(scopesRaw, ",")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
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
