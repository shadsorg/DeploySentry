// internal/members/service.go
package members

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrAlreadyMember = errors.New("user is already a member")
	ErrInvalidRole   = errors.New("invalid role")
	ErrLastOwner     = errors.New("cannot remove or demote the last owner")
	ErrOwnerAssign   = errors.New("cannot assign owner role via update; use ownership transfer")
	ErrNotFound      = errors.New("member not found")
)

// Service defines the business-logic layer for member management.
type Service interface {
	// Org members
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
	AddOrgMember(ctx context.Context, orgID uuid.UUID, email string, role models.OrgRole, addedBy uuid.UUID) (*OrgMemberRow, error)
	UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
	RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error
}

type memberService struct {
	repo Repository
}

// NewService creates a new member Service.
func NewService(repo Repository) Service {
	return &memberService{repo: repo}
}

// ---------------------------------------------------------------------------
// Org members
// ---------------------------------------------------------------------------

func (s *memberService) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error) {
	rows, err := s.repo.ListOrgMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing org members: %w", err)
	}
	return rows, nil
}

func (s *memberService) AddOrgMember(ctx context.Context, orgID uuid.UUID, email string, role models.OrgRole, addedBy uuid.UUID) (*OrgMemberRow, error) {
	if !models.ValidRole(role) {
		return nil, ErrInvalidRole
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	m := &models.OrgMember{
		OrgID:     orgID,
		UserID:    user.ID,
		Role:      role,
		InvitedBy: addedBy,
	}

	if err := s.repo.AddOrgMember(ctx, m); err != nil {
		if strings.Contains(err.Error(), "conflict") {
			return nil, ErrAlreadyMember
		}
		return nil, fmt.Errorf("adding org member: %w", err)
	}

	return &OrgMemberRow{
		OrgMember: *m,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}, nil
}

func (s *memberService) UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error {
	if !models.ValidRole(role) {
		return ErrInvalidRole
	}
	if role == models.OrgRoleOwner {
		return ErrOwnerAssign
	}

	// Guard: don't demote the last owner
	existing, err := s.repo.GetOrgMember(ctx, orgID, userID)
	if err != nil {
		return ErrNotFound
	}
	if existing.Role == models.OrgRoleOwner {
		count, err := s.repo.CountOrgOwners(ctx, orgID)
		if err != nil {
			return fmt.Errorf("counting owners: %w", err)
		}
		if count <= 1 {
			return ErrLastOwner
		}
	}

	if err := s.repo.UpdateOrgMemberRole(ctx, orgID, userID, role); err != nil {
		return fmt.Errorf("updating org member role: %w", err)
	}
	return nil
}

func (s *memberService) RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	existing, err := s.repo.GetOrgMember(ctx, orgID, userID)
	if err != nil {
		return ErrNotFound
	}
	if existing.Role == models.OrgRoleOwner {
		count, err := s.repo.CountOrgOwners(ctx, orgID)
		if err != nil {
			return fmt.Errorf("counting owners: %w", err)
		}
		if count <= 1 {
			return ErrLastOwner
		}
	}

	if err := s.repo.RemoveOrgMember(ctx, orgID, userID); err != nil {
		return fmt.Errorf("removing org member: %w", err)
	}
	return nil
}
