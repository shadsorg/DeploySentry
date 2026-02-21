// Package notifications implements event-driven notification dispatching
// to channels such as Slack, webhooks, and email.
package notifications

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// EventType identifies the category of event that triggered a notification.
type EventType string

const (
	// EventDeployStarted is emitted when a deployment begins.
	EventDeployStarted EventType = "deployment.started"
	// EventDeployCreated is emitted when a deployment is created.
	EventDeployCreated EventType = "deployment.created"
	// EventDeployPhaseCompleted is emitted when a deployment phase completes.
	EventDeployPhaseCompleted EventType = "deployment.phase.completed"
	// EventDeployCompleted is emitted when a deployment succeeds.
	EventDeployCompleted EventType = "deployment.completed"
	// EventDeployFailed is emitted when a deployment fails.
	EventDeployFailed EventType = "deployment.failed"
	// EventDeployRolledBack is emitted when a deployment is rolled back.
	EventDeployRolledBack EventType = "deployment.rolled_back"
	// EventDeployRollbackInitiated is emitted when a rollback is initiated.
	EventDeployRollbackInitiated EventType = "deployment.rollback.initiated"
	// EventDeployRollbackCompleted is emitted when a rollback completes.
	EventDeployRollbackCompleted EventType = "deployment.rollback.completed"

	// EventFlagCreated is emitted when a feature flag is created.
	EventFlagCreated EventType = "flag.created"
	// EventFlagUpdated is emitted when a feature flag is updated.
	EventFlagUpdated EventType = "flag.updated"
	// EventFlagToggled is emitted when a feature flag is toggled.
	EventFlagToggled EventType = "flag.toggled"
	// EventFlagArchived is emitted when a feature flag is archived.
	EventFlagArchived EventType = "flag.archived"

	// EventReleaseCreated is emitted when a new release is created.
	EventReleaseCreated EventType = "release.created"
	// EventReleasePromoted is emitted when a release is promoted.
	EventReleasePromoted EventType = "release.promoted"
	// EventReleaseHealthDegraded is emitted when a release health degrades.
	EventReleaseHealthDegraded EventType = "release.health.degraded"

	// EventHealthDegraded is emitted when deployment health degrades.
	EventHealthDegraded EventType = "health.degraded"
	// EventHealthAlertTriggered is emitted when a health alert is triggered.
	EventHealthAlertTriggered EventType = "health.alert.triggered"
	// EventHealthAlertResolved is emitted when a health alert is resolved.
	EventHealthAlertResolved EventType = "health.alert.resolved"
)

// Event represents a notification event with contextual payload data.
type Event struct {
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	OrgID     string            `json:"org_id"`
	ProjectID string            `json:"project_id"`
	Data      map[string]string `json:"data"`
}

// Channel defines the interface for a notification delivery channel.
type Channel interface {
	// Name returns the channel identifier.
	Name() string

	// Send delivers a notification event through this channel.
	Send(ctx context.Context, event *Event) error

	// Supports reports whether this channel handles the given event type.
	Supports(eventType EventType) bool
}

// RetryConfig controls the retry behavior for notification delivery.
type RetryConfig struct {
	// MaxRetries is the maximum number of delivery attempts per channel.
	// A value of 0 means no retries (single attempt only).
	MaxRetries int `json:"max_retries"`

	// BaseDelay is the initial delay before the first retry.
	BaseDelay time.Duration `json:"base_delay"`

	// MaxDelay is the maximum delay between retry attempts.
	MaxDelay time.Duration `json:"max_delay"`
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// NotificationService listens for domain events and dispatches notifications
// to all registered channels that support the event type.
type NotificationService struct {
	mu       sync.RWMutex
	channels []Channel
	retry    RetryConfig
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

// NewNotificationServiceWithRetry creates a new NotificationService with
// the given retry configuration for delivery attempts.
func NewNotificationServiceWithRetry(cfg RetryConfig) *NotificationService {
	return &NotificationService{retry: cfg}
}

// RegisterChannel adds a notification channel to the service.
func (s *NotificationService) RegisterChannel(ch Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = append(s.channels, ch)
}

// Dispatch sends an event to all registered channels that support the event type.
// Errors from individual channels are collected but do not prevent delivery
// to other channels. If a RetryConfig is set, failed deliveries are retried
// with exponential backoff.
func (s *NotificationService) Dispatch(ctx context.Context, event *Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	s.mu.RLock()
	channels := make([]Channel, len(s.channels))
	copy(channels, s.channels)
	s.mu.RUnlock()

	var errs []error
	for _, ch := range channels {
		if !ch.Supports(event.Type) {
			continue
		}
		if err := s.sendWithRetry(ctx, ch, event); err != nil {
			errs = append(errs, fmt.Errorf("channel %s: %w", ch.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification dispatch errors: %v", errs)
	}
	return nil
}

// sendWithRetry attempts to send an event through a channel, retrying with
// exponential backoff on failure according to the service's RetryConfig.
func (s *NotificationService) sendWithRetry(ctx context.Context, ch Channel, event *Event) error {
	maxAttempts := 1 + s.retry.MaxRetries
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = ch.Send(ctx, event)
		if lastErr == nil {
			return nil
		}

		// Don't wait after the last attempt.
		if attempt < maxAttempts-1 && s.retry.BaseDelay > 0 {
			delay := s.calculateDelay(attempt)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return fmt.Errorf("delivery failed after %d attempts: %w", maxAttempts, lastErr)
}

// calculateDelay returns the backoff delay for the given attempt index.
// The delay doubles with each attempt but is capped at MaxDelay.
func (s *NotificationService) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(s.retry.BaseDelay) * math.Pow(2, float64(attempt)))
	if s.retry.MaxDelay > 0 && delay > s.retry.MaxDelay {
		delay = s.retry.MaxDelay
	}
	return delay
}

// DispatchAsync sends an event to all registered channels asynchronously.
// It returns immediately and processes deliveries in the background.
func (s *NotificationService) DispatchAsync(ctx context.Context, event *Event) {
	go func() {
		_ = s.Dispatch(ctx, event)
	}()
}
