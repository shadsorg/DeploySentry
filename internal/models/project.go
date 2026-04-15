package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Project represents a deployable application within an organization.
type Project struct {
	ID          uuid.UUID `json:"id" db:"id"`
	OrgID       uuid.UUID `json:"org_id" db:"org_id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description,omitempty" db:"description"`
	RepoURL     string    `json:"repo_url,omitempty" db:"repo_url"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// ProjectRole defines the roles a member can hold within a project.
type ProjectRole string

const (
	// ProjectRoleAdmin has full control over the project.
	ProjectRoleAdmin ProjectRole = "admin"
	// ProjectRoleDeveloper can create deployments and manage flags.
	ProjectRoleDeveloper ProjectRole = "developer"
	// ProjectRoleViewer has read-only access to the project.
	ProjectRoleViewer ProjectRole = "viewer"
)

// ProjectMember represents a user's membership and role within a specific project.
type ProjectMember struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	ProjectID uuid.UUID   `json:"project_id" db:"project_id"`
	UserID    uuid.UUID   `json:"user_id" db:"user_id"`
	Role      ProjectRole `json:"role" db:"role"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Environment represents a deployment target (e.g., staging, production)
// within a project.
type Environment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	ApplicationID uuid.UUID `json:"application_id" db:"application_id"`
	Name          string    `json:"name" db:"name"`
	Slug          string    `json:"slug" db:"slug"`
	IsProduction  bool      `json:"is_production" db:"is_production"`
	SortOrder     int       `json:"sort_order" db:"sort_order"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Validate checks that the Project has all required fields populated.
func (p *Project) Validate() error {
	if p.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if p.Name == "" {
		return errors.New("project name is required")
	}
	if len(p.Name) > 100 {
		return errors.New("project name must be 100 characters or fewer")
	}
	if p.Slug == "" {
		return errors.New("project slug is required")
	}
	return nil
}

// FlagActivitySummary describes a flag with recent evaluation activity.
type FlagActivitySummary struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	LastEvaluated time.Time `json:"last_evaluated"`
}

// DeleteResult describes the outcome of a delete operation.
type DeleteResult struct {
	Deleted               string                `json:"deleted,omitempty"`
	EligibleForHardDelete *time.Time            `json:"eligible_for_hard_delete,omitempty"`
	ActiveFlags           []FlagActivitySummary `json:"active_flags,omitempty"`
}

// Validate checks that the Environment has all required fields populated.
func (e *Environment) Validate() error {
	if e.ApplicationID == uuid.Nil {
		return errors.New("application_id is required")
	}
	if e.Name == "" {
		return errors.New("environment name is required")
	}
	if e.Slug == "" {
		return errors.New("environment slug is required")
	}
	return nil
}
