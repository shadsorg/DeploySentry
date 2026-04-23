package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// authCmd is the parent command for authentication operations.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication with the DeploySentry platform",
	Long: `Manage authentication credentials for the DeploySentry CLI.

The CLI authenticates using an API key created in the DeploySentry
dashboard. Use 'deploysentry auth login' once per machine to paste the
key; credentials are stored in $HOME/.config/deploysentry/credentials.json
and reused by subsequent commands and by the MCP server.

Examples:
  # Log in (interactive prompt)
  deploysentry auth login

  # Log in non-interactively
  deploysentry auth login --token ds_live_xxx

  # Pick up a key from the environment
  DEPLOYSENTRY_API_KEY=ds_live_xxx deploysentry auth login

  # Check current auth status
  deploysentry auth status

  # Log out and clear credentials
  deploysentry auth logout`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with DeploySentry using an API key",
	Long: `Save a DeploySentry API key so the CLI (and MCP server) can
authenticate against the API.

Create a key in the dashboard at:
  <your-dashboard-url>/orgs/<org-slug>/api-keys

The key can be supplied in three ways (in priority order):
  1. --token flag
  2. DEPLOYSENTRY_API_KEY environment variable
  3. Interactive stdin prompt (default when no flag/env is set)

The key is validated by calling an authenticated API endpoint before
being persisted, so bad keys are rejected immediately.

Examples:
  deploysentry auth login
  deploysentry auth login --token ds_live_xxx
  DEPLOYSENTRY_API_KEY=ds_live_xxx deploysentry auth login`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication credentials",
	Long: `Remove the locally stored API key. The next CLI command will
prompt you to run 'deploysentry auth login' again.`,
	RunE: runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display current authentication status",
	Long: `Show the current authentication state.

Examples:
  deploysentry auth status
  deploysentry auth status -o json`,
	RunE: runAuthStatus,
}

func init() {
	authLoginCmd.Flags().String("api-url", "", "override the API base URL for authentication")
	authLoginCmd.Flags().String("token", "", "API key (skips the interactive prompt)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	rootCmd.AddCommand(authCmd)
}

// credentialsFile is the on-disk shape persisted by `auth login`.
//
// Historically this file held OAuth tokens; today it holds an API key.
// The shape is kept wide so older files from experimental builds can
// still be read without breaking. `TokenType` drives the auth scheme:
// "api_key" → Authorization: ApiKey <AccessToken>, anything else →
// Authorization: Bearer <AccessToken>.
type credentialsFile struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	User         string    `json:"user,omitempty"`
	Email        string    `json:"email,omitempty"`
}

const tokenTypeAPIKey = "api_key"

// credentialsPath returns the path to the credentials file.
func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "deploysentry", "credentials.json"), nil
}

// saveCredentials writes credentials to disk.
func saveCredentials(creds *credentialsFile) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}
	return nil
}

// loadCredentials reads credentials from disk.
func loadCredentials() (*credentialsFile, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not authenticated; run 'deploysentry auth login' first")
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}
	var creds credentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}
	return &creds, nil
}

// ensureValidToken loads credentials and returns them. API-key creds never
// expire; legacy bearer-token files are returned as-is (no refresh flow —
// the server-side OAuth endpoints are not implemented).
func ensureValidToken() (*credentialsFile, error) {
	creds, err := loadCredentials()
	if err != nil {
		return nil, err
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("credentials file is missing an access token; run 'deploysentry auth login' to re-authenticate")
	}
	return creds, nil
}

// runAuthLogin persists a DeploySentry API key to the credentials file.
func runAuthLogin(cmd *cobra.Command, args []string) error {
	apiURL, _ := cmd.Flags().GetString("api-url")
	if apiURL == "" {
		apiURL = viper.GetString("api_url")
	}
	if apiURL == "" {
		apiURL = "https://api.dr-sentry.com"
	}

	token, _ := cmd.Flags().GetString("token")
	if token == "" {
		token = os.Getenv("DEPLOYSENTRY_API_KEY")
	}
	if token == "" {
		prompted, err := promptForToken(cmd)
		if err != nil {
			return err
		}
		token = prompted
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("no API key provided")
	}

	if err := validateAPIKey(apiURL, token); err != nil {
		return fmt.Errorf("api key validation failed: %w", err)
	}

	creds := &credentialsFile{
		AccessToken: token,
		TokenType:   tokenTypeAPIKey,
	}
	if err := saveCredentials(creds); err != nil {
		return fmt.Errorf("validated key but failed to save credentials: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authenticated.")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Credentials saved to %s\n", mustCredentialsPath())
	return nil
}

// promptForToken reads a key from stdin.
func promptForToken(cmd *cobra.Command) (string, error) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Create an API key in the dashboard (Org → API Keys),")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "then paste it below. You can also skip this prompt with")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  --token <value>    or")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  DEPLOYSENTRY_API_KEY=<value> deploysentry auth login")
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "\nAPI key: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read API key from stdin: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// validateAPIKey calls an authenticated endpoint to confirm the key works.
// Uses GET /api/v1/orgs because every authenticated key has visibility
// to at least one org list operation and the server never 500s on empty
// results.
func validateAPIKey(apiURL, token string) error {
	client := newAPIClient(apiURL)
	client.setAPIKey(token)

	req, err := client.newRequest(http.MethodGet, "/api/v1/orgs", nil)
	if err != nil {
		return err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reach %s: %w", apiURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("the API rejected this key (HTTP %d)", resp.StatusCode)
	default:
		return fmt.Errorf("unexpected HTTP %d while validating the key", resp.StatusCode)
	}
}

// runAuthLogout clears stored credentials.
func runAuthLogout(cmd *cobra.Command, args []string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No credentials found; already logged out.\n")
			return nil
		}
		return fmt.Errorf("failed to remove credentials: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged out successfully. Credentials removed.\n")
	return nil
}

// runAuthStatus displays the current authentication state.
func runAuthStatus(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Status: Not authenticated\n")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Run 'deploysentry auth login' to authenticate.\n")
		return nil
	}

	if getOutputFormat() == "json" {
		status := map[string]interface{}{
			"authenticated": true,
			"user":          creds.User,
			"email":         creds.Email,
			"token_type":    creds.TokenType,
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Status:     Authenticated\n")
	if creds.User != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "User:       %s\n", creds.User)
	}
	if creds.Email != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Email:      %s\n", creds.Email)
	}
	tokenType := creds.TokenType
	if tokenType == "" {
		tokenType = "unknown"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token Type: %s\n", tokenType)
	return nil
}

// mustCredentialsPath returns the credentials path or a placeholder on error.
func mustCredentialsPath() string {
	p, err := credentialsPath()
	if err != nil {
		return "<unknown>"
	}
	return p
}
