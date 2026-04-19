package models

import (
	"time"

	"github.com/google/uuid"
)

// PolicyKind controls how a scope enforces rollout usage.
type PolicyKind string

const (
	// PolicyOff means rollout control is disabled; changes apply immediately (default / backward compat).
	PolicyOff PolicyKind = "off"
	// PolicyPrompt means user is prompted to attach a strategy; apply-immediately remains an option.
	PolicyPrompt PolicyKind = "prompt"
	// PolicyMandate means changes on this scope must attach a strategy; no immediate-apply path.
	PolicyMandate PolicyKind = "mandate"
)

// RolloutPolicy is the onboarding + mandate row for a scope.
type RolloutPolicy struct {
	ID          uuid.UUID   `json:"id"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	Environment *string     `json:"environment,omitempty"`
	TargetType  *TargetType `json:"target_type,omitempty"`
	Enabled     bool        `json:"enabled"`
	Policy      PolicyKind  `json:"policy"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID  `json:"updated_by,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
