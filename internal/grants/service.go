package grants

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrGrantNotFound = errors.New("grant not found")
	ErrDuplicate     = errors.New("grant already exists for this grantee on this resource")
)

// Service defines the business-logic layer for resource grants.
type Service interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error)
	ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error)
	Create(ctx context.Context, grant *models.ResourceGrant) (*models.ResourceGrant, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ResolveAccess(ctx context.Context, userID uuid.UUID, orgRole string, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
}

type grantService struct {
	repo Repository
}

// NewService creates a new grant Service.
func NewService(repo Repository) Service {
	return &grantService{repo: repo}
}

func (s *grantService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error) {
	rows, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing project grants: %w", err)
	}
	return rows, nil
}

func (s *grantService) ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error) {
	rows, err := s.repo.ListByApp(ctx, applicationID)
	if err != nil {
		return nil, fmt.Errorf("listing app grants: %w", err)
	}
	return rows, nil
}

func (s *grantService) Create(ctx context.Context, grant *models.ResourceGrant) (*models.ResourceGrant, error) {
	if err := grant.Validate(); err != nil {
		return nil, err
	}

	grant.ID = uuid.New()
	grant.CreatedAt = time.Now().UTC()

	if err := s.repo.Create(ctx, grant); err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("creating grant: %w", err)
	}
	return grant, nil
}

func (s *grantService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting grant: %w", err)
	}
	return nil
}

// ResolveAccess determines the effective permission a user has on a resource.
// It implements cascade logic: app grants take precedence when present,
// otherwise project grants are checked. Resources with no grants are open.
func (s *grantService) ResolveAccess(ctx context.Context, userID uuid.UUID, orgRole string, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	// Owners always have full access.
	if orgRole == "owner" {
		perm := models.PermissionWrite
		return &perm, nil
	}

	effectiveProjectID := projectID
	effectiveAppID := applicationID

	// If checking an app, determine whether it has its own grants.
	if applicationID != nil {
		hasGrants, err := s.repo.HasAnyGrants(ctx, nil, applicationID)
		if err != nil {
			return nil, fmt.Errorf("checking app grants: %w", err)
		}
		if hasGrants {
			// Use app-level grants.
			effectiveProjectID = nil
		} else {
			// Cascade to project.
			effectiveAppID = nil
		}
	}

	// Check if the effective resource has any grants at all.
	hasGrants, err := s.repo.HasAnyGrants(ctx, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, fmt.Errorf("checking resource grants: %w", err)
	}
	if !hasGrants {
		// Open resource — no grants means unrestricted.
		perm := models.PermissionWrite
		return &perm, nil
	}

	// Check direct user grant.
	perm, err := s.repo.GetUserPermission(ctx, userID, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, fmt.Errorf("checking user permission: %w", err)
	}
	if perm != nil {
		return perm, nil
	}

	// Check group-based grant.
	perm, err = s.repo.GetUserGroupPermission(ctx, userID, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, fmt.Errorf("checking group permission: %w", err)
	}
	if perm != nil {
		return perm, nil
	}

	// Denied.
	return nil, nil
}
