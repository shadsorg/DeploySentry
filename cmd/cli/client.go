package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// apiClient is an HTTP client for communicating with the DeploySentry API.
type apiClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string
	userAgent  string
}

// newAPIClient creates a new API client pointed at the given base URL.
func newAPIClient(baseURL string) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: fmt.Sprintf("deploysentry-cli/%s", version),
	}
}

// setToken sets the Bearer token for authentication.
func (c *apiClient) setToken(token string) {
	c.token = token
	c.apiKey = ""
}

// setAPIKey sets the API key for authentication.
func (c *apiClient) setAPIKey(key string) {
	c.apiKey = key
	c.token = ""
}

// newRequest creates a new HTTP request with the appropriate auth headers.
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers.
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set authentication header.
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	}

	return req, nil
}

// do executes an HTTP request and decodes the JSON response.
func (c *apiClient) do(req *http.Request) (map[string]interface{}, error) {
	if isVerbose() {
		fmt.Printf("--> %s %s\n", req.Method, req.URL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if isVerbose() {
		fmt.Printf("<-- %d %s\n", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-2xx status codes.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, data)
	}

	// Parse JSON response.
	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	return result, nil
}

// get performs an HTTP GET request to the given API path.
func (c *apiClient) get(path string) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// post performs an HTTP POST request to the given API path with the given body.
func (c *apiClient) post(path string, body interface{}) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// patch performs an HTTP PATCH request to the given API path with the given body.
func (c *apiClient) patch(path string, body interface{}) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// put performs an HTTP PUT request to the given API path with the given body.
func (c *apiClient) put(path string, body interface{}) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// delete performs an HTTP DELETE request to the given API path.
func (c *apiClient) delete(path string) (map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// apiError represents a structured error response from the DeploySentry API.
type apiError struct {
	StatusCode int
	Code       string
	Message    string
	Details    map[string]interface{}
}

func (e *apiError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("API error %d (%s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// parseAPIError parses an error response body into a structured apiError.
func parseAPIError(statusCode int, body []byte) *apiError {
	apiErr := &apiError{
		StatusCode: statusCode,
	}

	// Try to parse as JSON error response.
	var errResp struct {
		Error   string                 `json:"error"`
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			apiErr.Message = errResp.Message
		} else if errResp.Error != "" {
			apiErr.Message = errResp.Error
		}
		apiErr.Code = errResp.Code
		apiErr.Details = errResp.Details
	}

	// Fallback to standard HTTP status descriptions.
	if apiErr.Message == "" {
		switch statusCode {
		case http.StatusUnauthorized:
			apiErr.Message = "authentication required; run 'deploysentry auth login'"
		case http.StatusForbidden:
			apiErr.Message = "insufficient permissions for this operation"
		case http.StatusNotFound:
			apiErr.Message = "resource not found"
		case http.StatusConflict:
			apiErr.Message = "resource conflict"
		case http.StatusUnprocessableEntity:
			apiErr.Message = "invalid request parameters"
		case http.StatusTooManyRequests:
			apiErr.Message = "rate limit exceeded; try again later"
		case http.StatusInternalServerError:
			apiErr.Message = "internal server error; try again later"
		case http.StatusServiceUnavailable:
			apiErr.Message = "service temporarily unavailable; try again later"
		default:
			apiErr.Message = fmt.Sprintf("unexpected status code %d", statusCode)
		}
	}

	return apiErr
}

// exchangeAuthCode exchanges an OAuth authorization code for tokens.
func (c *apiClient) exchangeAuthCode(code, redirectURI string) (*credentialsFile, error) {
	body := map[string]string{
		"grant_type":   "authorization_code",
		"code":         code,
		"redirect_uri": redirectURI,
		"client_id":    "cli",
	}

	req, err := c.newRequest(http.MethodPost, "/oauth/token", body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(data))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		User         string `json:"user"`
		Email        string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	return &credentialsFile{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		User:         tokenResp.User,
		Email:        tokenResp.Email,
	}, nil
}
