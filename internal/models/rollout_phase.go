package models

import (
	"time"

	"github.com/google/uuid"
)

// PhaseStatus is the state of a single rollout phase.
type PhaseStatus string

const (
	// PhasePending means the phase has not been entered yet.
	PhasePending PhaseStatus = "pending"
	// PhaseActive means the engine has applied the step's percent and is polling health.
	PhaseActive PhaseStatus = "active"
	// PhaseAwaitingApproval means the phase is blocked on an approval gate.
	PhaseAwaitingApproval PhaseStatus = "awaiting_approval"
	// PhasePassed means the phase completed; advancing to the next.
	PhasePassed PhaseStatus = "passed"
	// PhaseFailed is reserved for phases that errored without rollback.
	PhaseFailed PhaseStatus = "failed"
	// PhaseRolledBack means rollback fired while this phase was active.
	PhaseRolledBack PhaseStatus = "rolled_back"
)

// RolloutPhase is the per-phase audit + current-state row.
type RolloutPhase struct {
	ID                uuid.UUID   `json:"id"`
	RolloutID         uuid.UUID   `json:"rollout_id"`
	PhaseIndex        int         `json:"phase_index"`
	StepSnapshot      Step        `json:"step_snapshot"`
	Status            PhaseStatus `json:"status"`
	EnteredAt         *time.Time  `json:"entered_at,omitempty"`
	ExitedAt          *time.Time  `json:"exited_at,omitempty"`
	AppliedPercent    *float64    `json:"applied_percent,omitempty"`
	HealthScoreAtExit *float64    `json:"health_score_at_exit,omitempty"`
	Notes             string      `json:"notes"`
}
