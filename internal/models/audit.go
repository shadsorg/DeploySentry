package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLogEntry records an auditable action performed by a user in the system.
// Audit log entries are immutable and append-only.
type AuditLogEntry struct {
	ID         uuid.UUID `json:"id" db:"id"`
	OrgID      uuid.UUID `json:"org_id" db:"org_id"`
	ProjectID  uuid.UUID `json:"project_id,omitempty" db:"project_id"`
	ActorID    uuid.UUID `json:"actor_id" db:"actor_id"`
	ActorName  string    `json:"actor_name,omitempty" db:"-"`
	Action     string    `json:"action" db:"action"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id" db:"entity_id"`
	OldValue   string    `json:"old_value,omitempty" db:"old_value"`
	NewValue   string    `json:"new_value,omitempty" db:"new_value"`
	IPAddress  string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	Revertible bool      `json:"revertible" db:"-"`
}
