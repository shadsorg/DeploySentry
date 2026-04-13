package deploysentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	defaultBaseURL      = "https://api.deploysentry.io"
	defaultCacheTimeout = 5 * time.Minute
)

// Client is the main entry point for the DeploySentry Go SDK. It manages
// flag evaluation, caching, and real-time streaming updates.
type Client struct {
	apiKey      string
	baseURL     string
	environment string
	projectID   string
	sessionID   string
	offlineMode bool
	httpClient  *http.Client
	cache       *flagCache
	sse         *sseClient
	logger      *log.Logger
	registry    map[string][]registration
	registryMu  sync.RWMutex
}

// Option configures a Client. Pass options to NewClient.
type Option func(*Client)

// WithAPIKey sets the API key used for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithEnvironment sets the environment identifier (e.g. "production",
// "staging").
func WithEnvironment(env string) Option {
	return func(c *Client) { c.environment = env }
}

// WithProject sets the project identifier.
func WithProject(id string) Option {
	return func(c *Client) { c.projectID = id }
}

// WithOfflineMode enables offline mode. When the API is unreachable the
// client serves stale cached values instead of returning errors.
func WithOfflineMode(enabled bool) Option {
	return func(c *Client) { c.offlineMode = enabled }
}

// WithCacheTimeout sets the TTL for cached flag values. A zero duration
// means cache entries never expire.
func WithCacheTimeout(d time.Duration) Option {
	return func(c *Client) { c.cache.ttl = d }
}

// WithHTTPClient sets a custom *http.Client for all API requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithSessionID sets a session identifier for consistent flag evaluation
// across multiple requests. This ensures a user sees the same flag values
// throughout their session.
func WithSessionID(id string) Option {
	return func(c *Client) { c.sessionID = id }
}

// WithLogger sets a custom logger. By default the SDK logs to stderr.
func WithLogger(l *log.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// NewClient creates a new DeploySentry client. At minimum you should provide
// WithAPIKey. Call Initialize to warm the cache and start streaming.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      newFlagCache(defaultCacheTimeout),
		logger:     log.New(os.Stderr, "", log.LstdFlags),
		registry:   make(map[string][]registration),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Initialize fetches all flags for the configured project and starts the
// SSE streaming connection for real-time updates. It should be called once
// during application startup.
func (c *Client) Initialize(ctx context.Context) error {
	if err := c.warmCache(ctx); err != nil {
		if !c.offlineMode {
			return fmt.Errorf("deploysentry: failed to warm cache: %w", err)
		}
		c.logger.Printf("deploysentry: failed to warm cache (offline mode active): %v", err)
	}

	// Start SSE streaming in the background.
	c.sse = newSSEClient(c.baseURL, c.apiKey, c.projectID, c.environment, c.sessionID, c.httpClient, func(flag Flag) {
		c.cache.set(flag)
	}, c.logger)
	c.sse.start(ctx)

	return nil
}

// RefreshSession clears the flag cache and re-fetches all flags from the
// API. Use this when the session context has changed and you need fresh
// evaluations.
func (c *Client) RefreshSession(ctx context.Context) error {
	c.cache.clear()
	return c.warmCache(ctx)
}

// Close stops the SSE streaming connection and releases resources.
func (c *Client) Close() {
	if c.sse != nil {
		c.sse.stop()
	}
}

// FlagsByCategory returns all cached flags matching the given category.
func (c *Client) FlagsByCategory(category FlagCategory) []Flag {
	return c.cache.byCategory(category)
}

// ExpiredFlags returns all cached flags whose expiration date is in the past.
func (c *Client) ExpiredFlags() []Flag {
	return c.cache.expired()
}

// FlagOwners returns the owners list for the given flag key, or nil if the
// flag is not in the cache.
func (c *Client) FlagOwners(flagKey string) []string {
	return c.cache.owners(flagKey)
}

// AllFlags returns a snapshot of every flag currently in the cache.
func (c *Client) AllFlags() []Flag {
	return c.cache.all()
}

// warmCache fetches all flags from the API and populates the cache.
func (c *Client) warmCache(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/flags?project_id=%s", c.baseURL, c.projectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var listResp listFlagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("decoding flags response: %w", err)
	}

	c.cache.setAll(listResp.Flags)
	return nil
}

// doEvaluate sends a single flag evaluation request to the API.
func (c *Client) doEvaluate(ctx context.Context, flagKey string, evalCtx *EvaluationContext) (*evaluateResponse, error) {
	body := evaluateRequest{
		FlagKey:     flagKey,
		Context:     evalCtx,
		Environment: c.environment,
		ProjectID:   c.projectID,
		SessionID:   c.sessionID,
	}

	respBody, err := c.post(ctx, "/api/v1/flags/evaluate", body)
	if err != nil {
		return nil, err
	}

	var result evaluateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding evaluation response: %w", err)
	}

	return &result, nil
}

// post sends a POST request with JSON body and returns the response bytes.
func (c *Client) post(ctx context.Context, path string, payload interface{}) ([]byte, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Register associates a handler with an operation name and an optional flag key.
// When Dispatch is called for the operation, the handler is returned if its
// associated flag is enabled. If no flagKey is provided the handler becomes the
// default, returned when no flagged handler matches. Registering a new default
// replaces the previous one.
func (c *Client) Register(operation string, handler any, flagKey ...string) {
	key := ""
	if len(flagKey) > 0 {
		key = flagKey[0]
	}
	c.registryMu.Lock()
	defer c.registryMu.Unlock()
	list := c.registry[operation]
	if key == "" {
		for i, r := range list {
			if r.flagKey == "" {
				list[i] = registration{handler: handler, flagKey: ""}
				c.registry[operation] = list
				return
			}
		}
		list = append(list, registration{handler: handler, flagKey: ""})
	} else {
		list = append(list, registration{handler: handler, flagKey: key})
	}
	c.registry[operation] = list
}

// Dispatch returns the handler registered for the given operation whose
// associated flag is currently enabled. If no flagged handler matches the
// default handler (registered without a flagKey) is returned. Dispatch panics
// if no handlers are registered for the operation or if no handler matches and
// no default is registered.
func (c *Client) Dispatch(operation string, ctx ...EvaluationContext) any {
	c.registryMu.RLock()
	list, ok := c.registry[operation]
	c.registryMu.RUnlock()
	if !ok || len(list) == 0 {
		panic(fmt.Sprintf("No handlers registered for operation '%s'. Call Register() before Dispatch().", operation))
	}
	for _, reg := range list {
		if reg.flagKey != "" {
			f, found, _ := c.cache.get(reg.flagKey)
			if found && f.Enabled {
				return reg.handler
			}
		}
	}
	for _, reg := range list {
		if reg.flagKey == "" {
			return reg.handler
		}
	}
	panic(fmt.Sprintf("No matching handler for operation '%s' and no default registered. Register a default handler (no flagKey) as the last registration.", operation))
}

// setAuthHeaders adds the authorization and common headers to a request.
func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if c.sessionID != "" {
		req.Header.Set("X-DeploySentry-Session", c.sessionID)
	}
}
