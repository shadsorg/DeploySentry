package models

import (
	"time"

	"github.com/google/uuid"
)

// TargetType identifies what a rollout applies to.
type TargetType string

const (
	// TargetTypeDeploy means the rollout target is a deployment (version shift).
	TargetTypeDeploy TargetType = "deploy"
	// TargetTypeConfig means the rollout target is a managed config value (flags included).
	TargetTypeConfig TargetType = "config"
	// TargetTypeAny means the strategy can be applied to any target type.
	TargetTypeAny TargetType = "any"
)

// ScopeType identifies the level at which a rollout-control entity is attached.
type ScopeType string

const (
	// ScopeOrg is organization scope.
	ScopeOrg ScopeType = "org"
	// ScopeProject is project scope.
	ScopeProject ScopeType = "project"
	// ScopeApp is application scope.
	ScopeApp ScopeType = "app"
)

// Strategy is a reusable rollout template.
type Strategy struct {
	ID                       uuid.UUID  `json:"id"`
	ScopeType                ScopeType  `json:"scope_type"`
	ScopeID                  uuid.UUID  `json:"scope_id"`
	Name                     string     `json:"name"`
	Description              string     `json:"description"`
	TargetType               TargetType `json:"target_type"`
	Steps                    []Step     `json:"steps"`
	DefaultHealthThreshold   float64    `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool       `json:"default_rollback_on_failure"`
	Version                  int        `json:"version"`
	IsSystem                 bool       `json:"is_system"`
	CreatedBy                *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy                *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// Step is a single phase of a strategy.
type Step struct {
	Percent          float64              `json:"percent"`
	MinDuration      time.Duration        `json:"min_duration"`
	MaxDuration      time.Duration        `json:"max_duration"`
	BakeTimeHealthy  time.Duration        `json:"bake_time_healthy"`
	HealthThreshold  *float64             `json:"health_threshold,omitempty"`
	Approval         *StepApproval        `json:"approval,omitempty"`
	Notify           *StepNotify          `json:"notify,omitempty"`
	AbortConditions  []StepAbortCondition `json:"abort_conditions,omitempty"`
	SignalOverride   *SignalSource        `json:"signal_override,omitempty"`
}

// StepApproval declares that a phase pauses at `awaiting_approval` until granted.
type StepApproval struct {
	RequiredRole string        `json:"required_role"`
	Timeout      time.Duration `json:"timeout"`
}

// StepNotify declares notification channels fired on phase entry/exit.
type StepNotify struct {
	OnEntry []string `json:"on_entry,omitempty"`
	OnExit  []string `json:"on_exit,omitempty"`
}

// StepAbortCondition is a fast-abort threshold evaluated continuously.
type StepAbortCondition struct {
	Metric    string        `json:"metric"`
	Operator  string        `json:"operator"`
	Threshold float64       `json:"threshold"`
	Window    time.Duration `json:"window"`
}

// SignalSource describes where health is read from for a rollout (or a single step).
// Kind `app_env` means "use the rollout's app+env health monitor"; other kinds
// are reserved for Plan 2.
type SignalSource struct {
	Kind string `json:"kind"`
}
