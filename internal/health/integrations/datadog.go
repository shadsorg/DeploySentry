package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/google/uuid"
)

// DatadogConfig holds configuration for the Datadog health check integration.
type DatadogConfig struct {
	// APIKey is the Datadog API key.
	APIKey string `json:"api_key"`

	// AppKey is the Datadog application key.
	AppKey string `json:"app_key"`

	// Site is the Datadog site (e.g., datadoghq.com, datadoghq.eu).
	Site string `json:"site"`

	// ErrorRateMetric is the Datadog metric name for error rate.
	ErrorRateMetric string `json:"error_rate_metric"`

	// LatencyMetric is the Datadog metric name for latency.
	LatencyMetric string `json:"latency_metric"`

	// ErrorRateThreshold is the maximum acceptable error rate.
	ErrorRateThreshold float64 `json:"error_rate_threshold"`

	// LatencyThreshold is the maximum acceptable latency in seconds.
	LatencyThreshold float64 `json:"latency_threshold"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`
}

// DefaultDatadogConfig returns a sensible default Datadog configuration.
func DefaultDatadogConfig() DatadogConfig {
	return DatadogConfig{
		Site:               "datadoghq.com",
		ErrorRateMetric:    "http.server.errors",
		LatencyMetric:      "http.server.latency.p99",
		ErrorRateThreshold: 0.01,
		LatencyThreshold:   1.0,
		Timeout:            10 * time.Second,
	}
}

// DatadogCheck implements health.HealthCheck by querying Datadog metrics
// to determine deployment health.
type DatadogCheck struct {
	config DatadogConfig
	client *http.Client
}

// NewDatadogCheck creates a new Datadog health check with the given configuration.
func NewDatadogCheck(config DatadogConfig) *DatadogCheck {
	return &DatadogCheck{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the check identifier.
func (d *DatadogCheck) Name() string {
	return "datadog"
}

// Check queries the Datadog API for error rate and latency metrics, computing
// a health score based on whether they fall within configured thresholds.
func (d *DatadogCheck) Check(ctx context.Context, deploymentID uuid.UUID) (*health.CheckResult, error) {
	result := &health.CheckResult{
		Name:      d.Name(),
		CheckedAt: time.Now().UTC(),
	}

	errorRate, err := d.queryMetric(ctx, d.config.ErrorRateMetric)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to query datadog error rate: %v", err)
		return result, nil
	}

	latency, err := d.queryMetric(ctx, d.config.LatencyMetric)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to query datadog latency: %v", err)
		return result, nil
	}

	// Compute score based on thresholds.
	errorScore := 1.0
	if d.config.ErrorRateThreshold > 0 {
		errorScore = 1.0 - (errorRate / d.config.ErrorRateThreshold)
		if errorScore < 0 {
			errorScore = 0
		}
	}

	latencyScore := 1.0
	if d.config.LatencyThreshold > 0 {
		latencyScore = 1.0 - (latency / d.config.LatencyThreshold)
		if latencyScore < 0 {
			latencyScore = 0
		}
	}

	score := (errorScore + latencyScore) / 2.0
	healthy := errorRate <= d.config.ErrorRateThreshold && latency <= d.config.LatencyThreshold

	result.Healthy = healthy
	result.Score = score
	result.Message = fmt.Sprintf("error_rate=%.4f latency=%.3fs", errorRate, latency)
	return result, nil
}

// datadogQueryResponse models the JSON response from the Datadog metrics query endpoint.
type datadogQueryResponse struct {
	Series []struct {
		Pointlist [][]float64 `json:"pointlist"`
	} `json:"series"`
}

// queryMetric queries a Datadog metric and returns the most recent value.
func (d *DatadogCheck) queryMetric(ctx context.Context, metric string) (float64, error) {
	now := time.Now().Unix()
	from := now - 300 // Last 5 minutes.

	endpoint := fmt.Sprintf(
		"https://api.%s/api/v1/query?from=%d&to=%d&query=%s",
		d.config.Site, from, now, metric,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("creating datadog request: %w", err)
	}

	req.Header.Set("DD-API-KEY", d.config.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", d.config.AppKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("querying datadog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("datadog returned status %d", resp.StatusCode)
	}

	var ddResp datadogQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddResp); err != nil {
		return 0, fmt.Errorf("decoding datadog response: %w", err)
	}

	if len(ddResp.Series) == 0 || len(ddResp.Series[0].Pointlist) == 0 {
		return 0, nil
	}

	// Return the most recent data point.
	points := ddResp.Series[0].Pointlist
	lastPoint := points[len(points)-1]
	if len(lastPoint) < 2 {
		return 0, fmt.Errorf("unexpected datadog point format")
	}

	return lastPoint[1], nil
}

// DatadogMonitorStatus represents the status of a Datadog monitor.
type DatadogMonitorStatus struct {
	MonitorID int    `json:"monitor_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Message   string `json:"message"`
}

// datadogMonitorResponse models the JSON response from the Datadog monitor endpoint.
type datadogMonitorResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"overall_state"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetMonitorStatus queries the Datadog API for the status of a specific monitor.
// Monitor statuses include: "OK", "Alert", "Warn", "No Data".
func (d *DatadogCheck) GetMonitorStatus(ctx context.Context, monitorID int) (*DatadogMonitorStatus, error) {
	endpoint := fmt.Sprintf(
		"https://api.%s/api/v1/monitor/%d",
		d.config.Site, monitorID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating datadog monitor request: %w", err)
	}

	req.Header.Set("DD-API-KEY", d.config.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", d.config.AppKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying datadog monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datadog returned status %d for monitor %d", resp.StatusCode, monitorID)
	}

	var monitorResp datadogMonitorResponse
	if err := json.NewDecoder(resp.Body).Decode(&monitorResp); err != nil {
		return nil, fmt.Errorf("decoding datadog monitor response: %w", err)
	}

	return &DatadogMonitorStatus{
		MonitorID: monitorResp.ID,
		Name:      monitorResp.Name,
		Status:    monitorResp.Status,
		Type:      monitorResp.Type,
		Message:   monitorResp.Message,
	}, nil
}

// MonitorStatusToScore converts a Datadog monitor status string to a health score
// in the range [0.0, 1.0].
func MonitorStatusToScore(status string) float64 {
	switch status {
	case "OK":
		return 1.0
	case "Warn":
		return 0.5
	case "Alert":
		return 0.0
	case "No Data":
		return 0.5
	default:
		return 0.0
	}
}
