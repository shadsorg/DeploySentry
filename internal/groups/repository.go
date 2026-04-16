package groups

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// GroupMemberRow extends GroupMember with user details from a JOIN.
type GroupMemberRow struct {
	models.GroupMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// GroupRow extends Group with a member count.
type GroupRow struct {
	models.Group
	MemberCount int `json:"member_count"`
}

// Repository defines persistence for groups and group members.
type Repository interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error)
	Create(ctx context.Context, g *models.Group) error
	Update(ctx context.Context, g *models.Group) error
	Delete(ctx context.Context, id uuid.UUID) error

	ListMembers(ctx context.Context, groupID uuid.UUID) ([]GroupMemberRow, error)
	AddMember(ctx context.Context, groupID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
}
