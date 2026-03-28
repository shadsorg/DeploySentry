package releases

import (
	"context"
	"encoding/json"
	"fmt"
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

// ReleaseService defines the interface for managing releases.
type ReleaseService interface {
	// Create creates a new release in draft state.
	Create(ctx context.Context, release *models.Release) error

	// GetByID retrieves a release by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*models.Release, error)

	// ListByApplication returns releases for an application.
	ListByApplication(ctx context.Context, appID uuid.UUID) ([]models.Release, error)

	// Start transitions a release from draft to rolling_out.
	Start(ctx context.Context, id uuid.UUID) error

	// Promote updates the traffic percentage for a rolling release.
	Promote(ctx context.Context, id uuid.UUID, trafficPercent int) error

	// Pause transitions a release to paused.
	Pause(ctx context.Context, id uuid.UUID) error

	// Rollback transitions a release to rolled_back.
	Rollback(ctx context.Context, id uuid.UUID) error

	// Complete transitions a release to completed.
	Complete(ctx context.Context, id uuid.UUID) error

	// Delete removes a draft release.
	Delete(ctx context.Context, id uuid.UUID) error

	// AddFlagChange adds a flag change to a release.
	AddFlagChange(ctx context.Context, fc *models.ReleaseFlagChange) error

	// ListFlagChanges returns the flag changes for a release.
	ListFlagChanges(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error)
}

// releaseService is the concrete implementation of ReleaseService.
type releaseService struct {
	repo      ReleaseRepository
	publisher EventPublisher
}

// NewReleaseService creates a new ReleaseService backed by the given repository.
func NewReleaseService(repo ReleaseRepository) ReleaseService {
	return &releaseService{repo: repo}
}

// NewReleaseServiceWithPublisher creates a new ReleaseService with an event publisher.
func NewReleaseServiceWithPublisher(repo ReleaseRepository, publisher EventPublisher) ReleaseService {
	return &releaseService{
		repo:      repo,
		publisher: publisher,
	}
}

// Create validates and persists a new release in draft state.
func (s *releaseService) Create(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	release.Status = models.ReleaseDraft
	now := time.Now().UTC()
	release.CreatedAt = now
	release.UpdatedAt = now

	if err := release.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.Create(ctx, release); err != nil {
		return fmt.Errorf("creating release: %w", err)
	}

	s.publishEvent(ctx, "release.created", release)
	return nil
}

// GetByID retrieves a release by its unique identifier.
func (s *releaseService) GetByID(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting release: %w", err)
	}
	return release, nil
}

// ListByApplication returns releases for an application.
func (s *releaseService) ListByApplication(ctx context.Context, appID uuid.UUID) ([]models.Release, error) {
	releases, err := s.repo.ListByApplication(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	return releases, nil
}

// Start transitions a release from draft to rolling_out.
func (s *releaseService) Start(ctx context.Context, id uuid.UUID) error {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if err := release.TransitionTo(models.ReleaseRollingOut); err != nil {
		return err
	}

	now := time.Now().UTC()
	release.StartedAt = &now
	release.UpdatedAt = now

	if err := s.repo.Update(ctx, release); err != nil {
		return fmt.Errorf("starting release: %w", err)
	}

	s.publishEvent(ctx, "release.started", release)
	return nil
}

// Promote updates the traffic percentage for a rolling release.
func (s *releaseService) Promote(ctx context.Context, id uuid.UUID, trafficPercent int) error {
	if trafficPercent < 0 || trafficPercent > 100 {
		return fmt.Errorf("traffic_percent must be between 0 and 100")
	}

	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if release.Status != models.ReleaseRollingOut && release.Status != models.ReleasePaused {
		return fmt.Errorf("can only promote releases in rolling_out or paused status, got %s", release.Status)
	}

	// If paused, transition back to rolling_out.
	if release.Status == models.ReleasePaused {
		if err := release.TransitionTo(models.ReleaseRollingOut); err != nil {
			return err
		}
	}

	release.TrafficPercent = trafficPercent
	release.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, release); err != nil {
		return fmt.Errorf("promoting release: %w", err)
	}

	s.publishEvent(ctx, "release.promoted", release)
	return nil
}

// Pause transitions a release to paused.
func (s *releaseService) Pause(ctx context.Context, id uuid.UUID) error {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if err := release.TransitionTo(models.ReleasePaused); err != nil {
		return err
	}

	release.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, release); err != nil {
		return fmt.Errorf("pausing release: %w", err)
	}

	s.publishEvent(ctx, "release.paused", release)
	return nil
}

// Rollback transitions a release to rolled_back.
func (s *releaseService) Rollback(ctx context.Context, id uuid.UUID) error {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if err := release.TransitionTo(models.ReleaseRolledBack); err != nil {
		return err
	}

	now := time.Now().UTC()
	release.CompletedAt = &now
	release.UpdatedAt = now

	if err := s.repo.Update(ctx, release); err != nil {
		return fmt.Errorf("rolling back release: %w", err)
	}

	s.publishEvent(ctx, "release.rolled_back", release)
	return nil
}

// Complete transitions a release to completed.
func (s *releaseService) Complete(ctx context.Context, id uuid.UUID) error {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if err := release.TransitionTo(models.ReleaseCompleted); err != nil {
		return err
	}

	now := time.Now().UTC()
	release.CompletedAt = &now
	release.UpdatedAt = now

	if err := s.repo.Update(ctx, release); err != nil {
		return fmt.Errorf("completing release: %w", err)
	}

	s.publishEvent(ctx, "release.completed", release)
	return nil
}

// Delete removes a release. Only draft releases can be deleted.
func (s *releaseService) Delete(ctx context.Context, id uuid.UUID) error {
	release, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if release.Status != models.ReleaseDraft {
		return fmt.Errorf("only draft releases can be deleted, current status: %s", release.Status)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting release: %w", err)
	}

	return nil
}

// AddFlagChange validates and persists a flag change for a release.
func (s *releaseService) AddFlagChange(ctx context.Context, fc *models.ReleaseFlagChange) error {
	if fc.ID == uuid.Nil {
		fc.ID = uuid.New()
	}
	fc.CreatedAt = time.Now().UTC()

	if err := fc.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.AddFlagChange(ctx, fc); err != nil {
		return fmt.Errorf("adding flag change: %w", err)
	}

	return nil
}

// ListFlagChanges returns the flag changes for a release.
func (s *releaseService) ListFlagChanges(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error) {
	changes, err := s.repo.ListFlagChanges(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("listing flag changes: %w", err)
	}
	return changes, nil
}

// releaseEvent is the JSON payload emitted for release domain events.
type releaseEvent struct {
	ReleaseID     uuid.UUID            `json:"release_id"`
	ApplicationID uuid.UUID            `json:"application_id"`
	Name          string               `json:"name"`
	Status        models.ReleaseStatus `json:"status"`
	Timestamp     time.Time            `json:"timestamp"`
}

// publishEvent is a fire-and-forget helper that publishes a release domain event.
func (s *releaseService) publishEvent(ctx context.Context, subject string, release *models.Release) {
	if s.publisher == nil {
		return
	}

	evt := releaseEvent{
		ReleaseID:     release.ID,
		ApplicationID: release.ApplicationID,
		Name:          release.Name,
		Status:        release.Status,
		Timestamp:     time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return
	}

	natsSubject := "releases." + subject
	_ = s.publisher.Publish(ctx, natsSubject, payload)
}
