package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/google/uuid"
)

// CustomHTTPConfig holds configuration for a custom HTTP health check endpoint.
type CustomHTTPConfig struct {
	// Name is a human-readable name for this custom check.
	Name string `json:"name"`

	// URL is the HTTP endpoint to poll for health status.
	URL string `json:"url"`

	// Method is the HTTP method to use (default: GET).
	Method string `json:"method"`

	// Headers are additional HTTP headers to include in the request.
	Headers map[string]string `json:"headers,omitempty"`

	// Interval is how often to poll the endpoint.
	Interval time.Duration `json:"interval"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`

	// HealthyStatusCodes are the HTTP status codes considered healthy.
	// If empty, any 2xx status code is considered healthy.
	HealthyStatusCodes []int `json:"healthy_status_codes,omitempty"`

	// ResponseParser defines how to extract the health signal from the response.
	// If nil, health is determined solely by the HTTP status code.
	ResponseParser *ResponseParserConfig `json:"response_parser,omitempty"`
}

// ResponseParserConfig defines how to extract health information from an
// HTTP response body.
type ResponseParserConfig struct {
	// Type is the response format type ("json" is currently supported).
	Type string `json:"type"`

	// ScorePath is a dot-separated JSON path to the health score field
	// (e.g., "data.health.score"). The value should be a number in [0, 1].
	ScorePath string `json:"score_path,omitempty"`

	// HealthyPath is a dot-separated JSON path to a boolean field indicating
	// health (e.g., "data.healthy").
	HealthyPath string `json:"healthy_path,omitempty"`

	// MessagePath is a dot-separated JSON path to a string field containing
	// a human-readable health message.
	MessagePath string `json:"message_path,omitempty"`
}

// DefaultCustomHTTPConfig returns a sensible default configuration for a
// custom HTTP health check.
func DefaultCustomHTTPConfig(name, url string) CustomHTTPConfig {
	return CustomHTTPConfig{
		Name:     name,
		URL:      url,
		Method:   http.MethodGet,
		Interval: 30 * time.Second,
		Timeout:  10 * time.Second,
	}
}

// CustomHTTPCheck implements health.HealthCheck by polling an arbitrary HTTP
// endpoint and extracting a health signal from the response.
type CustomHTTPCheck struct {
	config CustomHTTPConfig
	client *http.Client
}

// NewCustomHTTPCheck creates a new custom HTTP health check with the given configuration.
func NewCustomHTTPCheck(config CustomHTTPConfig) *CustomHTTPCheck {
	if config.Method == "" {
		config.Method = http.MethodGet
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	return &CustomHTTPCheck{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the check identifier.
func (c *CustomHTTPCheck) Name() string {
	if c.config.Name != "" {
		return c.config.Name
	}
	return "custom_http"
}

// Check polls the configured HTTP endpoint and extracts a health signal
// from the response.
func (c *CustomHTTPCheck) Check(ctx context.Context, deploymentID uuid.UUID) (*health.CheckResult, error) {
	result := &health.CheckResult{
		Name:      c.Name(),
		CheckedAt: time.Now().UTC(),
	}

	req, err := http.NewRequestWithContext(ctx, c.config.Method, c.config.URL, nil)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		return result, nil
	}

	// Apply configured headers.
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("request failed: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	// Check if the status code is considered healthy.
	statusHealthy := c.isHealthyStatus(resp.StatusCode)

	// Read response body for potential parsing.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // Limit to 1MB.
	if err != nil {
		result.Healthy = statusHealthy
		if statusHealthy {
			result.Score = 1.0
		}
		result.Message = fmt.Sprintf("status=%d (failed to read body: %v)", resp.StatusCode, err)
		return result, nil
	}

	// If a response parser is configured, extract health signal from body.
	if c.config.ResponseParser != nil && c.config.ResponseParser.Type == "json" {
		parsed, err := c.parseJSONResponse(body)
		if err != nil {
			result.Healthy = statusHealthy
			if statusHealthy {
				result.Score = 1.0
			}
			result.Message = fmt.Sprintf("status=%d (parse error: %v)", resp.StatusCode, err)
			return result, nil
		}

		result.Score = parsed.score
		result.Healthy = parsed.healthy && statusHealthy
		result.Message = parsed.message
		if result.Message == "" {
			result.Message = fmt.Sprintf("status=%d score=%.2f", resp.StatusCode, parsed.score)
		}
		return result, nil
	}

	// Without a parser, use status code only.
	result.Healthy = statusHealthy
	if statusHealthy {
		result.Score = 1.0
	}
	result.Message = fmt.Sprintf("status=%d", resp.StatusCode)
	return result, nil
}

// isHealthyStatus checks whether the HTTP status code is considered healthy.
func (c *CustomHTTPCheck) isHealthyStatus(code int) bool {
	if len(c.config.HealthyStatusCodes) > 0 {
		for _, healthy := range c.config.HealthyStatusCodes {
			if code == healthy {
				return true
			}
		}
		return false
	}
	// Default: any 2xx status code is healthy.
	return code >= 200 && code < 300
}

// parsedResponse holds extracted health fields from a parsed response body.
type parsedResponse struct {
	score   float64
	healthy bool
	message string
}

// parseJSONResponse extracts health signal fields from a JSON response body
// using the configured paths.
func (c *CustomHTTPCheck) parseJSONResponse(body []byte) (*parsedResponse, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}

	result := &parsedResponse{
		score:   1.0,
		healthy: true,
	}

	parser := c.config.ResponseParser

	// Extract score if path is configured.
	if parser.ScorePath != "" {
		val, err := extractJSONValue(data, parser.ScorePath)
		if err == nil {
			if num, ok := toFloat64(val); ok {
				result.score = num
			}
		}
	}

	// Extract healthy boolean if path is configured.
	if parser.HealthyPath != "" {
		val, err := extractJSONValue(data, parser.HealthyPath)
		if err == nil {
			if b, ok := val.(bool); ok {
				result.healthy = b
			}
		}
	}

	// Extract message if path is configured.
	if parser.MessagePath != "" {
		val, err := extractJSONValue(data, parser.MessagePath)
		if err == nil {
			if s, ok := val.(string); ok {
				result.message = s
			}
		}
	}

	return result, nil
}

// extractJSONValue traverses a JSON object using a dot-separated path and
// returns the value at the terminal key.
func extractJSONValue(data map[string]interface{}, path string) (interface{}, error) {
	keys := splitDotPath(path)
	var current interface{} = data

	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path element %q is not an object", key)
		}
		val, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("key %q not found", key)
		}
		current = val
	}

	return current, nil
}

// splitDotPath splits a dot-separated path into its components.
func splitDotPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// toFloat64 converts an interface value to float64 if possible.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
