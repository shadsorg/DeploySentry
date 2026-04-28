package rollout

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutGroupRepository persists RolloutGroup rows.
type RolloutGroupRepository interface {
	Create(ctx context.Context, g *models.RolloutGroup) error
	Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error)
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutGroup, error)
	Update(ctx context.Context, g *models.RolloutGroup) error
	Delete(ctx context.Context, id uuid.UUID) error
}
