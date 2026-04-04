package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// PagerDuty Events API v2 constants.
const (
	pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"
)

// PagerDuty event action types.
const (
	pdActionTrigger  = "trigger"
	pdActionResolve  = "resolve"
)

// PagerDuty severity levels.
const (
	pdSeverityCritical = "critical"
	pdSeverityWarning  = "warning"
	pdSeverityInfo     = "info"
)

// PagerDutyConfig holds configuration for the PagerDuty notification channel.
type PagerDutyConfig struct {
	// RoutingKey is the PagerDuty Events API v2 integration/routing key.
	RoutingKey string `json:"routing_key"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`

	// EventsURL overrides the default PagerDuty events endpoint (for testing).
	EventsURL string `json:"events_url,omitempty"`
}

// pdPayload is the PagerDuty Events API v2 request payload.
type pdPayload struct {
	RoutingKey  string       `json:"routing_key"`
	EventAction string       `json:"event_action"`
	DedupKey    string       `json:"dedup_key,omitempty"`
	Payload     *pdEventData `json:"payload,omitempty"`
}

// pdEventData holds the event details within a PagerDuty trigger action.
type pdEventData struct {
	Summary   string            `json:"summary"`
	Source    string            `json:"source"`
	Severity  string            `json:"severity"`
	Timestamp string            `json:"timestamp,omitempty"`
	Component string            `json:"component,omitempty"`
	Group     string            `json:"group,omitempty"`
	CustomDetails map[string]string `json:"custom_details,omitempty"`
}

// pdResponse is the PagerDuty Events API v2 response.
type pdResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	DedupKey string `json:"dedup_key"`
}

// PagerDutyChannel implements the Channel interface for triggering and
// resolving PagerDuty incidents via the Events API v2.
type PagerDutyChannel struct {
	config PagerDutyConfig
	client *http.Client

	// incidents tracks active incident dedup keys so we can auto-resolve.
	mu        sync.RWMutex
	incidents map[string]string // projectID -> dedupKey
}

// NewPagerDutyChannel creates a new PagerDuty notification channel.
func NewPagerDutyChannel(config PagerDutyConfig) *PagerDutyChannel {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.EventsURL == "" {
		config.EventsURL = pagerDutyEventsURL
	}
	return &PagerDutyChannel{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		incidents: make(map[string]string),
	}
}

// Name returns the channel identifier.
func (p *PagerDutyChannel) Name() string {
	return "pagerduty"
}

// Supports reports whether the PagerDuty channel handles the given event type.
// PagerDuty handles health degradation, health alerts, and deployment failures.
func (p *PagerDutyChannel) Supports(eventType EventType) bool {
	switch eventType {
	case EventHealthDegraded,
		EventHealthAlertTriggered,
		EventHealthAlertResolved,
		EventDeployFailed,
		EventDeployRollbackInitiated:
		return true
	default:
		return false
	}
}

// Send delivers a notification event to PagerDuty. For degradation and failure
// events, it triggers an incident. For resolution events, it resolves the
// corresponding incident.
func (p *PagerDutyChannel) Send(ctx context.Context, event *Event) error {
	switch event.Type {
	case EventHealthAlertResolved:
		return p.resolveIncident(ctx, event)
	default:
		return p.triggerIncident(ctx, event)
	}
}

// triggerIncident creates a PagerDuty incident for the given event.
func (p *PagerDutyChannel) triggerIncident(ctx context.Context, event *Event) error {
	project := event.Data["project_name"]
	if project == "" {
		project = event.ProjectID
	}

	dedupKey := fmt.Sprintf("deploysentry-%s-%s", event.ProjectID, event.Type)

	payload := &pdPayload{
		RoutingKey:  p.config.RoutingKey,
		EventAction: pdActionTrigger,
		DedupKey:    dedupKey,
		Payload: &pdEventData{
			Summary:   p.buildSummary(event),
			Source:    "deploysentry",
			Severity:  p.mapSeverity(event.Type),
			Timestamp: event.Timestamp.Format(time.RFC3339),
			Component: project,
			Group:     event.OrgID,
			CustomDetails: event.Data,
		},
	}

	resp, err := p.sendRequest(ctx, payload)
	if err != nil {
		return err
	}

	// Track the dedup key for auto-resolve.
	p.mu.Lock()
	p.incidents[event.ProjectID] = resp.DedupKey
	p.mu.Unlock()

	return nil
}

// resolveIncident resolves a previously triggered PagerDuty incident.
func (p *PagerDutyChannel) resolveIncident(ctx context.Context, event *Event) error {
	p.mu.RLock()
	dedupKey, exists := p.incidents[event.ProjectID]
	p.mu.RUnlock()

	if !exists {
		// Build the dedup key from convention if we don't have a tracked one.
		// This handles the case where the service restarted between trigger and resolve.
		dedupKey = fmt.Sprintf("deploysentry-%s-%s", event.ProjectID, EventHealthAlertTriggered)
	}

	payload := &pdPayload{
		RoutingKey:  p.config.RoutingKey,
		EventAction: pdActionResolve,
		DedupKey:    dedupKey,
	}

	_, err := p.sendRequest(ctx, payload)
	if err != nil {
		return err
	}

	// Remove the tracked incident.
	p.mu.Lock()
	delete(p.incidents, event.ProjectID)
	p.mu.Unlock()

	return nil
}

// sendRequest sends a payload to the PagerDuty Events API v2.
func (p *PagerDutyChannel) sendRequest(ctx context.Context, payload *pdPayload) (*pdResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling pagerduty payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.EventsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating pagerduty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending pagerduty event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pagerduty returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var pdResp pdResponse
	if err := json.Unmarshal(respBody, &pdResp); err != nil {
		return nil, fmt.Errorf("decoding pagerduty response: %w", err)
	}

	return &pdResp, nil
}

// buildSummary creates a human-readable summary for the PagerDuty incident.
func (p *PagerDutyChannel) buildSummary(event *Event) string {
	project := event.Data["project_name"]
	if project == "" {
		project = event.ProjectID
	}

	switch event.Type {
	case EventHealthDegraded:
		return fmt.Sprintf("Health degraded for %s: score %s", project, event.Data["score"])
	case EventHealthAlertTriggered:
		return fmt.Sprintf("Health alert triggered for %s: score %s", project, event.Data["score"])
	case EventDeployFailed:
		return fmt.Sprintf("Deployment FAILED for %s (version: %s): %s", project, event.Data["version"], event.Data["error"])
	case EventDeployRollbackInitiated:
		return fmt.Sprintf("Rollback initiated for %s (version: %s): %s", project, event.Data["version"], event.Data["reason"])
	default:
		return fmt.Sprintf("DeploySentry alert for %s: %s", project, event.Type)
	}
}

// mapSeverity maps a notification event type to a PagerDuty severity level.
func (p *PagerDutyChannel) mapSeverity(eventType EventType) string {
	switch eventType {
	case EventDeployFailed, EventHealthAlertTriggered:
		return pdSeverityCritical
	case EventHealthDegraded, EventDeployRollbackInitiated:
		return pdSeverityWarning
	default:
		return pdSeverityInfo
	}
}

// ActiveIncidents returns the number of currently tracked active incidents.
func (p *PagerDutyChannel) ActiveIncidents() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.incidents)
}
