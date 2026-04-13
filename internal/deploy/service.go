package deploy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// MessagePublisher defines the interface for publishing domain events
// to a message broker.
type MessagePublisher interface {
	// Publish sends a message with the given subject and payload.
	Publish(ctx context.Context, subject string, payload []byte) error
}

// DeployService defines the interface for managing deployments.
type DeployService interface {
	// CreateDeployment creates a new deployment in pending state.
	CreateDeployment(ctx context.Context, d *models.Deployment) error

	// GetDeployment retrieves a deployment by ID.
	GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error)

	// ListDeployments returns deployments for an application.
	ListDeployments(ctx context.Context, applicationID uuid.UUID, opts ListOptions) ([]*models.Deployment, error)

	// PromoteDeployment advances a deployment to full traffic.
	PromoteDeployment(ctx context.Context, id uuid.UUID) error

	// RollbackDeployment rolls back a running or paused deployment.
	RollbackDeployment(ctx context.Context, id uuid.UUID) error

	// PauseDeployment pauses a running deployment.
	PauseDeployment(ctx context.Context, id uuid.UUID) error

	// ResumeDeployment resumes a paused deployment.
	ResumeDeployment(ctx context.Context, id uuid.UUID) error

	// GetActiveDeployments returns all non-terminal deployments for an application.
	GetActiveDeployments(ctx context.Context, applicationID uuid.UUID) ([]*models.Deployment, error)

	// ListPhases returns all phases for a deployment, ordered by sort_order ascending.
	ListPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error)

	// ListRollbackRecords returns the rollback history for a deployment.
	ListRollbackRecords(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error)
}

// deployService is the concrete implementation of DeployService.
type deployService struct {
	repo      DeployRepository
	messaging MessagePublisher
}

// NewDeployService creates a new DeployService backed by the given repository
// and message publisher.
func NewDeployService(repo DeployRepository, messaging MessagePublisher) DeployService {
	return &deployService{
		repo:      repo,
		messaging: messaging,
	}
}

// CreateDeployment validates and persists a new deployment, then publishes
// a deployment.created event.
func (s *deployService) CreateDeployment(ctx context.Context, d *models.Deployment) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.Status = models.DeployStatusPending
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now

	if err := d.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Look up the previous completed deployment for this app+env
	prev, err := s.repo.GetLatestCompletedDeployment(ctx, d.ApplicationID, d.EnvironmentID)
	if err == nil && prev != nil {
		d.PreviousDeploymentID = &prev.ID
	}
	// If no previous deployment found, PreviousDeploymentID stays nil — that's fine for first deployment

	if err := s.repo.CreateDeployment(ctx, d); err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}

	s.publishEvent(ctx, "deployment.created", d.ID)
	return nil
}

// GetDeployment retrieves a deployment by its unique identifier.
func (s *deployService) GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	d, err := s.repo.GetDeployment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting deployment: %w", err)
	}
	return d, nil
}

// ListDeployments returns a paginated list of deployments for an application.
func (s *deployService) ListDeployments(ctx context.Context, applicationID uuid.UUID, opts ListOptions) ([]*models.Deployment, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	deployments, err := s.repo.ListDeployments(ctx, applicationID, opts)
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	return deployments, nil
}

// PromoteDeployment transitions a deployment to the promoting state, setting
// traffic to 100% and publishing a deployment.promoted event.
func (s *deployService) PromoteDeployment(ctx context.Context, id uuid.UUID) error {
	d, err := s.repo.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("getting deployment for promotion: %w", err)
	}

	if err := d.ValidateTransition(models.DeployStatusPromoting); err != nil {
		return err
	}

	d.Status = models.DeployStatusPromoting
	d.TrafficPercent = 100
	d.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("updating deployment for promotion: %w", err)
	}

	s.publishEvent(ctx, "deployment.promoted", d.ID)
	return nil
}

// RollbackDeployment transitions a deployment to the rolled_back state.
func (s *deployService) RollbackDeployment(ctx context.Context, id uuid.UUID) error {
	d, err := s.repo.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("getting deployment for rollback: %w", err)
	}

	if err := d.TransitionTo(models.DeployStatusRolledBack); err != nil {
		return err
	}

	if err := s.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("updating deployment for rollback: %w", err)
	}

	s.publishEvent(ctx, "deployment.rolled_back", d.ID)
	return nil
}

// PauseDeployment transitions a running deployment to the paused state.
func (s *deployService) PauseDeployment(ctx context.Context, id uuid.UUID) error {
	d, err := s.repo.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("getting deployment for pause: %w", err)
	}

	if d.Status != models.DeployStatusRunning {
		return errors.New("only running deployments can be paused")
	}

	if err := d.TransitionTo(models.DeployStatusPaused); err != nil {
		return err
	}

	if err := s.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("updating deployment for pause: %w", err)
	}

	s.publishEvent(ctx, "deployment.paused", d.ID)
	return nil
}

// ResumeDeployment transitions a paused deployment back to the running state.
func (s *deployService) ResumeDeployment(ctx context.Context, id uuid.UUID) error {
	d, err := s.repo.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("getting deployment for resume: %w", err)
	}

	if d.Status != models.DeployStatusPaused {
		return errors.New("only paused deployments can be resumed")
	}

	if err := d.TransitionTo(models.DeployStatusRunning); err != nil {
		return err
	}

	if err := s.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("updating deployment for resume: %w", err)
	}

	s.publishEvent(ctx, "deployment.resumed", d.ID)
	return nil
}

// GetActiveDeployments returns all non-terminal deployments (pending, running,
// paused, promoting) for the given application.
func (s *deployService) GetActiveDeployments(ctx context.Context, applicationID uuid.UUID) ([]*models.Deployment, error) {
	// Retrieve deployments for the application with a generous limit
	// and exclude terminal statuses at the repository level.
	active, err := s.repo.ListDeployments(ctx, applicationID, ListOptions{
		Limit:           100,
		ExcludeTerminal: true,
	})
	if err != nil {
		return nil, fmt.Errorf("listing active deployments: %w", err)
	}

	return active, nil
}

// ListPhases returns all phases for a deployment, ordered by sort_order.
func (s *deployService) ListPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	phases, err := s.repo.ListPhases(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("listing phases: %w", err)
	}
	return phases, nil
}

// ListRollbackRecords returns the rollback history for a deployment.
func (s *deployService) ListRollbackRecords(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error) {
	records, err := s.repo.ListRollbackRecords(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("listing rollback records: %w", err)
	}
	return records, nil
}

// publishEvent is a fire-and-forget helper that publishes a domain event.
// Errors are logged but do not fail the calling operation.
func (s *deployService) publishEvent(ctx context.Context, subject string, deploymentID uuid.UUID) {
	// Normalize subject to use plural "deployments." prefix for NATS subscriber compatibility
	natsSubject := "deployments." + subject
	payload := []byte(`{"deployment_id":"` + deploymentID.String() + `"}`)
	// Best-effort publish; errors are non-fatal for the calling operation.
	_ = s.messaging.Publish(ctx, natsSubject, payload)
}
