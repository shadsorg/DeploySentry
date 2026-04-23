package models

import (
	"time"

	"github.com/google/uuid"
)

// HealthStaleness indicates how fresh the latest health sample is.
type HealthStaleness string

const (
	HealthStalenessFresh   HealthStaleness = "fresh"
	HealthStalenessStale   HealthStaleness = "stale"
	HealthStalenessMissing HealthStaleness = "missing"
)

// CurrentStateResponse is the single-call assembly returned by
// GET /api/v1/applications/:id/environments/:id/current-state.
type CurrentStateResponse struct {
	Environment        EnvironmentSummary `json:"environment"`
	CurrentDeployment  *CurrentDeployment `json:"current_deployment,omitempty"`
	Health             HealthBlock        `json:"health"`
	RecentDeployments  []RecentDeployment `json:"recent_deployments"`
	ActiveRollout      any                `json:"active_rollout"` // placeholder, always null in Phase 3
}

// EnvironmentSummary is the identity block of the environment.
type EnvironmentSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug,omitempty"`
	Name string    `json:"name,omitempty"`
}

// CurrentDeployment is the currently-active deployment for the env.
type CurrentDeployment struct {
	ID             uuid.UUID    `json:"id"`
	Version        string       `json:"version"`
	CommitSHA      string       `json:"commit_sha,omitempty"`
	Status         DeployStatus `json:"status"`
	Mode           DeployMode   `json:"mode"`
	Source         *string      `json:"source,omitempty"`
	TrafficPercent int          `json:"traffic_percent"`
	StartedAt      *time.Time   `json:"started_at,omitempty"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
}

// HealthBlock is the rollup of the latest self-reported status.
type HealthBlock struct {
	State          HealthState     `json:"state"`
	Score          *float64        `json:"score,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	Source         string          `json:"source"` // app-push | agent | observability | unknown
	LastReportedAt *time.Time      `json:"last_reported_at,omitempty"`
	Staleness      HealthStaleness `json:"staleness"`
}

// RecentDeployment is a trimmed history row.
type RecentDeployment struct {
	ID          uuid.UUID    `json:"id"`
	Version     string       `json:"version"`
	Status      DeployStatus `json:"status"`
	Mode        DeployMode   `json:"mode"`
	Source      *string      `json:"source,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}
