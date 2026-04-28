// Package integrations provides health check implementations that connect
// to external observability platforms.
package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shadsorg/deploysentry/internal/health"
	"github.com/google/uuid"
)

// PrometheusConfig holds configuration for the Prometheus health check integration.
type PrometheusConfig struct {
	// BaseURL is the Prometheus server URL (e.g., http://prometheus:9090).
	BaseURL string `json:"base_url"`

	// ErrorRateQuery is a PromQL query that returns the error rate.
	// It should return a single scalar or vector with one element.
	ErrorRateQuery string `json:"error_rate_query"`

	// LatencyQuery is a PromQL query that returns the p99 latency in seconds.
	LatencyQuery string `json:"latency_query"`

	// ErrorRateThreshold is the maximum acceptable error rate (0.0-1.0).
	ErrorRateThreshold float64 `json:"error_rate_threshold"`

	// LatencyThreshold is the maximum acceptable p99 latency in seconds.
	LatencyThreshold float64 `json:"latency_threshold"`

	// Timeout is the HTTP request timeout for querying Prometheus.
	Timeout time.Duration `json:"timeout"`
}

// DefaultPrometheusConfig returns a sensible default Prometheus configuration.
func DefaultPrometheusConfig() PrometheusConfig {
	return PrometheusConfig{
		BaseURL:            "http://localhost:9090",
		ErrorRateQuery:     `rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])`,
		LatencyQuery:       `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))`,
		ErrorRateThreshold: 0.01,
		LatencyThreshold:   1.0,
		Timeout:            10 * time.Second,
	}
}

// PrometheusCheck implements health.HealthCheck by querying Prometheus metrics
// to determine the health of a deployment.
type PrometheusCheck struct {
	config PrometheusConfig
	client *http.Client
}

// NewPrometheusCheck creates a new Prometheus health check with the given configuration.
func NewPrometheusCheck(config PrometheusConfig) *PrometheusCheck {
	return &PrometheusCheck{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the check identifier.
func (p *PrometheusCheck) Name() string {
	return "prometheus"
}

// Check queries Prometheus for error rate and latency metrics, computing
// a health score based on whether they fall within configured thresholds.
func (p *PrometheusCheck) Check(ctx context.Context, deploymentID uuid.UUID) (*health.CheckResult, error) {
	result := &health.CheckResult{
		Name:      p.Name(),
		CheckedAt: time.Now().UTC(),
	}

	// Query error rate.
	errorRate, err := p.queryScalar(ctx, p.config.ErrorRateQuery)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to query error rate: %v", err)
		return result, nil
	}

	// Query latency.
	latency, err := p.queryScalar(ctx, p.config.LatencyQuery)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to query latency: %v", err)
		return result, nil
	}

	// Compute score based on thresholds.
	errorScore := 1.0
	if p.config.ErrorRateThreshold > 0 {
		errorScore = 1.0 - (errorRate / p.config.ErrorRateThreshold)
		if errorScore < 0 {
			errorScore = 0
		}
	}

	latencyScore := 1.0
	if p.config.LatencyThreshold > 0 {
		latencyScore = 1.0 - (latency / p.config.LatencyThreshold)
		if latencyScore < 0 {
			latencyScore = 0
		}
	}

	// Overall score is the average of error rate and latency scores.
	score := (errorScore + latencyScore) / 2.0
	healthy := errorRate <= p.config.ErrorRateThreshold && latency <= p.config.LatencyThreshold

	result.Healthy = healthy
	result.Score = score
	result.Message = fmt.Sprintf("error_rate=%.4f latency_p99=%.3fs", errorRate, latency)
	result.Metrics = map[string]float64{
		"error_rate":     errorRate,
		"latency_p99_ms": latency * 1000,
	}
	return result, nil
}

// prometheusResponse models the JSON response from the Prometheus /api/v1/query endpoint.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// queryScalar executes a PromQL instant query and returns the scalar result.
func (p *PrometheusCheck) queryScalar(ctx context.Context, query string) (float64, error) {
	u, err := url.Parse(p.config.BaseURL + "/api/v1/query")
	if err != nil {
		return 0, fmt.Errorf("parsing prometheus URL: %w", err)
	}

	params := url.Values{}
	params.Set("query", query)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("querying prometheus: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var promResp prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return 0, fmt.Errorf("decoding prometheus response: %w", err)
	}

	if promResp.Status != "success" {
		return 0, fmt.Errorf("prometheus query failed with status: %s", promResp.Status)
	}

	if len(promResp.Data.Result) == 0 {
		return 0, nil
	}

	values := promResp.Data.Result[0].Value
	if len(values) < 2 {
		return 0, fmt.Errorf("unexpected prometheus result format")
	}

	var valueStr string
	if err := json.Unmarshal(values[1], &valueStr); err != nil {
		return 0, fmt.Errorf("parsing prometheus value: %w", err)
	}

	var value float64
	_, err = fmt.Sscanf(valueStr, "%f", &value)
	if err != nil {
		return 0, fmt.Errorf("converting prometheus value to float: %w", err)
	}

	return value, nil
}
