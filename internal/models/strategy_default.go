package models

import (
	"time"

	"github.com/google/uuid"
)

// StrategyDefault pins a default strategy for a (scope, environment, target_type).
// Empty Environment or TargetType means wildcard.
type StrategyDefault struct {
	ID          uuid.UUID   `json:"id"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	Environment *string     `json:"environment,omitempty"`
	TargetType  *TargetType `json:"target_type,omitempty"`
	StrategyID  uuid.UUID   `json:"strategy_id"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID  `json:"updated_by,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
