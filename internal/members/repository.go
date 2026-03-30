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

// ProjectMemberRow extends ProjectMember with user profile fields from a JOIN.
type ProjectMemberRow struct {
	models.ProjectMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Repository defines persistence operations for org and project members.
type Repository interface {
	// Org members
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
	GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error)
	AddOrgMember(ctx context.Context, m *models.OrgMember) error
	UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
	RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error
	CountOrgOwners(ctx context.Context, orgID uuid.UUID) (int, error)

	// Project members
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error)
	GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error)
	AddProjectMember(ctx context.Context, m *models.ProjectMember) error
	UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error

	// User lookup
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}
