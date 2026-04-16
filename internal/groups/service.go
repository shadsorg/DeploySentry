// internal/groups/service.go
package groups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrGroupNotFound = errors.New("group not found")
	ErrSlugTaken     = errors.New("group slug already exists in this organization")
)

// Service defines the business-logic layer for group management.
type Service interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error)
	Create(ctx context.Context, orgID uuid.UUID, name, description string, createdBy uuid.UUID) (*models.Group, error)
	Update(ctx context.Context, orgID uuid.UUID, slug string, name, description string) (*models.Group, error)
	Delete(ctx context.Context, orgID uuid.UUID, slug string) error
	ListMembers(ctx context.Context, orgID uuid.UUID, groupSlug string) ([]GroupMemberRow, error)
	AddMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error
	RemoveMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error
}

type groupService struct {
	repo Repository
}

// NewService creates a new group Service.
func NewService(repo Repository) Service {
	return &groupService{repo: repo}
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// generateSlug converts a name into a URL-friendly slug.
func generateSlug(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func (s *groupService) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error) {
	rows, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	return rows, nil
}

func (s *groupService) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

func (s *groupService) Create(ctx context.Context, orgID uuid.UUID, name, description string, createdBy uuid.UUID) (*models.Group, error) {
	slug := generateSlug(name)

	// Check for existing slug in this org.
	if existing, _ := s.repo.GetBySlug(ctx, orgID, slug); existing != nil {
		return nil, ErrSlugTaken
	}

	now := time.Now().UTC()
	g := &models.Group{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        name,
		Slug:        slug,
		Description: description,
		CreatedBy:   &createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	if err := s.repo.Create(ctx, g); err != nil {
		return nil, fmt.Errorf("creating group: %w", err)
	}
	return g, nil
}

func (s *groupService) Update(ctx context.Context, orgID uuid.UUID, slug string, name, description string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, ErrGroupNotFound
	}

	newSlug := generateSlug(name)

	// If the slug changed, check uniqueness.
	if newSlug != g.Slug {
		if existing, _ := s.repo.GetBySlug(ctx, orgID, newSlug); existing != nil {
			return nil, ErrSlugTaken
		}
	}

	g.Name = name
	g.Slug = newSlug
	g.Description = description
	g.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, g); err != nil {
		return nil, fmt.Errorf("updating group: %w", err)
	}
	return g, nil
}

func (s *groupService) Delete(ctx context.Context, orgID uuid.UUID, slug string) error {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return ErrGroupNotFound
	}
	if err := s.repo.Delete(ctx, g.ID); err != nil {
		return fmt.Errorf("deleting group: %w", err)
	}
	return nil
}

// resolveGroup is a helper that looks up a group by org + slug.
func (s *groupService) resolveGroup(ctx context.Context, orgID uuid.UUID, groupSlug string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, groupSlug)
	if err != nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

func (s *groupService) ListMembers(ctx context.Context, orgID uuid.UUID, groupSlug string) ([]GroupMemberRow, error) {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListMembers(ctx, g.ID)
	if err != nil {
		return nil, fmt.Errorf("listing group members: %w", err)
	}
	return rows, nil
}

func (s *groupService) AddMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return err
	}
	if err := s.repo.AddMember(ctx, g.ID, userID); err != nil {
		return fmt.Errorf("adding group member: %w", err)
	}
	return nil
}

func (s *groupService) RemoveMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return err
	}
	if err := s.repo.RemoveMember(ctx, g.ID, userID); err != nil {
		return fmt.Errorf("removing group member: %w", err)
	}
	return nil
}
