package rollout

import (
	"context"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutRepository persists Rollout rows.
type RolloutRepository interface {
	Create(ctx context.Context, r *models.Rollout) error
	Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error)
	GetActiveByDeployment(ctx context.Context, deploymentID uuid.UUID) (*models.Rollout, error)
	GetActiveByConfig(ctx context.Context, flagKey, env string) (*models.Rollout, error)
	GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error)
	List(ctx context.Context, opts RolloutListOptions) ([]*models.Rollout, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, rollbackReason *string) error
	UpdatePhasePointer(ctx context.Context, id uuid.UUID, phaseIndex int, startedAt *time.Time, lastHealthy *time.Time) error
	MarkCompleted(ctx context.Context, id uuid.UUID) error
}

// RolloutListOptions filters list queries.
type RolloutListOptions struct {
	TargetType *models.TargetType
	Status     *models.RolloutStatus
	ReleaseID  *uuid.UUID
	Limit      int
}

// RolloutPhaseRepository persists per-phase state.
type RolloutPhaseRepository interface {
	BulkInsert(ctx context.Context, phases []*models.RolloutPhase) error
	ListByRollout(ctx context.Context, rolloutID uuid.UUID) ([]*models.RolloutPhase, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error
}

// RolloutEventRepository persists audit events.
type RolloutEventRepository interface {
	Insert(ctx context.Context, e *models.RolloutEvent) error
	ListByRollout(ctx context.Context, rolloutID uuid.UUID, limit int) ([]*models.RolloutEvent, error)
}
