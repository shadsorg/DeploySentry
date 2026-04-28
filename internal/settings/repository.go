package settings

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// SettingRepository defines persistence operations for hierarchical settings.
type SettingRepository interface {
	Create(ctx context.Context, setting *models.Setting) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Setting, error)
	ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error)
	Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error)
	Upsert(ctx context.Context, setting *models.Setting) error
	Delete(ctx context.Context, id uuid.UUID) error
}
