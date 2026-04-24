package models

import (
	"time"

	"github.com/google/uuid"
)

// RolloutStatus is the top-level state of a rollout.
type RolloutStatus string

const (
	// RolloutPending means the rollout is queued but no phase has been entered yet.
	RolloutPending RolloutStatus = "pending"
	// RolloutActive means the engine is currently driving through phases.
	RolloutActive RolloutStatus = "active"
	// RolloutPaused means an operator paused progression; engine waits.
	RolloutPaused RolloutStatus = "paused"
	// RolloutAwaitingApproval means a phase-level approval gate is blocking.
	RolloutAwaitingApproval RolloutStatus = "awaiting_approval"
	// RolloutSucceeded means all phases completed successfully.
	RolloutSucceeded RolloutStatus = "succeeded"
	// RolloutRolledBack means a rollback was triggered (health or operator).
	RolloutRolledBack RolloutStatus = "rolled_back"
	// RolloutAborted is reserved for future external abort signals.
	RolloutAborted RolloutStatus = "aborted"
	// RolloutSuperseded means another rollout took over the target.
	RolloutSuperseded RolloutStatus = "superseded"
)

// RolloutTargetRef points at the specific resource a rollout is driving.
// For TargetTypeDeploy: DeploymentID is set.
// For TargetTypeConfig: RuleID is set, and PreviousPercentage captures the
// pre-rollout value so Revert can restore it.
type RolloutTargetRef struct {
	DeploymentID       *string `json:"deployment_id,omitempty"`
	FlagKey            *string `json:"flag_key,omitempty"`
	Env                *string `json:"env,omitempty"`
	RuleID             *string `json:"rule_id,omitempty"`
	PreviousPercentage *int    `json:"previous_percentage,omitempty"`
}

// Rollout wraps a progressive change. One row per in-flight or historical rollout.
type Rollout struct {
	ID                     uuid.UUID        `json:"id"`
	ReleaseID              *uuid.UUID       `json:"release_id,omitempty"`
	TargetType             TargetType       `json:"target_type"`
	TargetRef              RolloutTargetRef `json:"target_ref"`
	StrategySnapshot       Strategy         `json:"strategy_snapshot"`
	SignalSource           SignalSource     `json:"signal_source"`
	Status                 RolloutStatus    `json:"status"`
	CurrentPhaseIndex      int              `json:"current_phase_index"`
	CurrentPhaseStartedAt  *time.Time       `json:"current_phase_started_at,omitempty"`
	LastHealthySince       *time.Time       `json:"last_healthy_since,omitempty"`
	RollbackReason         *string          `json:"rollback_reason,omitempty"`
	CreatedBy              *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt              time.Time        `json:"created_at"`
	CompletedAt            *time.Time       `json:"completed_at,omitempty"`
}

// RolloutTargetDisplay is a human-readable snapshot of what a rollout is
// driving. Attached at list time so operators can tell rollouts apart
// without clicking into each row. All fields are best-effort — if the
// underlying row (deployment, flag, etc.) has been deleted, fields stay
// empty and the UI renders a "(unknown)" placeholder.
type RolloutTargetDisplay struct {
	Kind            string `json:"kind"` // "deploy" | "config"
	Summary         string `json:"summary"`
	ApplicationSlug string `json:"application_slug,omitempty"`
	ApplicationName string `json:"application_name,omitempty"`
	ProjectSlug     string `json:"project_slug,omitempty"`
	EnvironmentSlug string `json:"environment_slug,omitempty"`
	Version         string `json:"version,omitempty"`
	FlagKey         string `json:"flag_key,omitempty"`
}

// RolloutWithTarget is the enriched list-row shape returned by the
// list endpoint. Keeps Rollout unchanged so existing consumers don't
// break; new clients render the Target block.
type RolloutWithTarget struct {
	*Rollout
	Target RolloutTargetDisplay `json:"target"`
	AgeSeconds int64             `json:"age_seconds"`
}

// IsTerminal reports whether status represents a finished rollout.
func (r *Rollout) IsTerminal() bool {
	switch r.Status {
	case RolloutSucceeded, RolloutRolledBack, RolloutAborted, RolloutSuperseded:
		return true
	}
	return false
}
