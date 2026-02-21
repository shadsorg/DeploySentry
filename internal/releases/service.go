package releases

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

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
}

// releaseService is the concrete implementation of ReleaseService.
type releaseService struct {
	repo ReleaseRepository
}

// NewReleaseService creates a new ReleaseService backed by the given repository.
func NewReleaseService(repo ReleaseRepository) ReleaseService {
	return &releaseService{repo: repo}
}

// Create validates and persists a new release in draft state.
func (s *releaseService) Create(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	release.Status = models.ReleaseStatusDraft
	now := time.Now().UTC()
	release.CreatedAt = now
	release.UpdatedAt = now

	if err := release.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateRelease(ctx, release); err != nil {
		return fmt.Errorf("creating release: %w", err)
	}
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

// Promote deploys a release to a specified environment. It transitions the
// release to active state if it is currently in draft, and creates a
// release-environment record.
func (s *releaseService) Promote(ctx context.Context, releaseID, environmentID, deployedBy uuid.UUID) error {
	release, err := s.repo.GetRelease(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("getting release for promotion: %w", err)
	}

	// Transition to active if currently in draft.
	if release.Status == models.ReleaseStatusDraft {
		if err := release.ValidateTransition(models.ReleaseStatusActive); err != nil {
			return err
		}
		release.Status = models.ReleaseStatusActive
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
		ID:            uuid.New(),
		ReleaseID:     releaseID,
		EnvironmentID: environmentID,
		Status:        models.ReleaseStatusActive,
		DeployedAt:    &now,
		DeployedBy:    &deployedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.CreateReleaseEnvironment(ctx, re); err != nil {
		return fmt.Errorf("creating release environment: %w", err)
	}

	return nil
}
