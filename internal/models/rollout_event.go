package models

import (
	"time"

	"github.com/google/uuid"
)

// EventType identifies a rollout audit event.
type EventType string

const (
	// EventAttached fires when a rollout is created from a deploy/config request.
	EventAttached EventType = "attached"
	// EventPhaseEntered fires when the engine enters a phase.
	EventPhaseEntered EventType = "phase_entered"
	// EventPhaseExited fires when a phase completes.
	EventPhaseExited EventType = "phase_exited"
	// EventPaused fires when an operator pauses the rollout.
	EventPaused EventType = "paused"
	// EventResumed fires when an operator resumes.
	EventResumed EventType = "resumed"
	// EventPromoted fires on manual promote.
	EventPromoted EventType = "promoted"
	// EventForcePromoted fires on force-promote (requires reason).
	EventForcePromoted EventType = "force_promoted"
	// EventApproved fires when an approval gate is granted.
	EventApproved EventType = "approved"
	// EventAbortConditionTripped fires when an abort condition trips.
	EventAbortConditionTripped EventType = "abort_condition_tripped"
	// EventRollbackTriggered fires when rollback begins.
	EventRollbackTriggered EventType = "rollback_triggered"
	// EventCompleted fires when the rollout succeeds.
	EventCompleted EventType = "completed"
	// EventSuperseded fires when another rollout takes over the target.
	EventSuperseded EventType = "superseded"
)

// RolloutEvent is one row in the audit trail.
type RolloutEvent struct {
	ID         uuid.UUID              `json:"id"`
	RolloutID  uuid.UUID              `json:"rollout_id"`
	EventType  EventType              `json:"event_type"`
	ActorType  string                 `json:"actor_type"` // "user" or "system"
	ActorID    *uuid.UUID             `json:"actor_id,omitempty"`
	Reason     *string                `json:"reason,omitempty"`
	Payload    map[string]interface{} `json:"payload"`
	OccurredAt time.Time              `json:"occurred_at"`
}
