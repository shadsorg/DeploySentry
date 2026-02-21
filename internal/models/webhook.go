package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// WebhookEndpoint represents a configured webhook URL that receives event notifications.
type WebhookEndpoint struct {
	ID           uuid.UUID `json:"id" db:"id"`
	OrgID        uuid.UUID `json:"org_id" db:"org_id"`
	ProjectID    uuid.UUID `json:"project_id" db:"project_id"`
	URL          string    `json:"url" db:"url"`
	Secret       string    `json:"-" db:"secret"`
	Description  string    `json:"description,omitempty" db:"description"`
	Active       bool      `json:"active" db:"active"`
	EventTypes   []string  `json:"event_types" db:"-"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// WebhookDelivery records an individual attempt to deliver a webhook payload
// to an endpoint.
type WebhookDelivery struct {
	ID           uuid.UUID `json:"id" db:"id"`
	EndpointID   uuid.UUID `json:"endpoint_id" db:"endpoint_id"`
	EventID      uuid.UUID `json:"event_id" db:"event_id"`
	URL          string    `json:"url" db:"url"`
	RequestBody  string    `json:"request_body" db:"request_body"`
	ResponseCode int       `json:"response_code" db:"response_code"`
	ResponseBody string    `json:"response_body,omitempty" db:"response_body"`
	Success      bool      `json:"success" db:"success"`
	Attempt      int       `json:"attempt" db:"attempt"`
	Error        string    `json:"error,omitempty" db:"error"`
	DeliveredAt  time.Time `json:"delivered_at" db:"delivered_at"`
}

// WebhookEvent represents an event that triggers webhook deliveries.
type WebhookEvent struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	ProjectID uuid.UUID `json:"project_id" db:"project_id"`
	EventType string    `json:"event_type" db:"event_type"`
	Payload   string    `json:"payload" db:"payload"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Validate checks that the WebhookEndpoint has all required fields populated.
func (w *WebhookEndpoint) Validate() error {
	if w.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if w.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if w.URL == "" {
		return errors.New("url is required")
	}
	if len(w.EventTypes) == 0 {
		return errors.New("at least one event type is required")
	}
	return nil
}
