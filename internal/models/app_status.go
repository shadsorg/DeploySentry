package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HealthState is the self-reported or observed health of a deployed application.
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateDegraded  HealthState = "degraded"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateUnknown   HealthState = "unknown"
)

// AppStatus is the latest self-reported status for a given (application, environment).
type AppStatus struct {
	ApplicationID uuid.UUID         `json:"application_id" db:"application_id"`
	EnvironmentID uuid.UUID         `json:"environment_id" db:"environment_id"`
	Version       string            `json:"version" db:"version"`
	CommitSHA     string            `json:"commit_sha,omitempty" db:"commit_sha"`
	HealthState   HealthState       `json:"health_state" db:"health_state"`
	HealthScore   *float64          `json:"health_score,omitempty" db:"health_score"`
	HealthReason  string            `json:"health_reason,omitempty" db:"health_reason"`
	DeploySlot    string            `json:"deploy_slot,omitempty" db:"deploy_slot"`
	Tags          map[string]string `json:"tags" db:"tags"`
	Source        string            `json:"source" db:"source"`
	ReportedAt    time.Time         `json:"reported_at" db:"reported_at"`
}

// AppStatusSample is a single historical reading retained for sparkline and forensics.
type AppStatusSample struct {
	ID            int64       `json:"id" db:"id"`
	ApplicationID uuid.UUID   `json:"application_id" db:"application_id"`
	EnvironmentID uuid.UUID   `json:"environment_id" db:"environment_id"`
	Version       string      `json:"version" db:"version"`
	HealthState   HealthState `json:"health_state" db:"health_state"`
	HealthScore   *float64    `json:"health_score,omitempty" db:"health_score"`
	ReportedAt    time.Time   `json:"reported_at" db:"reported_at"`
}

// ReportStatusPayload is the JSON body accepted by POST /applications/:id/status.
type ReportStatusPayload struct {
	Version      string            `json:"version"`
	CommitSHA    string            `json:"commit_sha,omitempty"`
	Health       HealthState       `json:"health"`
	HealthScore  *float64          `json:"health_score,omitempty"`
	HealthReason string            `json:"health_reason,omitempty"`
	DeploySlot   string            `json:"deploy_slot,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// Validate enforces the required fields and enum constraints on a status report.
func (p *ReportStatusPayload) Validate() error {
	if p.Version == "" {
		return errors.New("version is required")
	}
	switch p.Health {
	case HealthStateHealthy, HealthStateDegraded, HealthStateUnhealthy, HealthStateUnknown:
	case "":
		return errors.New("health is required")
	default:
		return fmt.Errorf("unsupported health state %q", p.Health)
	}
	if p.HealthScore != nil {
		if *p.HealthScore < 0 || *p.HealthScore > 1 {
			return errors.New("health_score must be between 0 and 1")
		}
	}
	return nil
}
