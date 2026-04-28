// Package currentstate assembles the per-environment "current state"
// response consumed by the dashboard and external integrations.
package currentstate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/deploy"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ErrEnvNotFound is returned when the referenced environment does not exist.
var ErrEnvNotFound = errors.New("environment not found")

// FreshnessWindow is the cliff below which a health sample counts as "fresh".
const FreshnessWindow = 60 * time.Second

// StalenessWindow is the cliff below which a sample counts as "stale"
// (but above FreshnessWindow). Above this cliff the sample is "missing".
const StalenessWindow = 5 * time.Minute

// DeployLookup exposes the subset of deploy.DeployService used by the assembler.
type DeployLookup interface {
	GetActiveDeployments(ctx context.Context, applicationID uuid.UUID) ([]*models.Deployment, error)
	ListDeployments(ctx context.Context, applicationID uuid.UUID, opts deploy.ListOptions) ([]*models.Deployment, error)
}

// StatusLookup exposes the subset of appstatus.Repository used by the assembler.
type StatusLookup interface {
	GetStatus(ctx context.Context, appID, envID uuid.UUID) (*models.AppStatus, error)
}

// EnvironmentLookup exposes the env-repo method needed to decorate the response.
type EnvironmentLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.OrgEnvironment, error)
}

// Service assembles the response.
type Service struct {
	deploys      DeployLookup
	statuses     StatusLookup
	environments EnvironmentLookup
	now          func() time.Time
}

// NewService constructs the assembler.
func NewService(d DeployLookup, s StatusLookup, e EnvironmentLookup) *Service {
	return &Service{deploys: d, statuses: s, environments: e, now: time.Now}
}

// Resolve returns the single-call response for (appID, envID).
func (s *Service) Resolve(ctx context.Context, appID, envID uuid.UUID, limit int) (*models.CurrentStateResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	env, err := s.environments.GetByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("lookup environment: %w", err)
	}
	if env == nil {
		return nil, ErrEnvNotFound
	}

	resp := &models.CurrentStateResponse{
		Environment: models.EnvironmentSummary{
			ID:   env.ID,
			Slug: env.Slug,
			Name: env.Name,
		},
	}

	// Current deployment: pick the newest active for (app, env); fall back
	// to the newest completed.
	active, err := s.deploys.GetActiveDeployments(ctx, appID)
	if err == nil {
		for _, d := range active {
			if d.EnvironmentID == envID {
				resp.CurrentDeployment = toCurrentDeployment(d)
				break
			}
		}
	}

	recent, err := s.deploys.ListDeployments(ctx, appID, deploy.ListOptions{
		EnvironmentID: &envID,
		Limit:         limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	resp.RecentDeployments = make([]models.RecentDeployment, 0, len(recent))
	for _, d := range recent {
		resp.RecentDeployments = append(resp.RecentDeployments, toRecent(d))
	}

	// If no active-but-non-terminal deployment existed, use the first
	// (most recent) entry from the history as "current".
	if resp.CurrentDeployment == nil && len(recent) > 0 {
		resp.CurrentDeployment = toCurrentDeployment(recent[0])
	}

	// Health block from the latest app_status sample.
	status, err := s.statuses.GetStatus(ctx, appID, envID)
	if err != nil || status == nil {
		resp.Health = models.HealthBlock{
			State:     models.HealthStateUnknown,
			Source:    "unknown",
			Staleness: models.HealthStalenessMissing,
		}
	} else {
		reported := status.ReportedAt
		resp.Health = models.HealthBlock{
			State:          status.HealthState,
			Score:          status.HealthScore,
			Reason:         status.HealthReason,
			Source:         status.Source,
			LastReportedAt: &reported,
			Staleness:      classifyStaleness(s.now(), reported),
		}
	}

	return resp, nil
}

func classifyStaleness(now, reported time.Time) models.HealthStaleness {
	age := now.Sub(reported)
	switch {
	case age < FreshnessWindow:
		return models.HealthStalenessFresh
	case age < StalenessWindow:
		return models.HealthStalenessStale
	default:
		return models.HealthStalenessMissing
	}
}

func toCurrentDeployment(d *models.Deployment) *models.CurrentDeployment {
	return &models.CurrentDeployment{
		ID:             d.ID,
		Version:        d.Version,
		CommitSHA:     d.CommitSHA,
		Status:         d.Status,
		Mode:           d.Mode,
		Source:         d.Source,
		TrafficPercent: d.TrafficPercent,
		StartedAt:      d.StartedAt,
		CompletedAt:    d.CompletedAt,
	}
}

func toRecent(d *models.Deployment) models.RecentDeployment {
	return models.RecentDeployment{
		ID:          d.ID,
		Version:     d.Version,
		Status:      d.Status,
		Mode:        d.Mode,
		Source:      d.Source,
		CompletedAt: d.CompletedAt,
		CreatedAt:   d.CreatedAt,
	}
}
