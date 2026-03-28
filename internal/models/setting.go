package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Setting represents a hierarchical configuration setting scoped to exactly
// one level of the org > project > application > environment hierarchy.
type Setting struct {
	ID            uuid.UUID       `json:"id"`
	OrgID         *uuid.UUID      `json:"org_id,omitempty"`
	ProjectID     *uuid.UUID      `json:"project_id,omitempty"`
	ApplicationID *uuid.UUID      `json:"application_id,omitempty"`
	EnvironmentID *uuid.UUID      `json:"environment_id,omitempty"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	UpdatedBy     *uuid.UUID      `json:"updated_by,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Validate checks that the Setting has all required fields populated and
// that exactly one scope is set.
func (s *Setting) Validate() error {
	if s.Key == "" {
		return errors.New("key is required")
	}
	if len(s.Value) == 0 {
		return errors.New("value is required")
	}
	scopeCount := 0
	if s.OrgID != nil {
		scopeCount++
	}
	if s.ProjectID != nil {
		scopeCount++
	}
	if s.ApplicationID != nil {
		scopeCount++
	}
	if s.EnvironmentID != nil {
		scopeCount++
	}
	if scopeCount == 0 {
		return errors.New("exactly one scope (org_id, project_id, application_id, environment_id) must be set")
	}
	if scopeCount > 1 {
		return errors.New("only one scope (org_id, project_id, application_id, environment_id) may be set")
	}
	return nil
}

// ScopeLevel returns the name of the scope level this setting is scoped to.
func (s *Setting) ScopeLevel() string {
	switch {
	case s.EnvironmentID != nil:
		return "environment"
	case s.ApplicationID != nil:
		return "application"
	case s.ProjectID != nil:
		return "project"
	case s.OrgID != nil:
		return "org"
	default:
		return ""
	}
}
