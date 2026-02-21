package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReleaseStatus represents the lifecycle state of a release.
type ReleaseStatus string

const (
	// ReleaseStatusDraft indicates the release is being prepared.
	ReleaseStatusDraft ReleaseStatus = "draft"
	// ReleaseStatusActive indicates the release is actively being deployed.
	ReleaseStatusActive ReleaseStatus = "active"
	// ReleaseStatusCompleted indicates the release has been fully deployed.
	ReleaseStatusCompleted ReleaseStatus = "completed"
	// ReleaseStatusFailed indicates the release encountered a failure.
	ReleaseStatusFailed ReleaseStatus = "failed"
	// ReleaseStatusArchived indicates the release has been archived.
	ReleaseStatusArchived ReleaseStatus = "archived"
)

// validReleaseTransitions defines which release status transitions are allowed.
var validReleaseTransitions = map[ReleaseStatus][]ReleaseStatus{
	ReleaseStatusDraft:     {ReleaseStatusActive, ReleaseStatusArchived},
	ReleaseStatusActive:    {ReleaseStatusCompleted, ReleaseStatusFailed, ReleaseStatusArchived},
	ReleaseStatusCompleted: {ReleaseStatusArchived},
	ReleaseStatusFailed:    {ReleaseStatusDraft, ReleaseStatusArchived},
	ReleaseStatusArchived:  {},
}

// Release represents a versioned release that can be deployed through a pipeline.
type Release struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	ProjectID   uuid.UUID     `json:"project_id" db:"project_id"`
	Version     string        `json:"version" db:"version"`
	Title       string        `json:"title" db:"title"`
	Description string        `json:"description,omitempty" db:"description"`
	CommitSHA   string        `json:"commit_sha,omitempty" db:"commit_sha"`
	Artifact    string        `json:"artifact" db:"artifact"`
	Status      ReleaseStatus `json:"status" db:"status"`
	CreatedBy   uuid.UUID     `json:"created_by" db:"created_by"`
	ReleasedAt  *time.Time    `json:"released_at,omitempty" db:"released_at"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
}

// ReleaseEnvironment tracks which environments a release has been deployed to
// and the status of each deployment.
type ReleaseEnvironment struct {
	ID            uuid.UUID     `json:"id" db:"id"`
	ReleaseID     uuid.UUID     `json:"release_id" db:"release_id"`
	EnvironmentID uuid.UUID     `json:"environment_id" db:"environment_id"`
	DeploymentID  *uuid.UUID    `json:"deployment_id,omitempty" db:"deployment_id"`
	Status        ReleaseStatus `json:"status" db:"status"`
	DeployedAt    *time.Time    `json:"deployed_at,omitempty" db:"deployed_at"`
	DeployedBy    *uuid.UUID    `json:"deployed_by,omitempty" db:"deployed_by"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at" db:"updated_at"`
}

// Validate checks that the Release has all required fields populated.
func (r *Release) Validate() error {
	if r.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if r.Version == "" {
		return errors.New("version is required")
	}
	if r.Title == "" {
		return errors.New("title is required")
	}
	if r.Artifact == "" {
		return errors.New("artifact is required")
	}
	if r.CreatedBy == uuid.Nil {
		return errors.New("created_by is required")
	}
	return nil
}

// ValidateTransition checks whether moving from the release's current status
// to the target status is allowed.
func (r *Release) ValidateTransition(target ReleaseStatus) error {
	allowed, ok := validReleaseTransitions[r.Status]
	if !ok {
		return fmt.Errorf("unknown current release status %q", r.Status)
	}
	for _, s := range allowed {
		if s == target {
			return nil
		}
	}
	return fmt.Errorf("invalid release status transition from %q to %q", r.Status, target)
}
