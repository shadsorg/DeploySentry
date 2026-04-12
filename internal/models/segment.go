package models

import (
	"time"

	"github.com/google/uuid"
)

// Segment defines a reusable group of conditions for flag targeting.
type Segment struct {
	ID          uuid.UUID          `json:"id" db:"id"`
	ProjectID   uuid.UUID          `json:"project_id" db:"project_id"`
	Key         string             `json:"key" db:"key"`
	Name        string             `json:"name" db:"name"`
	Description string             `json:"description" db:"description"`
	CombineOp   string             `json:"combine_op" db:"combine_op"`
	Conditions  []SegmentCondition `json:"conditions" db:"-"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
}

// SegmentCondition defines a single condition within a segment.
type SegmentCondition struct {
	ID        uuid.UUID `json:"id" db:"id"`
	SegmentID uuid.UUID `json:"segment_id" db:"segment_id"`
	Attribute string    `json:"attribute" db:"attribute"`
	Operator  string    `json:"operator" db:"operator"`
	Value     string    `json:"value" db:"value"`
	Priority  int       `json:"priority" db:"priority"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
