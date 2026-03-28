package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReleaseStatus represents the lifecycle state of a release.
type ReleaseStatus string

const (
	// ReleaseDraft indicates the release is being prepared.
	ReleaseDraft ReleaseStatus = "draft"
	// ReleaseRollingOut indicates the release is actively rolling out.
	ReleaseRollingOut ReleaseStatus = "rolling_out"
	// ReleasePaused indicates the release has been temporarily paused.
	ReleasePaused ReleaseStatus = "paused"
	// ReleaseCompleted indicates the release has been fully deployed.
	ReleaseCompleted ReleaseStatus = "completed"
	// ReleaseRolledBack indicates the release was rolled back.
	ReleaseRolledBack ReleaseStatus = "rolled_back"
)

// releaseTransitions defines which release status transitions are allowed.
var releaseTransitions = map[ReleaseStatus][]ReleaseStatus{
	ReleaseDraft:      {ReleaseRollingOut},
	ReleaseRollingOut: {ReleasePaused, ReleaseCompleted, ReleaseRolledBack},
	ReleasePaused:     {ReleaseRollingOut, ReleaseRolledBack},
}

// Release represents a bundle of flag changes that can be rolled out together.
type Release struct {
	ID             uuid.UUID     `json:"id"`
	ApplicationID  uuid.UUID     `json:"application_id"`
	Name           string        `json:"name"`
	Description    string        `json:"description,omitempty"`
	SessionSticky  bool          `json:"session_sticky"`
	StickyHeader   string        `json:"sticky_header,omitempty"`
	TrafficPercent int           `json:"traffic_percent"`
	Status         ReleaseStatus `json:"status"`
	CreatedBy      *uuid.UUID    `json:"created_by,omitempty"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	CompletedAt    *time.Time    `json:"completed_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// Validate checks that the Release has all required fields populated.
func (r *Release) Validate() error {
	if r.ApplicationID == uuid.Nil {
		return errors.New("application_id is required")
	}
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.SessionSticky && r.StickyHeader == "" {
		return errors.New("sticky_header is required when session_sticky is true")
	}
	if r.TrafficPercent < 0 || r.TrafficPercent > 100 {
		return errors.New("traffic_percent must be between 0 and 100")
	}
	return nil
}

// TransitionTo attempts to move the release to the target status.
// It validates the transition and updates the status if allowed.
func (r *Release) TransitionTo(newStatus ReleaseStatus) error {
	allowed, ok := releaseTransitions[r.Status]
	if !ok {
		return fmt.Errorf("no transitions from terminal status %q", r.Status)
	}
	for _, s := range allowed {
		if s == newStatus {
			r.Status = newStatus
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", r.Status, newStatus)
}
