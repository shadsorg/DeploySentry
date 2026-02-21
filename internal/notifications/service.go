// Package notifications implements event-driven notification dispatching
// to channels such as Slack, webhooks, and email.
package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType identifies the category of event that triggered a notification.
type EventType string

const (
	// EventDeployStarted is emitted when a deployment begins.
	EventDeployStarted EventType = "deployment.started"
	// EventDeployCompleted is emitted when a deployment succeeds.
	EventDeployCompleted EventType = "deployment.completed"
	// EventDeployFailed is emitted when a deployment fails.
	EventDeployFailed EventType = "deployment.failed"
	// EventDeployRolledBack is emitted when a deployment is rolled back.
	EventDeployRolledBack EventType = "deployment.rolled_back"
	// EventFlagToggled is emitted when a feature flag is toggled.
	EventFlagToggled EventType = "flag.toggled"
	// EventReleaseCreated is emitted when a new release is created.
	EventReleaseCreated EventType = "release.created"
	// EventReleasePromoted is emitted when a release is promoted.
	EventReleasePromoted EventType = "release.promoted"
	// EventHealthDegraded is emitted when deployment health degrades.
	EventHealthDegraded EventType = "health.degraded"
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

// NotificationService listens for domain events and dispatches notifications
// to all registered channels that support the event type.
type NotificationService struct {
	mu       sync.RWMutex
	channels []Channel
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

// RegisterChannel adds a notification channel to the service.
func (s *NotificationService) RegisterChannel(ch Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = append(s.channels, ch)
}

// Dispatch sends an event to all registered channels that support the event type.
// Errors from individual channels are collected but do not prevent delivery
// to other channels.
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
		if err := ch.Send(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("channel %s: %w", ch.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification dispatch errors: %v", errs)
	}
	return nil
}

// DispatchAsync sends an event to all registered channels asynchronously.
// It returns immediately and processes deliveries in the background.
func (s *NotificationService) DispatchAsync(ctx context.Context, event *Event) {
	go func() {
		_ = s.Dispatch(ctx, event)
	}()
}
