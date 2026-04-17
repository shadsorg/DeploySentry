// Package mcpserver provides an MCP (Model Context Protocol) server for
// DeploySentry, exposing CLI functionality as tools that AI assistants can call.
package mcpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
)

// apiClient is a lightweight HTTP client for the DeploySentry API.
type apiClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string
}

// credentials mirrors the on-disk OAuth token file.
type credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         string    `json:"user"`
	Email        string    `json:"email"`
}

// checkReady builds an apiClient from config/env/credentials and returns an
// actionable error when authentication is missing.
func checkReady() (*apiClient, error) {
	baseURL := viper.GetString("api_url")
	if baseURL == "" {
		baseURL = os.Getenv("DEPLOYSENTRY_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.dr-sentry.com"
	}

	c := &apiClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Prefer explicit API key.
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = os.Getenv("DEPLOYSENTRY_API_KEY")
	}
	if apiKey != "" {
		c.apiKey = apiKey
		return c, nil
	}

	// Fall back to OAuth credentials file.
	creds, err := loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w\nRun 'deploysentry auth login' or set DEPLOYSENTRY_API_KEY", err)
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("credentials file has no access token; run 'deploysentry auth login'")
	}
	c.token = creds.AccessToken
	return c, nil
}

// loadCredentials reads ~/.config/deploysentry/credentials.json.
func loadCredentials() (*credentials, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determine home directory: %w", err)
	}
	path := filepath.Join(home, ".config", "deploysentry", "credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	return &creds, nil
}

// resolveOrg returns the org from the given param, falling back to Viper config.
func resolveOrg(param string) (string, error) {
	if param != "" {
		return param, nil
	}
	org := viper.GetString("org")
	if org == "" {
		return "", fmt.Errorf("org is required: pass it as a parameter, set DEPLOYSENTRY_ORG, or add 'org' to ~/.deploysentry.yml")
	}
	return org, nil
}

// resolveProject returns the project from the given param, falling back to Viper config.
func resolveProject(param string) (string, error) {
	if param != "" {
		return param, nil
	}
	project := viper.GetString("project")
	if project == "" {
		return "", fmt.Errorf("project is required: pass it as a parameter, set DEPLOYSENTRY_PROJECT, or add 'project' to ~/.deploysentry.yml")
	}
	return project, nil
}

// resolveApp resolves an application slug to its UUID by calling the API.
// Requires org and project to be already resolved.
func resolveApp(c *apiClient, org, project, app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("app is required: pass it as a parameter")
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, project, app))
	if err != nil {
		return "", fmt.Errorf("failed to resolve app '%s': %w", app, err)
	}
	id, ok := data["id"].(string)
	if !ok {
		return "", fmt.Errorf("app '%s' not found or missing id in response", app)
	}
	return id, nil
}

// resolveEnv resolves an environment slug to its UUID by calling the API.
func resolveEnv(c *apiClient, org, env string) (string, error) {
	if env == "" {
		return "", fmt.Errorf("env is required: pass it as a parameter")
	}
	data, err := c.get(fmt.Sprintf("/api/v1/orgs/%s/environments", org))
	if err != nil {
		return "", fmt.Errorf("failed to list environments: %w", err)
	}
	envs, ok := data["environments"].([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected environments response format")
	}
	for _, e := range envs {
		em, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if em["slug"] == env || em["name"] == env {
			if id, ok := em["id"].(string); ok {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("environment '%s' not found", env)
}

// get performs an HTTP GET request.
func (c *apiClient) get(path string) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// post performs an HTTP POST request with a JSON body.
func (c *apiClient) post(path string, body interface{}) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *apiClient) newRequest(method, path string, body interface{}) (*http.Request, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "deploysentry-mcp/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	}

	return req, nil
}

func (c *apiClient) do(req *http.Request) (map[string]interface{}, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}
	return result, nil
}

// jsonResult marshals data to indented JSON and returns it as a CallToolResult.
func jsonResult(data interface{}) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// errResult returns an error as a CallToolResult with IsError set.
func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}
