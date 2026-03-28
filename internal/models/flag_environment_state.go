package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlagEnvironmentState tracks the per-environment state of a feature flag.
type FlagEnvironmentState struct {
	ID            uuid.UUID        `json:"id"`
	FlagID        uuid.UUID        `json:"flag_id"`
	EnvironmentID uuid.UUID        `json:"environment_id"`
	Enabled       bool             `json:"enabled"`
	Value         *json.RawMessage `json:"value,omitempty"`
	UpdatedBy     *uuid.UUID       `json:"updated_by,omitempty"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// Validate checks that the FlagEnvironmentState has all required fields populated.
func (s *FlagEnvironmentState) Validate() error {
	if s.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if s.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	return nil
}
