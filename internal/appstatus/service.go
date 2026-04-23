package appstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

const defaultSource = "app-push"

// DeployCreator is the narrow dependency the service needs from the deploy
// package to auto-create mode=record rows on new versions.
type DeployCreator interface {
	CreateDeployment(ctx context.Context, d *models.Deployment) error
}

// Service processes inbound status reports.
type Service struct {
	repo   Repository
	deploy DeployCreator
}

// NewService constructs the Service.
func NewService(repo Repository, deploy DeployCreator) *Service {
	return &Service{repo: repo, deploy: deploy}
}

// ReportInput carries the values needed for a single report. Callers
// (handlers, SDK adapters) validate the payload before delegating here.
type ReportInput struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	Payload       models.ReportStatusPayload
	Source        string
	CreatedBy     uuid.UUID
}

// Report persists a status sample, optionally auto-creates a mode=record
// deployment when the reported version is new to this (app, env), and
// returns the latest status row.
func (s *Service) Report(ctx context.Context, in ReportInput) (*models.AppStatus, error) {
	if in.ApplicationID == uuid.Nil {
		return nil, errors.New("application_id is required")
	}
	if in.EnvironmentID == uuid.Nil {
		return nil, errors.New("environment_id is required")
	}
	if err := in.Payload.Validate(); err != nil {
		return nil, err
	}

	source := in.Source
	if source == "" {
		source = defaultSource
	}

	now := time.Now().UTC()
	tags := in.Payload.Tags
	if tags == nil {
		tags = map[string]string{}
	}
	status := &models.AppStatus{
		ApplicationID: in.ApplicationID,
		EnvironmentID: in.EnvironmentID,
		Version:       in.Payload.Version,
		CommitSHA:     in.Payload.CommitSHA,
		HealthState:   in.Payload.Health,
		HealthScore:   in.Payload.HealthScore,
		HealthReason:  in.Payload.HealthReason,
		DeploySlot:    in.Payload.DeploySlot,
		Tags:          tags,
		Source:        source,
		ReportedAt:    now,
	}

	if err := s.repo.UpsertStatus(ctx, status); err != nil {
		return nil, fmt.Errorf("upsert app_status: %w", err)
	}

	if err := s.repo.AppendHistory(ctx, &models.AppStatusSample{
		ApplicationID: in.ApplicationID,
		EnvironmentID: in.EnvironmentID,
		Version:       in.Payload.Version,
		HealthState:   in.Payload.Health,
		HealthScore:   in.Payload.HealthScore,
		ReportedAt:    now,
	}); err != nil {
		// History is best-effort; log-and-continue is acceptable, but
		// returning the error here keeps behavior simple and testable.
		return nil, fmt.Errorf("append app_status_history: %w", err)
	}

	// Auto-create a recorded deployment the first time we see this version.
	exists, err := s.repo.HasDeploymentForVersion(ctx, in.ApplicationID, in.EnvironmentID, in.Payload.Version)
	if err != nil {
		return nil, fmt.Errorf("check deployment existence: %w", err)
	}
	if !exists && s.deploy != nil {
		artifact := in.Payload.Version
		d := &models.Deployment{
			ApplicationID: in.ApplicationID,
			EnvironmentID: in.EnvironmentID,
			Strategy:      models.DeployStrategyRolling, // informational only in record mode
			Artifact:      artifact,
			Version:       in.Payload.Version,
			CommitSHA:     in.Payload.CommitSHA,
			CreatedBy:     in.CreatedBy,
			Mode:          models.DeployModeRecord,
			Source:        &source,
		}
		if err := s.deploy.CreateDeployment(ctx, d); err != nil {
			return nil, fmt.Errorf("auto-create deployment: %w", err)
		}
	}

	return status, nil
}
