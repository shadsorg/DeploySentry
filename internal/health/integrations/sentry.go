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

// SentryWebhookEvent represents a Sentry webhook event payload.
type SentryWebhookEvent struct {
	Action string `json:"action"`
	Data   struct {
		Issue struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Level    string `json:"level"`
			Platform string `json:"platform"`
			Project  struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"project"`
			Count      string `json:"count"`
			IsNew      bool   `json:"isNew"`
			IsRegression bool `json:"isRegression"`
		} `json:"issue"`
	} `json:"data"`
}

// SentryWebhookResult holds the processed result of a Sentry webhook event.
type SentryWebhookResult struct {
	Action       string  `json:"action"`
	IssueID      string  `json:"issue_id"`
	Title        string  `json:"title"`
	Level        string  `json:"level"`
	Project      string  `json:"project"`
	IsNew        bool    `json:"is_new"`
	IsRegression bool    `json:"is_regression"`
	HealthScore  float64 `json:"health_score"`
}

// SentryWebhookReceiver processes incoming Sentry webhook events and converts
// them to health check signals.
type SentryWebhookReceiver struct {
	config   SentryConfig
	listener WebhookListener
}

// WebhookListener receives processed webhook events. Implementations can
// update health status, trigger alerts, or record audit data.
type WebhookListener interface {
	// OnWebhookEvent is called when a valid webhook event has been processed.
	OnWebhookEvent(ctx context.Context, result *SentryWebhookResult)
}

// NewSentryWebhookReceiver creates a new SentryWebhookReceiver with the given
// configuration and optional listener.
func NewSentryWebhookReceiver(config SentryConfig, listener WebhookListener) *SentryWebhookReceiver {
	return &SentryWebhookReceiver{
		config:   config,
		listener: listener,
	}
}

// HandleWebhook processes an incoming Sentry webhook HTTP request. It parses
// the webhook payload, computes a health signal score based on the event, and
// notifies the registered listener.
func (r *SentryWebhookReceiver) HandleWebhook(ctx context.Context, req *http.Request) (*SentryWebhookResult, error) {
	if req.Method != http.MethodPost {
		return nil, fmt.Errorf("unsupported method %s, expected POST", req.Method)
	}

	var event SentryWebhookEvent
	if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
		return nil, fmt.Errorf("decoding sentry webhook payload: %w", err)
	}

	// Compute a health score based on the event type.
	// New issues and regressions have a bigger impact on health.
	var score float64
	switch {
	case event.Data.Issue.IsRegression:
		score = 0.2
	case event.Data.Issue.IsNew && event.Data.Issue.Level == "error":
		score = 0.3
	case event.Data.Issue.IsNew:
		score = 0.5
	case event.Action == "resolved":
		score = 1.0
	default:
		score = 0.6
	}

	result := &SentryWebhookResult{
		Action:       event.Action,
		IssueID:      event.Data.Issue.ID,
		Title:        event.Data.Issue.Title,
		Level:        event.Data.Issue.Level,
		Project:      event.Data.Issue.Project.Slug,
		IsNew:        event.Data.Issue.IsNew,
		IsRegression: event.Data.Issue.IsRegression,
		HealthScore:  score,
	}

	if r.listener != nil {
		r.listener.OnWebhookEvent(ctx, result)
	}

	return result, nil
}
