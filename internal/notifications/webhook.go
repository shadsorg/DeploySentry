package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookConfig holds configuration for the webhook notification channel.
type WebhookConfig struct {
	// URL is the endpoint to deliver webhook payloads to.
	URL string `json:"url"`

	// Secret is the HMAC signing secret used to sign payloads.
	Secret string `json:"secret"`

	// MaxRetries is the maximum number of delivery attempts.
	MaxRetries int `json:"max_retries"`

	// RetryDelay is the base delay between retry attempts.
	// Actual delay doubles with each attempt (exponential backoff).
	RetryDelay time.Duration `json:"retry_delay"`

	// Timeout is the HTTP request timeout for each delivery attempt.
	Timeout time.Duration `json:"timeout"`

	// EventTypes filters which event types this webhook receives.
	// An empty slice means all events are delivered.
	EventTypes []EventType `json:"event_types,omitempty"`
}

// webhookPayload is the JSON payload delivered to webhook endpoints.
type webhookPayload struct {
	ID        string            `json:"id"`
	EventType EventType         `json:"event_type"`
	Timestamp time.Time         `json:"timestamp"`
	OrgID     string            `json:"org_id"`
	ProjectID string            `json:"project_id"`
	Data      map[string]string `json:"data"`
}

// DeliveryResult records the outcome of a webhook delivery attempt.
type DeliveryResult struct {
	Attempt      int       `json:"attempt"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `json:"response_body,omitempty"`
	Error        string    `json:"error,omitempty"`
	Success      bool      `json:"success"`
	DeliveredAt  time.Time `json:"delivered_at"`
}

// WebhookChannel implements the Channel interface for delivering notifications
// via HTTP webhooks with HMAC-SHA256 signing and exponential backoff retry.
type WebhookChannel struct {
	config WebhookConfig
	client *http.Client
}

// NewWebhookChannel creates a new webhook notification channel.
func NewWebhookChannel(config WebhookConfig) *WebhookChannel {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	return &WebhookChannel{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the channel identifier.
func (w *WebhookChannel) Name() string {
	return "webhook"
}

// Supports reports whether the webhook channel handles the given event type.
func (w *WebhookChannel) Supports(eventType EventType) bool {
	if len(w.config.EventTypes) == 0 {
		return true
	}
	for _, et := range w.config.EventTypes {
		if et == eventType {
			return true
		}
	}
	return false
}

// Send delivers a notification event to the configured webhook URL with
// HMAC-SHA256 signing and retry logic.
func (w *WebhookChannel) Send(ctx context.Context, event *Event) error {
	payload := &webhookPayload{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		EventType: event.Type,
		Timestamp: event.Timestamp,
		OrgID:     event.OrgID,
		ProjectID: event.ProjectID,
		Data:      event.Data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	signature := w.sign(body)

	var lastErr error
	delay := w.config.RetryDelay

	for attempt := 1; attempt <= w.config.MaxRetries; attempt++ {
		result := w.deliver(ctx, body, signature, attempt)
		if result.Success {
			return nil
		}

		lastErr = fmt.Errorf("attempt %d: status=%d error=%s", attempt, result.StatusCode, result.Error)

		// Don't retry on the last attempt.
		if attempt < w.config.MaxRetries {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			delay *= 2 // Exponential backoff.
		}
	}

	return fmt.Errorf("webhook delivery failed after %d attempts: %w", w.config.MaxRetries, lastErr)
}

// deliver performs a single webhook delivery attempt.
func (w *WebhookChannel) deliver(ctx context.Context, body []byte, signature string, attempt int) *DeliveryResult {
	result := &DeliveryResult{
		Attempt:     attempt,
		DeliveredAt: time.Now().UTC(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.URL, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DeploySentry-Signature", signature)
	req.Header.Set("X-DeploySentry-Delivery", fmt.Sprintf("%d", attempt))

	resp, err := w.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	result.ResponseBody = string(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// sign computes the HMAC-SHA256 signature of the payload using the configured secret.
func (w *WebhookChannel) sign(payload []byte) string {
	if w.config.Secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(w.config.Secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates an incoming webhook signature against the expected
// HMAC-SHA256 of the payload. This is a utility function for webhook consumers.
func VerifySignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
