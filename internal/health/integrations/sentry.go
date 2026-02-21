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

// SentryConfig holds configuration for the Sentry health check integration.
type SentryConfig struct {
	// BaseURL is the Sentry API base URL.
	BaseURL string `json:"base_url"`

	// AuthToken is the Sentry authentication token.
	AuthToken string `json:"auth_token"`

	// Organization is the Sentry organization slug.
	Organization string `json:"organization"`

	// Project is the Sentry project slug.
	Project string `json:"project"`

	// ErrorThreshold is the maximum acceptable new errors in the check window.
	ErrorThreshold int `json:"error_threshold"`

	// CheckWindow is how far back to look for errors.
	CheckWindow time.Duration `json:"check_window"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`
}

// DefaultSentryConfig returns a sensible default Sentry configuration.
func DefaultSentryConfig() SentryConfig {
	return SentryConfig{
		BaseURL:        "https://sentry.io/api/0",
		ErrorThreshold: 10,
		CheckWindow:    5 * time.Minute,
		Timeout:        10 * time.Second,
	}
}

// SentryCheck implements health.HealthCheck by querying the Sentry API for
// recent error counts to assess deployment health.
type SentryCheck struct {
	config SentryConfig
	client *http.Client
}

// NewSentryCheck creates a new Sentry health check with the given configuration.
func NewSentryCheck(config SentryConfig) *SentryCheck {
	return &SentryCheck{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the check identifier.
func (s *SentryCheck) Name() string {
	return "sentry"
}

// Check queries the Sentry API for recent issues and computes a health score
// based on the error count relative to the configured threshold.
func (s *SentryCheck) Check(ctx context.Context, deploymentID uuid.UUID) (*health.CheckResult, error) {
	result := &health.CheckResult{
		Name:      s.Name(),
		CheckedAt: time.Now().UTC(),
	}

	errorCount, err := s.getRecentErrorCount(ctx)
	if err != nil {
		result.Healthy = false
		result.Score = 0
		result.Message = fmt.Sprintf("failed to query sentry: %v", err)
		return result, nil
	}

	// Compute score based on threshold.
	score := 1.0
	if s.config.ErrorThreshold > 0 {
		score = 1.0 - (float64(errorCount) / float64(s.config.ErrorThreshold))
		if score < 0 {
			score = 0
		}
	}

	healthy := errorCount <= s.config.ErrorThreshold

	result.Healthy = healthy
	result.Score = score
	result.Message = fmt.Sprintf("recent_errors=%d threshold=%d", errorCount, s.config.ErrorThreshold)
	return result, nil
}

// sentryIssuesResponse models the JSON response from the Sentry issues endpoint.
type sentryIssuesResponse []struct {
	ID    string `json:"id"`
	Count string `json:"count"`
}

// getRecentErrorCount queries the Sentry API for the number of recent issues.
func (s *SentryCheck) getRecentErrorCount(ctx context.Context) (int, error) {
	endpoint := fmt.Sprintf(
		"%s/projects/%s/%s/issues/?statsPeriod=5m&query=is:unresolved",
		s.config.BaseURL, s.config.Organization, s.config.Project,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("creating sentry request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.AuthToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("querying sentry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sentry returned status %d", resp.StatusCode)
	}

	var issues sentryIssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return 0, fmt.Errorf("decoding sentry response: %w", err)
	}

	return len(issues), nil
}
