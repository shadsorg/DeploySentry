package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EventPublisher defines the interface for publishing domain events
// to a message broker.
type EventPublisher interface {
	// Publish sends a message with the given subject and payload.
	Publish(ctx context.Context, subject string, payload []byte) error
}

// PromotionGateType defines the type of promotion gate.
type PromotionGateType string

const (
	// PromotionGateAuto is a gate that is evaluated automatically.
	PromotionGateAuto PromotionGateType = "auto"
	// PromotionGateManual is a gate that requires manual approval.
	PromotionGateManual PromotionGateType = "manual"
)

// PromotionGate defines a gate condition that must be satisfied before
// a release can be promoted to the next environment.
type PromotionGate struct {
	ID            uuid.UUID         `json:"id"`
	ReleaseID     uuid.UUID         `json:"release_id"`
	EnvironmentID uuid.UUID         `json:"environment_id"`
	Type          PromotionGateType `json:"type"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Passed        bool              `json:"passed"`
	ApprovedBy    *uuid.UUID        `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time        `json:"approved_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// HealthProvider is an interface for retrieving deployment health scores.
type HealthProvider interface {
	// GetHealthScore returns the current health score for a deployment.
	GetHealthScore(ctx context.Context, deploymentID uuid.UUID) (float64, error)
}

// ReleaseHealthSummary aggregates health information for a release
// across all environments it has been deployed to.
type ReleaseHealthSummary struct {
	ReleaseID    uuid.UUID                   `json:"release_id"`
	OverallScore float64                     `json:"overall_score"`
	Healthy      bool                        `json:"healthy"`
	Environments []*EnvironmentHealthSummary `json:"environments"`
	EvaluatedAt  time.Time                   `json:"evaluated_at"`
}

// EnvironmentHealthSummary holds health information for a single
// release-environment pairing.
type EnvironmentHealthSummary struct {
	EnvironmentID   uuid.UUID                      `json:"environment_id"`
	LifecycleStatus models.ReleaseLifecycleStatus  `json:"lifecycle_status"`
	HealthScore     float64                        `json:"health_score"`
	Healthy         bool                           `json:"healthy"`
}

// ReleaseStatusResponse is the response body for the release status endpoint.
type ReleaseStatusResponse struct {
	ReleaseID       uuid.UUID                      `json:"release_id"`
	Version         string                         `json:"version"`
	Status          models.ReleaseStatus           `json:"status"`
	LifecycleStatus models.ReleaseLifecycleStatus  `json:"lifecycle_status"`
	Environments    []*models.ReleaseEnvironment   `json:"environments"`
}

// ReleaseService defines the interface for managing releases.
type ReleaseService interface {
	// Create creates a new release in draft state.
	Create(ctx context.Context, release *models.Release) error

	// Get retrieves a release by ID.
	Get(ctx context.Context, id uuid.UUID) (*models.Release, error)

	// List returns releases for a project.
	List(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error)

	// Promote transitions a release to the next environment in its pipeline.
	Promote(ctx context.Context, releaseID, environmentID, deployedBy uuid.UUID) error

	// UpdateStatus updates the lifecycle status of a release.
	UpdateStatus(ctx context.Context, releaseID uuid.UUID, status models.ReleaseLifecycleStatus) error

	// SetPromotionGate adds or updates a promotion gate for a release and environment.
	SetPromotionGate(ctx context.Context, gate *PromotionGate) error

	// CheckPromotionGates verifies whether all promotion gates for a release
	// and target environment have been satisfied.
	CheckPromotionGates(ctx context.Context, releaseID, environmentID uuid.UUID) (bool, error)

	// GetReleaseHealth returns aggregated health information for a release
	// across all environments.
	GetReleaseHealth(ctx context.Context, releaseID uuid.UUID) (*ReleaseHealthSummary, error)

	// GetReleaseStatus returns the current status of a release including all
	// environment deployments.
	GetReleaseStatus(ctx context.Context, releaseID uuid.UUID) (*ReleaseStatusResponse, error)
}

// releaseService is the concrete implementation of ReleaseService.
type releaseService struct {
	repo           ReleaseRepository
	publisher      EventPublisher
	healthProvider HealthProvider
	healthThreshold float64

	mu    sync.RWMutex
	gates map[uuid.UUID][]*PromotionGate // keyed by releaseID
}

// NewReleaseService creates a new ReleaseService backed by the given repository.
func NewReleaseService(repo ReleaseRepository) ReleaseService {
	return &releaseService{
		repo:            repo,
		healthThreshold: 0.7,
		gates:           make(map[uuid.UUID][]*PromotionGate),
	}
}

// NewReleaseServiceWithPublisher creates a new ReleaseService with an event publisher.
func NewReleaseServiceWithPublisher(repo ReleaseRepository, publisher EventPublisher) ReleaseService {
	return &releaseService{
		repo:            repo,
		publisher:       publisher,
		healthThreshold: 0.7,
		gates:           make(map[uuid.UUID][]*PromotionGate),
	}
}

// NewReleaseServiceFull creates a new ReleaseService with all optional dependencies.
func NewReleaseServiceFull(repo ReleaseRepository, publisher EventPublisher, healthProvider HealthProvider, healthThreshold float64) ReleaseService {
	if healthThreshold <= 0 {
		healthThreshold = 0.7
	}
	return &releaseService{
		repo:            repo,
		publisher:       publisher,
		healthProvider:  healthProvider,
		healthThreshold: healthThreshold,
		gates:           make(map[uuid.UUID][]*PromotionGate),
	}
}

// Create validates and persists a new release in draft state.
func (s *releaseService) Create(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	release.Status = models.ReleaseStatusDraft
	release.LifecycleStatus = models.ReleaseLifecycleBuilding
	now := time.Now().UTC()
	release.CreatedAt = now
	release.UpdatedAt = now

	if err := release.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateRelease(ctx, release); err != nil {
		return fmt.Errorf("creating release: %w", err)
	}

	s.publishEvent(ctx, "release.created", release)
	return nil
}

// Get retrieves a release by its unique identifier.
func (s *releaseService) Get(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	release, err := s.repo.GetRelease(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting release: %w", err)
	}
	return release, nil
}

// List returns a paginated list of releases for a project.
func (s *releaseService) List(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	releases, err := s.repo.ListReleases(ctx, projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	return releases, nil
}

// Promote deploys a release to a specified environment. It checks promotion gates,
// transitions the release to active state if it is currently in draft, and creates a
// release-environment record.
func (s *releaseService) Promote(ctx context.Context, releaseID, environmentID, deployedBy uuid.UUID) error {
	release, err := s.repo.GetRelease(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("getting release for promotion: %w", err)
	}

	// Check promotion gates before allowing promotion.
	passed, err := s.CheckPromotionGates(ctx, releaseID, environmentID)
	if err != nil {
		return fmt.Errorf("checking promotion gates: %w", err)
	}
	if !passed {
		return fmt.Errorf("promotion gates not satisfied for release %s to environment %s", releaseID, environmentID)
	}

	// Transition to active if currently in draft.
	if release.Status == models.ReleaseStatusDraft {
		if err := release.ValidateTransition(models.ReleaseStatusActive); err != nil {
			return err
		}
		release.Status = models.ReleaseStatusActive
		release.LifecycleStatus = models.ReleaseLifecycleDeploying
		now := time.Now().UTC()
		release.ReleasedAt = &now
		release.UpdatedAt = now
		if err := s.repo.UpdateRelease(ctx, release); err != nil {
			return fmt.Errorf("activating release: %w", err)
		}
	}

	// Create the release-environment association.
	now := time.Now().UTC()
	re := &models.ReleaseEnvironment{
		ID:              uuid.New(),
		ReleaseID:       releaseID,
		EnvironmentID:   environmentID,
		Status:          models.ReleaseStatusActive,
		LifecycleStatus: models.ReleaseLifecycleDeploying,
		DeployedAt:      &now,
		DeployedBy:      &deployedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.CreateReleaseEnvironment(ctx, re); err != nil {
		return fmt.Errorf("creating release environment: %w", err)
	}

	s.publishEvent(ctx, "release.promoted", release)
	return nil
}

// UpdateStatus updates the lifecycle status of a release, validating the transition.
func (s *releaseService) UpdateStatus(ctx context.Context, releaseID uuid.UUID, status models.ReleaseLifecycleStatus) error {
	release, err := s.repo.GetRelease(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("getting release for status update: %w", err)
	}

	if err := models.ValidateLifecycleTransition(release.LifecycleStatus, status); err != nil {
		return err
	}

	release.LifecycleStatus = status
	release.UpdatedAt = time.Now().UTC()

	// Map lifecycle status to release status for high-level tracking.
	switch status {
	case models.ReleaseLifecycleDeployed, models.ReleaseLifecycleHealthy:
		if release.Status == models.ReleaseStatusDraft {
			release.Status = models.ReleaseStatusActive
		}
	case models.ReleaseLifecycleRolledBack:
		release.Status = models.ReleaseStatusFailed
	}

	if err := s.repo.UpdateRelease(ctx, release); err != nil {
		return fmt.Errorf("updating release status: %w", err)
	}

	s.publishEvent(ctx, "release.status_changed", release)
	return nil
}

// SetPromotionGate adds or updates a promotion gate for a release.
func (s *releaseService) SetPromotionGate(ctx context.Context, gate *PromotionGate) error {
	if gate.ID == uuid.Nil {
		gate.ID = uuid.New()
	}
	now := time.Now().UTC()
	gate.CreatedAt = now
	gate.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	gates := s.gates[gate.ReleaseID]

	// Update existing gate if it matches by name and environment.
	for i, existing := range gates {
		if existing.EnvironmentID == gate.EnvironmentID && existing.Name == gate.Name {
			gates[i] = gate
			s.gates[gate.ReleaseID] = gates
			return nil
		}
	}

	// Add new gate.
	s.gates[gate.ReleaseID] = append(gates, gate)
	return nil
}

// CheckPromotionGates verifies whether all promotion gates for a release
// and target environment have been satisfied.
func (s *releaseService) CheckPromotionGates(ctx context.Context, releaseID, environmentID uuid.UUID) (bool, error) {
	s.mu.RLock()
	gates := s.gates[releaseID]
	s.mu.RUnlock()

	for _, gate := range gates {
		if gate.EnvironmentID != environmentID {
			continue
		}
		if !gate.Passed {
			return false, nil
		}
	}
	return true, nil
}

// GetReleaseHealth returns aggregated health information for a release across
// all environments it has been deployed to.
func (s *releaseService) GetReleaseHealth(ctx context.Context, releaseID uuid.UUID) (*ReleaseHealthSummary, error) {
	envs, err := s.repo.ListReleaseEnvironments(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("listing release environments: %w", err)
	}

	summary := &ReleaseHealthSummary{
		ReleaseID:    releaseID,
		Environments: make([]*EnvironmentHealthSummary, 0, len(envs)),
		EvaluatedAt:  time.Now().UTC(),
	}

	if len(envs) == 0 {
		summary.OverallScore = 1.0
		summary.Healthy = true
		return summary, nil
	}

	var totalScore float64
	allHealthy := true

	for _, env := range envs {
		score := env.HealthScore

		// If a health provider is configured and the environment has a deployment,
		// fetch the live health score.
		if s.healthProvider != nil && env.DeploymentID != nil {
			liveScore, err := s.healthProvider.GetHealthScore(ctx, *env.DeploymentID)
			if err == nil {
				score = liveScore
			}
		}

		healthy := score >= s.healthThreshold
		if !healthy {
			allHealthy = false
		}

		summary.Environments = append(summary.Environments, &EnvironmentHealthSummary{
			EnvironmentID:   env.EnvironmentID,
			LifecycleStatus: env.LifecycleStatus,
			HealthScore:     score,
			Healthy:         healthy,
		})
		totalScore += score
	}

	summary.OverallScore = totalScore / float64(len(envs))
	summary.Healthy = allHealthy
	return summary, nil
}

// GetReleaseStatus returns the current status of a release including all
// environment deployments.
func (s *releaseService) GetReleaseStatus(ctx context.Context, releaseID uuid.UUID) (*ReleaseStatusResponse, error) {
	release, err := s.repo.GetRelease(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("getting release: %w", err)
	}

	envs, err := s.repo.ListReleaseEnvironments(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("listing release environments: %w", err)
	}

	return &ReleaseStatusResponse{
		ReleaseID:       release.ID,
		Version:         release.Version,
		Status:          release.Status,
		LifecycleStatus: release.LifecycleStatus,
		Environments:    envs,
	}, nil
}

// releaseEvent is the JSON payload emitted for release domain events.
type releaseEvent struct {
	ReleaseID       uuid.UUID                      `json:"release_id"`
	ProjectID       uuid.UUID                      `json:"project_id"`
	Version         string                         `json:"version"`
	Status          models.ReleaseStatus           `json:"status"`
	LifecycleStatus models.ReleaseLifecycleStatus  `json:"lifecycle_status"`
	Timestamp       time.Time                      `json:"timestamp"`
}

// publishEvent is a fire-and-forget helper that publishes a release domain event.
// Errors are non-fatal for the calling operation.
func (s *releaseService) publishEvent(ctx context.Context, subject string, release *models.Release) {
	if s.publisher == nil {
		return
	}

	evt := releaseEvent{
		ReleaseID:       release.ID,
		ProjectID:       release.ProjectID,
		Version:         release.Version,
		Status:          release.Status,
		LifecycleStatus: release.LifecycleStatus,
		Timestamp:       time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return
	}

	// Best-effort publish; errors are non-fatal for the calling operation.
	_ = s.publisher.Publish(ctx, subject, payload)
}
