package rollout

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)

// StrategyRepository persists strategy templates.
type StrategyRepository interface {
	Create(ctx context.Context, s *models.Strategy) error
	// CreateTx persists a new strategy through an open transaction. Used by
	// the staging service so the create rides the same tx as the rest of the
	// deploy batch.
	CreateTx(ctx context.Context, tx pgx.Tx, s *models.Strategy) error
	Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error)
	GetByName(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, name string) (*models.Strategy, error)
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Strategy, error)
	ListByAnyScope(ctx context.Context, scopeIDs []ScopeRef) ([]*models.Strategy, error)
	Update(ctx context.Context, s *models.Strategy, expectedVersion int) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	IsReferenced(ctx context.Context, id uuid.UUID) (bool, error) // true if any strategy_defaults row references it
}

// StrategyDefaultRepository persists (scope, env, target_type) → strategy defaults.
type StrategyDefaultRepository interface {
	Upsert(ctx context.Context, d *models.StrategyDefault) error
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.StrategyDefault, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByKey(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, env *string, target *models.TargetType) error
}

// RolloutPolicyRepository persists onboarding + mandate policies per scope.
type RolloutPolicyRepository interface {
	Upsert(ctx context.Context, p *models.RolloutPolicy) error
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutPolicy, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ScopeRef is a scope-type + scope-id pair used in multi-scope lookups.
type ScopeRef struct {
	Type models.ScopeType
	ID   uuid.UUID
}
