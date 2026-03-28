package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ReleaseFlagChange tracks an individual flag change within a release.
type ReleaseFlagChange struct {
	ID              uuid.UUID        `json:"id"`
	ReleaseID       uuid.UUID        `json:"release_id"`
	FlagID          uuid.UUID        `json:"flag_id"`
	EnvironmentID   uuid.UUID        `json:"environment_id"`
	PreviousValue   *json.RawMessage `json:"previous_value,omitempty"`
	NewValue        *json.RawMessage `json:"new_value,omitempty"`
	PreviousEnabled *bool            `json:"previous_enabled,omitempty"`
	NewEnabled      *bool            `json:"new_enabled,omitempty"`
	AppliedAt       *time.Time       `json:"applied_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

// Validate checks that the ReleaseFlagChange has all required fields populated.
func (c *ReleaseFlagChange) Validate() error {
	if c.ReleaseID == uuid.Nil {
		return errors.New("release_id is required")
	}
	if c.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if c.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	return nil
}
