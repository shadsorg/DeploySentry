package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// StagedChange is a per-user, pending dashboard mutation that has not yet
// been committed to its production table. Spec lives at
// docs/superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md.
//
// ResourceID is nil for staged CREATEs; ProvisionalID is set instead so other
// staged rows in the same batch can reference the soon-to-exist resource. On
// Deploy commit, ProvisionalID is rewritten to the real id everywhere it is
// referenced.
type StagedChange struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	UserID        uuid.UUID       `json:"user_id" db:"user_id"`
	OrgID         uuid.UUID       `json:"org_id" db:"org_id"`
	ResourceType  string          `json:"resource_type" db:"resource_type"`
	ResourceID    *uuid.UUID      `json:"resource_id,omitempty" db:"resource_id"`
	ProvisionalID *uuid.UUID      `json:"provisional_id,omitempty" db:"provisional_id"`
	Action        string          `json:"action" db:"action"`
	FieldPath     string          `json:"field_path,omitempty" db:"field_path"`
	OldValue      json.RawMessage `json:"old_value,omitempty" db:"old_value"`
	NewValue      json.RawMessage `json:"new_value,omitempty" db:"new_value"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}
