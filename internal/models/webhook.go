package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Webhook represents a webhook endpoint configuration.
type Webhook struct {
	ID              uuid.UUID      `json:"id" db:"id"`
	OrgID           uuid.UUID      `json:"org_id" db:"org_id"`
	ProjectID       *uuid.UUID     `json:"project_id,omitempty" db:"project_id"`
	Name            string         `json:"name" db:"name"`
	URL             string         `json:"url" db:"url"`
	Secret          string         `json:"-" db:"secret"` // Hidden from JSON for security
	Encrypted       bool           `json:"-" db:"encrypted"`
	Events          pq.StringArray `json:"events" db:"events"`
	IsActive        bool           `json:"is_active" db:"is_active"`
	RetryAttempts   int            `json:"retry_attempts" db:"retry_attempts"`
	TimeoutSeconds  int            `json:"timeout_seconds" db:"timeout_seconds"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
	CreatedBy       *uuid.UUID     `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy       *uuid.UUID     `json:"updated_by,omitempty" db:"updated_by"`
}

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	WebhookID    uuid.UUID              `json:"webhook_id" db:"webhook_id"`
	EventType    string                 `json:"event_type" db:"event_type"`
	Payload      map[string]interface{} `json:"payload" db:"payload"`
	Status       DeliveryStatus         `json:"status" db:"status"`
	HTTPStatus   *int                   `json:"http_status,omitempty" db:"http_status"`
	ResponseBody *string                `json:"response_body,omitempty" db:"response_body"`
	ErrorMessage *string                `json:"error_message,omitempty" db:"error_message"`
	AttemptCount int                    `json:"attempt_count" db:"attempt_count"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	SentAt       *time.Time             `json:"sent_at,omitempty" db:"sent_at"`
	NextRetryAt  *time.Time             `json:"next_retry_at,omitempty" db:"next_retry_at"`
}

// DeliveryStatus represents the status of a webhook delivery attempt.
type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusCancelled DeliveryStatus = "cancelled"
)

// WebhookEvent represents different types of events that can trigger webhooks.
type WebhookEvent string

const (
	// Flag events
	EventFlagCreated   WebhookEvent = "flag.created"
	EventFlagUpdated   WebhookEvent = "flag.updated"
	EventFlagToggled   WebhookEvent = "flag.toggled"
	EventFlagArchived  WebhookEvent = "flag.archived"
	EventFlagEvaluated WebhookEvent = "flag.evaluated"

	// Flag lifecycle events — emitted by the CrowdSoft feature-agent flow.
	// Payloads share a stable schema (see docs/Feature_Lifecycle.md) because
	// both the agent and the CrowdSoft portal consume them.
	EventFlagSmokeTestPassed              WebhookEvent = "flag.smoke_test.passed"
	EventFlagSmokeTestFailed              WebhookEvent = "flag.smoke_test.failed"
	EventFlagUserTestPassed               WebhookEvent = "flag.user_test.passed"
	EventFlagUserTestFailed               WebhookEvent = "flag.user_test.failed"
	EventFlagScheduledForRemovalSet       WebhookEvent = "flag.scheduled_for_removal.set"
	EventFlagScheduledForRemovalCancelled WebhookEvent = "flag.scheduled_for_removal.cancelled"
	EventFlagScheduledForRemovalDue       WebhookEvent = "flag.scheduled_for_removal.due"
	EventFlagIterationExhausted           WebhookEvent = "flag.iteration_exhausted"

	// Deployment events
	EventDeploymentCreated    WebhookEvent = "deployment.created"
	EventDeploymentRecorded   WebhookEvent = "deployment.recorded"
	EventDeploymentStarted    WebhookEvent = "deployment.started"
	EventDeploymentCompleted  WebhookEvent = "deployment.completed"
	EventDeploymentFailed     WebhookEvent = "deployment.failed"
	EventDeploymentRolledback WebhookEvent = "deployment.rolledback"
	EventDeploymentPaused        WebhookEvent = "deployment.paused"
	EventDeploymentResumed       WebhookEvent = "deployment.resumed"
	EventDeploymentPhaseChanged  WebhookEvent = "deployment.phase_changed"

	// Release events
	EventReleaseCreated  WebhookEvent = "release.created"
	EventReleasePromoted WebhookEvent = "release.promoted"

	// System events
	EventSystemAlert    WebhookEvent = "system.alert"
	EventSystemRecovery WebhookEvent = "system.recovery"

	// Audit events
	EventAuditLog WebhookEvent = "audit.log"
)

// AllWebhookEvents returns all available webhook event types.
func AllWebhookEvents() []WebhookEvent {
	return []WebhookEvent{
		EventFlagCreated,
		EventFlagUpdated,
		EventFlagToggled,
		EventFlagArchived,
		EventFlagEvaluated,
		EventFlagSmokeTestPassed,
		EventFlagSmokeTestFailed,
		EventFlagUserTestPassed,
		EventFlagUserTestFailed,
		EventFlagScheduledForRemovalSet,
		EventFlagScheduledForRemovalCancelled,
		EventFlagScheduledForRemovalDue,
		EventFlagIterationExhausted,
		EventDeploymentCreated,
		EventDeploymentRecorded,
		EventDeploymentStarted,
		EventDeploymentCompleted,
		EventDeploymentFailed,
		EventDeploymentRolledback,
		EventDeploymentPaused,
		EventDeploymentResumed,
		EventDeploymentPhaseChanged,
		EventReleaseCreated,
		EventReleasePromoted,
		EventSystemAlert,
		EventSystemRecovery,
		EventAuditLog,
	}
}

// CreateWebhookRequest represents a request to create a new webhook.
type CreateWebhookRequest struct {
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	Name           string     `json:"name" validate:"required,min=1,max=255"`
	URL            string     `json:"url" validate:"required,url"`
	Events         []string   `json:"events" validate:"required,min=1"`
	RetryAttempts  *int       `json:"retry_attempts,omitempty" validate:"omitempty,min=0,max=10"`
	TimeoutSeconds *int       `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=300"`
}

// UpdateWebhookRequest represents a request to update an existing webhook.
type UpdateWebhookRequest struct {
	Name           *string  `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	URL            *string  `json:"url,omitempty" validate:"omitempty,url"`
	Events         []string `json:"events,omitempty" validate:"omitempty,min=1"`
	IsActive       *bool    `json:"is_active,omitempty"`
	RetryAttempts  *int     `json:"retry_attempts,omitempty" validate:"omitempty,min=0,max=10"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=300"`
}

// WebhookTestRequest represents a request to test a webhook.
type WebhookTestRequest struct {
	EventType string                 `json:"event_type" validate:"required"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// WebhookListOptions represents options for listing webhooks.
type WebhookListOptions struct {
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	IsActive  *bool      `json:"is_active,omitempty"`
	Events    []string   `json:"events,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// WebhookDeliveryListOptions represents options for listing webhook deliveries.
type WebhookDeliveryListOptions struct {
	WebhookID *uuid.UUID      `json:"webhook_id,omitempty"`
	Status    *DeliveryStatus `json:"status,omitempty"`
	EventType *string         `json:"event_type,omitempty"`
	Since     *time.Time      `json:"since,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Offset    int             `json:"offset,omitempty"`
}

// WebhookEventPayload represents the structure of webhook event payloads.
type WebhookEventPayload struct {
	Event       WebhookEvent           `json:"event"`
	Timestamp   time.Time              `json:"timestamp"`
	OrgID       uuid.UUID              `json:"org_id"`
	ProjectID   *uuid.UUID             `json:"project_id,omitempty"`
	Data        map[string]interface{} `json:"data"`
	Environment *string                `json:"environment,omitempty"`
	UserID      *uuid.UUID             `json:"user_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookSignature represents the signature header for webhook verification.
type WebhookSignature struct {
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

// IsValidEvent checks if the given event type is valid.
func IsValidEvent(event string) bool {
	for _, validEvent := range AllWebhookEvents() {
		if string(validEvent) == event {
			return true
		}
	}
	return false
}

// ShouldRetry determines if a delivery should be retried based on the status and attempt count.
func (d *WebhookDelivery) ShouldRetry(maxAttempts int) bool {
	return d.Status == DeliveryStatusFailed && d.AttemptCount < maxAttempts
}

// CalculateNextRetryDelay calculates the delay before the next retry using exponential backoff.
func (d *WebhookDelivery) CalculateNextRetryDelay() time.Duration {
	// Exponential backoff: 2^attempt_count minutes, max 60 minutes
	delay := time.Duration(1<<uint(d.AttemptCount)) * time.Minute
	if delay > 60*time.Minute {
		delay = 60 * time.Minute
	}
	return delay
}

// SetNextRetryAt sets the next retry timestamp based on the calculated delay.
func (d *WebhookDelivery) SetNextRetryAt() {
	delay := d.CalculateNextRetryDelay()
	nextRetry := time.Now().Add(delay)
	d.NextRetryAt = &nextRetry
}

// Validate checks that the webhook has all required fields populated.
func (w *Webhook) Validate() error {
	if w.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if w.ProjectID == nil || *w.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if w.URL == "" {
		return errors.New("url is required")
	}
	if len(w.Events) == 0 {
		return errors.New("at least one event type is required")
	}
	return nil
}

// HasEvent checks if the webhook is configured to listen for the given event.
func (w *Webhook) HasEvent(event WebhookEvent) bool {
	for _, e := range w.Events {
		if e == string(event) {
			return true
		}
	}
	return false
}
