// Package deploy implements the deployment management domain, including
// creation, promotion, rollback, and lifecycle management of deployments.
package deploy

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// DeployRepository defines the persistence interface for deployment entities.
type DeployRepository interface {
	// CreateDeployment persists a new deployment record.
	CreateDeployment(ctx context.Context, d *models.Deployment) error

	// GetDeployment retrieves a deployment by its unique identifier.
	GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error)

	// ListDeployments returns deployments for a project, ordered by creation time descending.
	ListDeployments(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Deployment, error)

	// UpdateDeployment persists changes to an existing deployment.
	UpdateDeployment(ctx context.Context, d *models.Deployment) error

	// ListDeploymentPhases returns the ordered phases for a deployment.
	ListDeploymentPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error)

	// CreateDeploymentPhase persists a new deployment phase.
	CreateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error

	// UpdateDeploymentPhase persists changes to an existing deployment phase.
	UpdateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error

	// GetPipeline retrieves a deploy pipeline by ID.
	GetPipeline(ctx context.Context, id uuid.UUID) (*models.DeployPipeline, error)

	// GetLatestDeployment returns the most recent deployment for a project and environment.
	GetLatestDeployment(ctx context.Context, projectID, environmentID uuid.UUID) (*models.Deployment, error)
}

// ListOptions controls pagination and filtering for list queries.
type ListOptions struct {
	Limit         int                  `json:"limit"`
	Offset        int                  `json:"offset"`
	EnvironmentID *uuid.UUID           `json:"environment_id,omitempty"`
	Status        *models.DeployStatus `json:"status,omitempty"`
}
