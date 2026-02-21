package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// authCmd is the parent command for authentication operations.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication with the DeploySentry platform",
	Long: `Manage authentication credentials for the DeploySentry CLI.

The auth command group allows you to log in via browser-based OAuth,
view your current authentication status, and log out by clearing
stored credentials.

Credentials are stored in $HOME/.config/deploysentry/credentials.json.

Examples:
  # Log in via browser
  deploysentry auth login

  # Check current auth status
  deploysentry auth status

  # Log out and clear credentials
  deploysentry auth logout`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with DeploySentry via browser-based OAuth",
	Long: `Open a browser window to authenticate with DeploySentry using OAuth 2.0.

A local HTTP server is started on a random port to receive the OAuth
callback. The browser is opened automatically; if it cannot be opened,
the URL is printed so you can navigate to it manually.

After successful authentication, credentials are saved locally for
subsequent CLI operations.

Examples:
  # Log in with default settings
  deploysentry auth login

  # Log in to a specific API host
  deploysentry auth login --api-url https://api.deploysentry.example.com`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication credentials",
	Long: `Remove locally stored authentication credentials.

This command deletes the credentials file from the local filesystem.
You will need to log in again to perform authenticated operations.

Examples:
  # Log out
  deploysentry auth logout`,
	RunE: runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display current authentication status",
	Long: `Show the current authentication state, including the authenticated user
and token expiry information.

Examples:
  # Show auth status
  deploysentry auth status

  # Show auth status in JSON format
  deploysentry auth status -o json`,
	RunE: runAuthStatus,
}

func init() {
	authLoginCmd.Flags().String("api-url", "", "override the API base URL for authentication")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	rootCmd.AddCommand(authCmd)
}

// credentialsFile holds the OAuth tokens persisted to disk.
type credentialsFile struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         string    `json:"user"`
	Email        string    `json:"email"`
}

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

// runAuthLogin performs the browser-based OAuth login flow.
func runAuthLogin(cmd *cobra.Command, args []string) error {
	apiURL, _ := cmd.Flags().GetString("api-url")
	if apiURL == "" {
		apiURL = viper.GetString("api_url")
	}
	if apiURL == "" {
		apiURL = "https://api.deploysentry.io"
	}

	// Generate a random state parameter for CSRF protection.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("failed to generate state parameter: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Start a local HTTP server to receive the OAuth callback.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	resultCh := make(chan *credentialsFile, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Validate state parameter.
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth state mismatch; possible CSRF attack")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			http.Error(w, "Authentication failed", http.StatusBadRequest)
			errCh <- fmt.Errorf("authentication failed: %s", errMsg)
			return
		}

		// Exchange the authorization code for tokens.
		client := newAPIClient(apiURL)
		tokenResp, err := client.exchangeAuthCode(code, callbackURL)
		if err != nil {
			http.Error(w, "Token exchange failed", http.StatusInternalServerError)
			errCh <- fmt.Errorf("token exchange failed: %w", err)
			return
		}

		// Display a success page in the browser.
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h2>Authentication successful!</h2>
			<p>You can close this window and return to the terminal.</p>
			<script>window.close();</script>
		</body></html>`)

		resultCh <- tokenResp
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Build the authorization URL.
	authURL := fmt.Sprintf(
		"%s/oauth/authorize?response_type=code&client_id=cli&redirect_uri=%s&state=%s&scope=read+write",
		apiURL, callbackURL, state,
	)

	fmt.Fprintf(cmd.OutOrStdout(), "Opening browser for authentication...\n")
	fmt.Fprintf(cmd.OutOrStdout(), "If the browser does not open, visit:\n  %s\n\n", authURL)

	// Attempt to open the browser.
	if err := openBrowser(authURL); err != nil {
		if isVerbose() {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open browser: %v\n", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Waiting for authentication...\n")

	// Wait for the callback or timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case creds := <-resultCh:
		_ = server.Shutdown(context.Background())
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("authenticated successfully but failed to save credentials: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Authenticated as %s (%s)\n", creds.User, creds.Email)
		fmt.Fprintf(cmd.OutOrStdout(), "Credentials saved to %s\n", mustCredentialsPath())
		return nil
	case err := <-errCh:
		_ = server.Shutdown(context.Background())
		return err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return fmt.Errorf("authentication timed out after 5 minutes")
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
			fmt.Fprintf(cmd.OutOrStdout(), "No credentials found; already logged out.\n")
			return nil
		}
		return fmt.Errorf("failed to remove credentials: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged out successfully. Credentials removed.\n")
	return nil
}

// runAuthStatus displays the current authentication state.
func runAuthStatus(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Status: Not authenticated\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Run 'deploysentry auth login' to authenticate.\n")
		return nil
	}

	expired := time.Now().After(creds.ExpiresAt)

	if getOutputFormat() == "json" {
		status := map[string]interface{}{
			"authenticated": true,
			"user":          creds.User,
			"email":         creds.Email,
			"token_type":    creds.TokenType,
			"expires_at":    creds.ExpiresAt.Format(time.RFC3339),
			"expired":       expired,
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Status:     Authenticated\n")
	fmt.Fprintf(cmd.OutOrStdout(), "User:       %s\n", creds.User)
	fmt.Fprintf(cmd.OutOrStdout(), "Email:      %s\n", creds.Email)
	fmt.Fprintf(cmd.OutOrStdout(), "Token Type: %s\n", creds.TokenType)
	fmt.Fprintf(cmd.OutOrStdout(), "Expires At: %s\n", creds.ExpiresAt.Format(time.RFC3339))
	if expired {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING:    Token has expired. Run 'deploysentry auth login' to re-authenticate.\n")
	}
	return nil
}

// openBrowser attempts to open a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	return cmd.Start()
}

// mustCredentialsPath returns the credentials path or a placeholder on error.
func mustCredentialsPath() string {
	p, err := credentialsPath()
	if err != nil {
		return "<unknown>"
	}
	return p
}
