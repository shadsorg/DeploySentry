// internal/members/repository.go
package members

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// OrgMemberRow extends OrgMember with user profile fields from a JOIN.
type OrgMemberRow struct {
	models.OrgMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Repository defines persistence operations for org members.
type Repository interface {
	// Org members
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
	GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error)
	AddOrgMember(ctx context.Context, m *models.OrgMember) error
	UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
	RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error
	CountOrgOwners(ctx context.Context, orgID uuid.UUID) (int, error)

	// User lookup
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}
