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

	// WithTx executes fn inside a database transaction backed by this repository.
	WithTx(ctx context.Context, fn TxFunc) error
}

// TxRepository is a subset of DeployRepository scoped to a database transaction.
type TxRepository interface {
	UpdateDeployment(ctx context.Context, d *models.Deployment) error
	UpdatePhase(ctx context.Context, phase *models.DeploymentPhase) error
	CreateRollbackRecord(ctx context.Context, record *models.RollbackRecord) error
}

// TxFunc is a function executed inside a database transaction.
type TxFunc func(tx TxRepository) error

// ListOptions controls pagination and filtering for list queries.
type ListOptions struct {
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
	EnvironmentID   *uuid.UUID           `json:"environment_id,omitempty"`
	Status          *models.DeployStatus `json:"status,omitempty"`
	ExcludeTerminal bool                 `json:"exclude_terminal,omitempty"`
}
