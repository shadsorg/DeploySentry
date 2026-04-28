// Package deploy implements the deployment management domain, including
// creation, promotion, rollback, and lifecycle management of deployments.
package deploy

import (
	"context"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// DeployRepository defines the persistence interface for deployment entities.
type DeployRepository interface {
	// CreateDeployment persists a new deployment record.
	CreateDeployment(ctx context.Context, d *models.Deployment) error

	// GetDeployment retrieves a deployment by its unique identifier.
	GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error)

	// ListDeployments returns deployments for an application, ordered by creation time descending.
	ListDeployments(ctx context.Context, applicationID uuid.UUID, opts ListOptions) ([]*models.Deployment, error)

	// UpdateDeployment persists changes to an existing deployment.
	UpdateDeployment(ctx context.Context, d *models.Deployment) error

	// GetLatestDeployment returns the most recent deployment for an application and environment.
	GetLatestDeployment(ctx context.Context, applicationID, environmentID uuid.UUID) (*models.Deployment, error)

	// CreatePhase persists a new deployment phase record.
	CreatePhase(ctx context.Context, phase *models.DeploymentPhase) error

	// ListPhases returns all phases for a deployment, ordered by sort_order ascending.
	ListPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error)

	// UpdatePhase persists changes to a deployment phase.
	UpdatePhase(ctx context.Context, phase *models.DeploymentPhase) error

	// GetActivePhase returns the currently active phase for a deployment, or nil if none.
	GetActivePhase(ctx context.Context, deploymentID uuid.UUID) (*models.DeploymentPhase, error)

	// GetLatestCompletedDeployment returns the most recent completed deployment
	// for an application and environment. Used to populate previous_deployment_id.
	GetLatestCompletedDeployment(ctx context.Context, applicationID, environmentID uuid.UUID) (*models.Deployment, error)

	// CreateRollbackRecord persists a rollback history entry.
	CreateRollbackRecord(ctx context.Context, record *models.RollbackRecord) error

	// ListRollbackRecords returns rollback history for a deployment.
	ListRollbackRecords(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error)

	// ListDistinctArtifacts returns the most-recently-used distinct artifact
	// strings for an application, newest-first, capped at limit. Powers the
	// artifact autocomplete on the Deployment Create form.
	ListDistinctArtifacts(ctx context.Context, applicationID uuid.UUID, limit int) ([]ArtifactSuggestion, error)

	// ListDistinctVersions returns the most-recently-used distinct versions
	// for an application, optionally filtered to a single environment.
	// Each suggestion carries the commit_sha seen with that version and
	// the environment IDs it has run in. Newest-first, capped at limit.
	ListDistinctVersions(ctx context.Context, applicationID uuid.UUID, environmentID *uuid.UUID, limit int) ([]VersionSuggestion, error)

	// UpsertBuildDeployment inserts or updates a record-mode deployment row
	// identified by (application_id, environment_id, commit_sha,
	// workflow_name). Returns the row's ID and whether it was newly created.
	// Powers the GitHub workflow_run webhook ingestion path.
	UpsertBuildDeployment(ctx context.Context, in BuildDeploymentUpsert) (id uuid.UUID, created bool, err error)
}

// ArtifactSuggestion is a single artifact dropdown entry.
type ArtifactSuggestion struct {
	Value       string `json:"value"`
	LastSeenAt  string `json:"last_seen_at"`
}

// VersionSuggestion is a single version dropdown entry. EnvironmentIDs
// lists every environment the version has shipped to (empty when the
// caller filtered by a specific env).
type VersionSuggestion struct {
	Version        string      `json:"version"`
	CommitSHA      string      `json:"commit_sha,omitempty"`
	LastSeenAt     string      `json:"last_seen_at"`
	EnvironmentIDs []uuid.UUID `json:"environment_ids,omitempty"`
}

// BuildDeploymentUpsert captures the fields the GitHub webhook adapter
// needs to write a record-mode deployment row. WorkflowName is carried
// via the existing `source` column using the form "github-actions:<name>"
// so the (app, env, commit_sha, source) key remains unique per workflow
// without a schema change.
type BuildDeploymentUpsert struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	CommitSHA     string
	WorkflowName  string
	Version       string
	Artifact      string
	Status        models.DeployStatus
	HTMLURL       string
	CreatedBy     uuid.UUID
	StartedAt     *time.Time
	CompletedAt   *time.Time
}

// ListOptions controls pagination and filtering for list queries.
type ListOptions struct {
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
	EnvironmentID   *uuid.UUID           `json:"environment_id,omitempty"`
	Status          *models.DeployStatus `json:"status,omitempty"`
	ExcludeTerminal bool                 `json:"exclude_terminal,omitempty"`
}
