package grants

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// GrantRow extends ResourceGrant with the grantee name for display.
type GrantRow struct {
	models.ResourceGrant
	GranteeName string `json:"grantee_name"`
	GranteeType string `json:"grantee_type"` // "user" or "group"
}

// Repository defines persistence for resource grants.
type Repository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error)
	ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error)
	Create(ctx context.Context, g *models.ResourceGrant) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Access resolution queries
	HasAnyGrants(ctx context.Context, projectID *uuid.UUID, applicationID *uuid.UUID) (bool, error)
	GetUserPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
	GetUserGroupPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
}
