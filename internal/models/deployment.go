package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DeployStatus represents the lifecycle state of a deployment.
type DeployStatus string

const (
	// DeployStatusPending indicates the deployment has been created but not started.
	DeployStatusPending DeployStatus = "pending"
	// DeployStatusRunning indicates the deployment is actively rolling out.
	DeployStatusRunning DeployStatus = "running"
	// DeployStatusPaused indicates the deployment has been temporarily paused.
	DeployStatusPaused DeployStatus = "paused"
	// DeployStatusPromoting indicates the deployment is being promoted to full traffic.
	DeployStatusPromoting DeployStatus = "promoting"
	// DeployStatusCompleted indicates the deployment finished successfully.
	DeployStatusCompleted DeployStatus = "completed"
	// DeployStatusFailed indicates the deployment encountered an unrecoverable error.
	DeployStatusFailed DeployStatus = "failed"
	// DeployStatusRolledBack indicates the deployment was rolled back.
	DeployStatusRolledBack DeployStatus = "rolled_back"
	// DeployStatusCancelled indicates the deployment was cancelled by a user.
	DeployStatusCancelled DeployStatus = "cancelled"
)

// DeployStrategy identifies the deployment strategy used.
type DeployStrategyType string

const (
	// DeployStrategyCanary routes a percentage of traffic to the new version.
	DeployStrategyCanary DeployStrategyType = "canary"
	// DeployStrategyBlueGreen swaps between two identical environments.
	DeployStrategyBlueGreen DeployStrategyType = "blue_green"
	// DeployStrategyRolling replaces instances incrementally.
	DeployStrategyRolling DeployStrategyType = "rolling"
)

// validTransitions defines which status transitions are allowed.
var validTransitions = map[DeployStatus][]DeployStatus{
	DeployStatusPending:    {DeployStatusRunning, DeployStatusCancelled},
	DeployStatusRunning:    {DeployStatusPaused, DeployStatusPromoting, DeployStatusCompleted, DeployStatusFailed, DeployStatusRolledBack, DeployStatusCancelled},
	DeployStatusPaused:     {DeployStatusRunning, DeployStatusRolledBack, DeployStatusCancelled},
	DeployStatusPromoting:  {DeployStatusCompleted, DeployStatusFailed, DeployStatusRolledBack},
	DeployStatusCompleted:  {},
	DeployStatusFailed:     {},
	DeployStatusRolledBack: {},
	DeployStatusCancelled:  {},
}

// DeployPipeline defines an ordered sequence of environments that a deployment
// must pass through (e.g., staging -> canary -> production).
type DeployPipeline struct {
	ID          uuid.UUID `json:"id" db:"id"`
	ProjectID   uuid.UUID `json:"project_id" db:"project_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	// Stages is the ordered list of environment IDs in this pipeline.
	Stages    []uuid.UUID `json:"stages" db:"-"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Deployment represents a single deployment of an artifact to an environment.
type Deployment struct {
	ID             uuid.UUID          `json:"id" db:"id"`
	ProjectID      uuid.UUID          `json:"project_id" db:"project_id"`
	EnvironmentID  uuid.UUID          `json:"environment_id" db:"environment_id"`
	PipelineID     *uuid.UUID         `json:"pipeline_id,omitempty" db:"pipeline_id"`
	ReleaseID      *uuid.UUID         `json:"release_id,omitempty" db:"release_id"`
	Strategy       DeployStrategyType `json:"strategy" db:"strategy"`
	Status         DeployStatus       `json:"status" db:"status"`
	Artifact       string             `json:"artifact" db:"artifact"`
	Version        string             `json:"version" db:"version"`
	CommitSHA      string             `json:"commit_sha,omitempty" db:"commit_sha"`
	TrafficPercent int                `json:"traffic_percent" db:"traffic_percent"`
	Metadata       map[string]string  `json:"metadata,omitempty" db:"-"`
	CreatedBy      uuid.UUID          `json:"created_by" db:"created_by"`
	StartedAt      *time.Time         `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt      time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at" db:"updated_at"`
}

// DeploymentPhase represents a discrete step within a deployment rollout,
// such as an incremental canary traffic increase.
type DeploymentPhase struct {
	ID             uuid.UUID    `json:"id" db:"id"`
	DeploymentID   uuid.UUID    `json:"deployment_id" db:"deployment_id"`
	Name           string       `json:"name" db:"name"`
	Status         DeployStatus `json:"status" db:"status"`
	TrafficPercent int          `json:"traffic_percent" db:"traffic_percent"`
	Duration       int          `json:"duration_seconds" db:"duration_seconds"`
	SortOrder      int          `json:"sort_order" db:"sort_order"`
	StartedAt      *time.Time   `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty" db:"completed_at"`
}

// ValidateTransition checks whether moving from the deployment's current status
// to the target status is allowed by the state machine. It returns an error
// describing the violation if the transition is not permitted.
func (d *Deployment) ValidateTransition(target DeployStatus) error {
	allowed, ok := validTransitions[d.Status]
	if !ok {
		return fmt.Errorf("unknown current status %q", d.Status)
	}
	for _, s := range allowed {
		if s == target {
			return nil
		}
	}
	return fmt.Errorf("invalid status transition from %q to %q", d.Status, target)
}

// TransitionTo attempts to move the deployment to the target status.
// It validates the transition and updates the status and relevant timestamps.
func (d *Deployment) TransitionTo(target DeployStatus) error {
	if err := d.ValidateTransition(target); err != nil {
		return err
	}
	now := time.Now().UTC()
	d.Status = target
	d.UpdatedAt = now

	switch target {
	case DeployStatusRunning:
		if d.StartedAt == nil {
			d.StartedAt = &now
		}
	case DeployStatusCompleted, DeployStatusFailed, DeployStatusRolledBack, DeployStatusCancelled:
		d.CompletedAt = &now
	}
	return nil
}

// Validate checks that the Deployment has all required fields populated.
func (d *Deployment) Validate() error {
	if d.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if d.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	if d.Artifact == "" {
		return errors.New("artifact is required")
	}
	if d.Version == "" {
		return errors.New("version is required")
	}
	if d.CreatedBy == uuid.Nil {
		return errors.New("created_by is required")
	}
	switch d.Strategy {
	case DeployStrategyCanary, DeployStrategyBlueGreen, DeployStrategyRolling:
		// valid
	default:
		return errors.New("unsupported deploy strategy")
	}
	return nil
}

// IsTerminal reports whether the deployment is in a terminal state
// (completed, failed, rolled back, or cancelled).
func (d *Deployment) IsTerminal() bool {
	switch d.Status {
	case DeployStatusCompleted, DeployStatusFailed, DeployStatusRolledBack, DeployStatusCancelled:
		return true
	}
	return false
}
