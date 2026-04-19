package models

import (
	"time"

	"github.com/google/uuid"
)

// CoordinationPolicy determines how a RolloutGroup reacts when any of its
// member rollouts rolls back.
type CoordinationPolicy string

const (
	// CoordinationIndependent means sibling rollbacks do not affect each other.
	CoordinationIndependent CoordinationPolicy = "independent"
	// CoordinationPauseOnSiblingAbort means active siblings are paused when any
	// rollout in the group rolls back.
	CoordinationPauseOnSiblingAbort CoordinationPolicy = "pause_on_sibling_abort"
	// CoordinationCascadeAbort means active siblings are rolled back when any
	// rollout in the group rolls back.
	CoordinationCascadeAbort CoordinationPolicy = "cascade_abort"
)

// RolloutGroup is an optional bundle grouping related rollouts. Rollouts
// reference their group via Rollout.ReleaseID (column name preserved from
// Plan 2 migration 050; the value is a rollout_groups.id).
type RolloutGroup struct {
	ID                 uuid.UUID          `json:"id"`
	ScopeType          ScopeType          `json:"scope_type"`
	ScopeID            uuid.UUID          `json:"scope_id"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	CoordinationPolicy CoordinationPolicy `json:"coordination_policy"`
	CreatedBy          *uuid.UUID         `json:"created_by,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}
